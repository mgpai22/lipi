package translate

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

// single text item to translate
type TranslationItem struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

// translated text item
type TranslationResult struct {
	Index int    `json:"index"`
	Text  string `json:"text"`
}

// interface for text translation
type Translator interface {
	Translate(
		ctx context.Context,
		items []TranslationItem,
	) ([]TranslationResult, error)
}

// optional interface for translators that support concurrent batch processing
type ConcurrentTranslator interface {
	Translator
	TranslateWithConcurrency(
		ctx context.Context,
		items []TranslationItem,
		concurrency int,
	) ([]TranslationResult, error)
}

// translation service provider
type Provider string

const (
	ProviderGemini    Provider = "gemini"
	ProviderOpenAI    Provider = "openai"
	ProviderAnthropic Provider = "anthropic"
)

type Options struct {
	InputLanguage  string
	TargetLanguage string
	Model          string
	Prompt         string
	BatchSize      int // items per API request (default 50)
}

// creates Translator based on provider
func Factory(
	ctx context.Context,
	provider Provider,
	apiKey string,
	opts Options,
) (Translator, error) {
	if opts.TargetLanguage == "" {
		return nil, fmt.Errorf("target language is required")
	}

	switch provider {
	case ProviderGemini:
		return NewGeminiTranslator(ctx, apiKey, opts)
	case ProviderOpenAI:
		return NewOpenAITranslator(ctx, apiKey, opts)
	case ProviderAnthropic:
		return NewAnthropicTranslator(ctx, apiKey, opts)
	default:
		return nil, fmt.Errorf("unsupported translation provider: %s", provider)
	}
}

// BuildPrompt creates the translation prompt for LLM providers
func BuildPrompt(opts Options, items []TranslationItem) string {
	var sb strings.Builder

	if opts.InputLanguage != "" {
		sb.WriteString(fmt.Sprintf(
			"Translate the following %s subtitle texts to %s.\n\n",
			opts.InputLanguage,
			opts.TargetLanguage,
		))
	} else {
		sb.WriteString(fmt.Sprintf(
			"Translate the following subtitle texts to %s.\n\n",
			opts.TargetLanguage,
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

	if opts.Prompt != "" {
		sb.WriteString(
			fmt.Sprintf("Additional instructions: %s\n\n", opts.Prompt),
		)
	}

	sb.WriteString("Input JSON:\n")

	inputJSON, _ := json.MarshalIndent(items, "", "  ")
	sb.Write(inputJSON)

	sb.WriteString("\n\nOutput the translated JSON array only:")

	return sb.String()
}
