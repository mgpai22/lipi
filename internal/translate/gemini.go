package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"sync"

	"google.golang.org/genai"
)

// implements Translator using Google Gemini
type GeminiTranslator struct {
	client  *genai.Client
	model   string
	options Options
}

func NewGeminiTranslator(
	ctx context.Context,
	apiKey string,
	opts Options,
) (*GeminiTranslator, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	model := opts.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}

	return &GeminiTranslator{
		client:  client,
		model:   model,
		options: opts,
	}, nil
}

const DefaultBatchSize = 50

func (t *GeminiTranslator) batchSize() int {
	if t.options.BatchSize > 0 {
		return t.options.BatchSize
	}
	return DefaultBatchSize
}

func (t *GeminiTranslator) Translate(
	ctx context.Context,
	items []TranslationItem,
) ([]TranslationResult, error) {
	if len(items) == 0 {
		return []TranslationResult{}, nil
	}

	batchSize := t.batchSize()
	if len(items) <= batchSize {
		return t.translateBatch(ctx, items)
	}

	var allResults []TranslationResult
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}

		batch := items[i:end]
		results, err := t.translateBatch(ctx, batch)
		if err != nil {
			return nil, fmt.Errorf("batch %d failed: %w", i/batchSize, err)
		}
		allResults = append(allResults, results...)
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Index < allResults[j].Index
	})

	return allResults, nil
}

// Items are split into batches of BatchSize (default 50). Each batch becomes
// one API request. Workers (up to concurrency) pull batches from a shared queue.
func (t *GeminiTranslator) TranslateWithConcurrency(
	ctx context.Context,
	items []TranslationItem,
	concurrency int,
) ([]TranslationResult, error) {
	if len(items) == 0 {
		return []TranslationResult{}, nil
	}

	if concurrency <= 0 {
		concurrency = 3
	}

	batchSize := t.batchSize()
	var batches [][]TranslationItem
	for i := 0; i < len(items); i += batchSize {
		end := i + batchSize
		if end > len(items) {
			end = len(items)
		}
		batches = append(batches, items[i:end])
	}

	if len(batches) == 1 {
		return t.translateBatch(ctx, batches[0])
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	type batchResult struct {
		Index   int
		Results []TranslationResult
		Error   error
	}

	workChan := make(chan int)
	resultChan := make(chan batchResult, len(batches))

	var wg sync.WaitGroup
	for i := 0; i < concurrency && i < len(batches); i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case batchIdx, ok := <-workChan:
					if !ok {
						return
					}
					if ctx.Err() != nil {
						return
					}

					results, err := t.translateBatch(ctx, batches[batchIdx])
					if err != nil {
						cancel()
					}
					resultChan <- batchResult{
						Index:   batchIdx,
						Results: results,
						Error:   err,
					}
				}
			}
		}()
	}

	go func() {
		defer close(workChan)
		for i := range batches {
			select {
			case <-ctx.Done():
				return
			case workChan <- i:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	results := make([]batchResult, 0, len(batches))
	var firstErr error
	for result := range resultChan {
		if result.Error != nil && firstErr == nil {
			firstErr = fmt.Errorf(
				"batch %d failed: %w",
				result.Index,
				result.Error,
			)
			cancel()
		}
		if result.Error == nil {
			results = append(results, result)
		}
	}

	if firstErr != nil {
		return nil, firstErr
	}

	sort.Slice(results, func(i, j int) bool {
		return results[i].Index < results[j].Index
	})

	var allResults []TranslationResult
	for _, r := range results {
		allResults = append(allResults, r.Results...)
	}

	sort.Slice(allResults, func(i, j int) bool {
		return allResults[i].Index < allResults[j].Index
	})

	return allResults, nil
}

func (t *GeminiTranslator) translateBatch(
	ctx context.Context,
	items []TranslationItem,
) ([]TranslationResult, error) {
	prompt := BuildPrompt(t.options, items)

	parts := []*genai.Part{
		genai.NewPartFromText(prompt),
	}
	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	result, err := t.client.Models.GenerateContent(ctx, t.model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("translation failed: %w", err)
	}

	return t.parseResponse(result, len(items))
}

func (t *GeminiTranslator) parseResponse(
	result *genai.GenerateContentResponse,
	expectedCount int,
) ([]TranslationResult, error) {
	if result == nil || len(result.Candidates) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	var responseText string
	for _, candidate := range result.Candidates {
		if candidate.Content == nil {
			continue
		}
		for _, part := range candidate.Content.Parts {
			if part.Text != "" {
				responseText += part.Text
			}
		}
		if responseText != "" {
			break
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("no text in Gemini response")
	}

	responseText = cleanJSONResponse(responseText)

	results, err := extractTranslationResults(responseText)
	if err != nil {
		return nil, fmt.Errorf(
			"failed to parse JSON response: %w (response: %s)",
			err,
			truncateString(responseText, 200),
		)
	}

	if len(results) != expectedCount {
		return nil, fmt.Errorf(
			"expected %d results, got %d",
			expectedCount,
			len(results),
		)
	}

	return results, nil
}

func cleanJSONResponse(s string) string {
	s = strings.TrimSpace(s)

	jsonBlockRegex := regexp.MustCompile("```(?:json)?\\s*")
	s = jsonBlockRegex.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "```", "")

	s = strings.TrimSpace(s)

	return s
}

// fixes invalid JSON escape sequences like \N (SRT newline).
// It replaces \N with \\N so JSON can parse it, preserving the literal \N in the output.
func fixInvalidEscapes(s string) string {
	var result strings.Builder
	result.Grow(len(s))

	i := 0
	for i < len(s) {
		if i < len(s)-1 && s[i] == '\\' {
			next := s[i+1]
			// Valid JSON escape sequences: ", \, /, b, f, n, r, t, u
			switch next {
			case '"', '\\', '/', 'b', 'f', 'n', 'r', 't', 'u':
				// Valid escape, keep as-is
				result.WriteByte(s[i])
				result.WriteByte(s[i+1])
				i += 2
			default:
				// Invalid escape like \N - escape the backslash
				result.WriteString("\\\\")
				result.WriteByte(next)
				i += 2
			}
		} else {
			result.WriteByte(s[i])
			i++
		}
	}

	return result.String()
}

func extractTranslationResults(text string) ([]TranslationResult, error) {
	text = fixInvalidEscapes(text)

	for i := 0; i < len(text); i++ {
		if text[i] != '[' && text[i] != '{' {
			continue
		}
		decoder := json.NewDecoder(strings.NewReader(text[i:]))
		var raw json.RawMessage
		if err := decoder.Decode(&raw); err != nil {
			continue
		}
		if results, ok := tryExtractResults(raw); ok && len(results) > 0 {
			return results, nil
		}
	}
	return nil, fmt.Errorf("no valid translation JSON found in response")
}

func tryExtractResults(raw json.RawMessage) ([]TranslationResult, bool) {
	var results []TranslationResult
	if err := json.Unmarshal(
		raw,
		&results,
	); err == nil &&
		validateResults(results) {
		return results, true
	}

	wrapperKeys := []string{"results", "translations", "data", "items"}
	var wrapper map[string]json.RawMessage
	if err := json.Unmarshal(raw, &wrapper); err != nil {
		return nil, false
	}

	for _, key := range wrapperKeys {
		if fieldRaw, exists := wrapper[key]; exists {
			var fieldResults []TranslationResult
			if err := json.Unmarshal(
				fieldRaw,
				&fieldResults,
			); err == nil && validateResults(fieldResults) {
				return fieldResults, true
			}
		}
	}

	for _, fieldRaw := range wrapper {
		var fieldResults []TranslationResult
		if err := json.Unmarshal(
			fieldRaw,
			&fieldResults,
		); err == nil && validateResults(fieldResults) {
			return fieldResults, true
		}
	}

	return nil, false
}

func validateResults(results []TranslationResult) bool {
	for _, r := range results {
		if r.Text != "" {
			return true
		}
	}
	return false
}

func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

func (t *GeminiTranslator) Close() error {
	return nil
}
