package translate

import (
	"context"
	"fmt"
	"sort"
	"sync"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
)

// implements Translator using Anthropic Claude
type AnthropicTranslator struct {
	client  anthropic.Client
	model   anthropic.Model
	options Options
}

func NewAnthropicTranslator(
	ctx context.Context,
	apiKey string,
	opts Options,
) (*AnthropicTranslator, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	client := anthropic.NewClient(option.WithAPIKey(apiKey))

	model := anthropic.Model(opts.Model)
	if opts.Model == "" {
		model = anthropic.ModelClaudeHaiku4_5
	}

	return &AnthropicTranslator{
		client:  client,
		model:   model,
		options: opts,
	}, nil
}

func (t *AnthropicTranslator) batchSize() int {
	if t.options.BatchSize > 0 {
		return t.options.BatchSize
	}
	return DefaultBatchSize
}

func (t *AnthropicTranslator) Translate(
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
func (t *AnthropicTranslator) TranslateWithConcurrency(
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

func (t *AnthropicTranslator) translateBatch(
	ctx context.Context,
	items []TranslationItem,
) ([]TranslationResult, error) {
	prompt := BuildPrompt(t.options, items)

	message, err := t.client.Messages.New(
		ctx,
		anthropic.MessageNewParams{
			Model:     t.model,
			MaxTokens: 4096,
			Messages: []anthropic.MessageParam{
				anthropic.NewUserMessage(
					anthropic.NewTextBlock(prompt),
				),
			},
		},
	)
	if err != nil {
		return nil, fmt.Errorf("translation failed: %w", err)
	}

	return t.parseResponse(message, len(items))
}

func (t *AnthropicTranslator) parseResponse(
	message *anthropic.Message,
	expectedCount int,
) ([]TranslationResult, error) {
	if message == nil || len(message.Content) == 0 {
		return nil, fmt.Errorf("empty response from Anthropic")
	}

	var responseText string
	for _, block := range message.Content {
		if block.Type == "text" {
			responseText += block.Text
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("no text in Anthropic response")
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

func (t *AnthropicTranslator) Close() error {
	return nil
}
