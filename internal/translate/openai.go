package translate

import (
	"context"
	"fmt"
	"sort"
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
	prompt := BuildPrompt(t.options, items)

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
