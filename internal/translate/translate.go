package translate

import (
	"context"
	"fmt"
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

// translation service provider
type Provider string

const (
	ProviderGemini Provider = "gemini"
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
	default:
		return nil, fmt.Errorf("unsupported translation provider: %s", provider)
	}
}
