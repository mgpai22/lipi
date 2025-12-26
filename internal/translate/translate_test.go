package translate

import (
	"context"
	"os"
	"testing"
)

func TestFactoryReturnsGeminiTranslator(t *testing.T) {
	ctx := context.Background()
	opts := Options{TargetLanguage: "Japanese"}
	translator, err := Factory(ctx, ProviderGemini, "fake-key", opts)
	if err != nil {
		t.Fatalf("Factory(ProviderGemini) returned error: %v", err)
	}
	if _, ok := translator.(*GeminiTranslator); !ok {
		t.Errorf("expected *GeminiTranslator, got %T", translator)
	}
}

func TestFactoryReturnsOpenAITranslator(t *testing.T) {
	ctx := context.Background()
	opts := Options{TargetLanguage: "Spanish"}
	translator, err := Factory(ctx, ProviderOpenAI, "fake-key", opts)
	if err != nil {
		t.Fatalf("Factory(ProviderOpenAI) returned error: %v", err)
	}
	if _, ok := translator.(*OpenAITranslator); !ok {
		t.Errorf("expected *OpenAITranslator, got %T", translator)
	}
}

func TestFactoryRequiresTargetLanguage(t *testing.T) {
	ctx := context.Background()
	opts := Options{} // no TargetLanguage
	_, err := Factory(ctx, ProviderGemini, "fake-key", opts)
	if err == nil {
		t.Error("expected error for missing target language")
	}
}

func TestFactoryRejectsUnknownProvider(t *testing.T) {
	ctx := context.Background()
	opts := Options{TargetLanguage: "French"}
	_, err := Factory(ctx, Provider("unknown"), "fake-key", opts)
	if err == nil {
		t.Error("expected error for unknown provider")
	}
}

func TestGeminiTranslatorImplementsConcurrentTranslator(t *testing.T) {
	ctx := context.Background()
	opts := Options{TargetLanguage: "Korean"}
	translator, err := Factory(ctx, ProviderGemini, "fake-key", opts)
	if err != nil {
		t.Fatalf("Factory error: %v", err)
	}
	if _, ok := translator.(ConcurrentTranslator); !ok {
		t.Error("GeminiTranslator should implement ConcurrentTranslator")
	}
}

func TestOpenAITranslatorImplementsConcurrentTranslator(t *testing.T) {
	ctx := context.Background()
	opts := Options{TargetLanguage: "German"}
	translator, err := Factory(ctx, ProviderOpenAI, "fake-key", opts)
	if err != nil {
		t.Fatalf("Factory error: %v", err)
	}
	if _, ok := translator.(ConcurrentTranslator); !ok {
		t.Error("OpenAITranslator should implement ConcurrentTranslator")
	}
}

func TestFactoryReturnsAnthropicTranslator(t *testing.T) {
	ctx := context.Background()
	opts := Options{TargetLanguage: "French"}
	translator, err := Factory(ctx, ProviderAnthropic, "fake-key", opts)
	if err != nil {
		t.Fatalf("Factory(ProviderAnthropic) returned error: %v", err)
	}
	if _, ok := translator.(*AnthropicTranslator); !ok {
		t.Errorf("expected *AnthropicTranslator, got %T", translator)
	}
}

func TestAnthropicTranslatorImplementsConcurrentTranslator(t *testing.T) {
	ctx := context.Background()
	opts := Options{TargetLanguage: "Italian"}
	translator, err := Factory(ctx, ProviderAnthropic, "fake-key", opts)
	if err != nil {
		t.Fatalf("Factory error: %v", err)
	}
	if _, ok := translator.(ConcurrentTranslator); !ok {
		t.Error("AnthropicTranslator should implement ConcurrentTranslator")
	}
}

// Integration test: only runs if OPENAI_API_KEY is set
func TestOpenAITranslatorIntegration(t *testing.T) {
	apiKey := os.Getenv("OPENAI_API_KEY")
	if apiKey == "" {
		t.Skip("OPENAI_API_KEY not set; skipping integration test")
	}

	ctx := context.Background()
	opts := Options{TargetLanguage: "Spanish"}
	translator, err := NewOpenAITranslator(ctx, apiKey, opts)
	if err != nil {
		t.Fatalf("NewOpenAITranslator error: %v", err)
	}

	items := []TranslationItem{
		{Index: 0, Text: "Hello"},
		{Index: 1, Text: "Goodbye"},
	}

	results, err := translator.Translate(ctx, items)
	if err != nil {
		t.Fatalf("Translate error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Text == "" {
			t.Errorf("result index %d has empty text", r.Index)
		}
	}
}

// Integration test: only runs if ANTHROPIC_API_KEY is set
func TestAnthropicTranslatorIntegration(t *testing.T) {
	apiKey := os.Getenv("ANTHROPIC_API_KEY")
	if apiKey == "" {
		t.Skip("ANTHROPIC_API_KEY not set; skipping integration test")
	}

	ctx := context.Background()
	opts := Options{TargetLanguage: "Japanese"}
	translator, err := NewAnthropicTranslator(ctx, apiKey, opts)
	if err != nil {
		t.Fatalf("NewAnthropicTranslator error: %v", err)
	}

	items := []TranslationItem{
		{Index: 0, Text: "Hello"},
		{Index: 1, Text: "Goodbye"},
	}

	results, err := translator.Translate(ctx, items)
	if err != nil {
		t.Fatalf("Translate error: %v", err)
	}
	if len(results) != 2 {
		t.Errorf("expected 2 results, got %d", len(results))
	}
	for _, r := range results {
		if r.Text == "" {
			t.Errorf("result index %d has empty text", r.Index)
		}
	}
}
