package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"sort"
	"strings"
	"sync"

	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// implements Translator using OpenAI Chat Completions
type OpenAITranslator struct {
	client  openai.Client
	model   string
	options Options
}

func NewOpenAITranslator(
	ctx context.Context,
	apiKey string,
	opts Options,
) (*OpenAITranslator, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	model := opts.Model
	if model == "" {
		model = "gpt-5-mini"
	}

	return &OpenAITranslator{
		client:  client,
		model:   model,
		options: opts,
	}, nil
}

func (t *OpenAITranslator) batchSize() int {
	if t.options.BatchSize > 0 {
		return t.options.BatchSize
	}
	return DefaultBatchSize
}

func (t *OpenAITranslator) Translate(
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
func (t *OpenAITranslator) TranslateWithConcurrency(
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

func (t *OpenAITranslator) translateBatch(
	ctx context.Context,
	items []TranslationItem,
) ([]TranslationResult, error) {
	prompt := t.buildPrompt(items)

	completion, err := t.client.Chat.Completions.New(
		ctx,
		openai.ChatCompletionNewParams{
			Messages: []openai.ChatCompletionMessageParamUnion{
				openai.UserMessage(prompt),
			},
			Model: t.model,
		},
	)
	if err != nil {
		return nil, fmt.Errorf("translation failed: %w", err)
	}

	return t.parseResponse(completion, len(items))
}

func (t *OpenAITranslator) buildPrompt(items []TranslationItem) string {
	var sb strings.Builder

	if t.options.InputLanguage != "" {
		sb.WriteString(fmt.Sprintf(
			"Translate the following %s subtitle texts to %s.\n\n",
			t.options.InputLanguage,
			t.options.TargetLanguage,
		))
	} else {
		sb.WriteString(fmt.Sprintf(
			"Translate the following subtitle texts to %s.\n\n",
			t.options.TargetLanguage,
		))
	}

	sb.WriteString("IMPORTANT INSTRUCTIONS:\n")
	sb.WriteString(
		"1. Translate ONLY the text content, preserving the meaning.\n",
	)
	sb.WriteString(
		"2. Keep any formatting tags (like {\\pos}, {\\an}, etc.) unchanged.\n",
	)
	sb.WriteString("3. Preserve line breaks (\\N) in the same positions.\n")
	sb.WriteString("4. Return ONLY a JSON array with the same structure.\n")
	sb.WriteString("5. Each object must have 'index' and 'text' fields.\n")
	sb.WriteString(
		"6. The 'index' values must match the input indices exactly.\n",
	)
	sb.WriteString("7. Do not add any explanation or markdown formatting.\n\n")

	if t.options.Prompt != "" {
		sb.WriteString(
			fmt.Sprintf("Additional instructions: %s\n\n", t.options.Prompt),
		)
	}

	sb.WriteString("Input JSON:\n")

	inputJSON, _ := json.MarshalIndent(items, "", "  ")
	sb.Write(inputJSON)

	sb.WriteString("\n\nOutput the translated JSON array only:")

	return sb.String()
}

func (t *OpenAITranslator) parseResponse(
	completion *openai.ChatCompletion,
	expectedCount int,
) ([]TranslationResult, error) {
	if completion == nil || len(completion.Choices) == 0 {
		return nil, fmt.Errorf("empty response from OpenAI")
	}

	responseText := completion.Choices[0].Message.Content

	if responseText == "" {
		return nil, fmt.Errorf("no text in OpenAI response")
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

func (t *OpenAITranslator) Close() error {
	return nil
}
