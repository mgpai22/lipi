package translate

import (
	"testing"
)

func TestExtractTranslationResults(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantErr   bool
	}{
		{
			name: "plain valid array",
			input: `[
				{"index": 0, "text": "こんにちは"},
				{"index": 1, "text": "さようなら"}
			]`,
			wantCount: 2,
		},
		{
			name: "preamble with valid array",
			input: `Here is the translation:
			[
				{"index": 0, "text": "Bonjour"},
				{"index": 1, "text": "Au revoir"}
			]`,
			wantCount: 2,
		},
		{
			name: "valid array with trailing text",
			input: `[
				{"index": 0, "text": "Hola"}
			]
			I hope this helps!`,
			wantCount: 1,
		},
		{
			name:      "code fenced JSON",
			input:     `[{"index": 0, "text": "翻訳されたテキスト"}]`,
			wantCount: 1,
		},
		{
			name: "wrapper object with results key",
			input: `{"results": [
				{"index": 0, "text": "Translated"}
			]}`,
			wantCount: 1,
		},
		{
			name: "wrapper object with translations key",
			input: `{"translations": [
				{"index": 0, "text": "Übersetzt"}
			]}`,
			wantCount: 1,
		},
		{
			name: "wrapper object with data key",
			input: `{"data": [
				{"index": 0, "text": "Переведено"}
			]}`,
			wantCount: 1,
		},
		{
			name:    "empty array",
			input:   `[]`,
			wantErr: true,
		},
		{
			name:    "no JSON at all",
			input:   `This is just plain text.`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `[{"index": 0, "text": "incomplete"`,
			wantErr: true,
		},
		{
			name:    "array with empty text",
			input:   `[{"index": 0, "text": ""}]`,
			wantErr: true,
		},
		{
			name: "complex preamble",
			input: `I've translated the subtitles for you. Here is the JSON:

			[
				{"index": 0, "text": "First translation"},
				{"index": 1, "text": "Second translation"}
			]

			Let me know if you need anything else!`,
			wantCount: 2,
		},
		{
			name: "SRT newline escape in text",
			input: `[
				{"index": 0, "text": "That's why they are fuming...\Nthese Babu and Pappu."}
			]`,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			results, err := extractTranslationResults(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(results) != tt.wantCount {
				t.Errorf("got %d results, want %d", len(results), tt.wantCount)
			}
		})
	}
}

func TestCleanJSONResponse(t *testing.T) {
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{
			name:  "plain JSON",
			input: `[{"index": 0, "text": "hello"}]`,
			want:  `[{"index": 0, "text": "hello"}]`,
		},
		{
			name:  "json code fence",
			input: "```json\n[{\"index\": 0, \"text\": \"hello\"}]\n```",
			want:  `[{"index": 0, "text": "hello"}]`,
		},
		{
			name:  "plain code fence",
			input: "```\n[{\"index\": 0, \"text\": \"hello\"}]\n```",
			want:  `[{"index": 0, "text": "hello"}]`,
		},
		{
			name:  "with leading/trailing whitespace",
			input: "  \n\n```json\n[{\"index\": 0}]\n```\n\n  ",
			want:  `[{"index": 0}]`,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := cleanJSONResponse(tt.input); got != tt.want {
				t.Errorf("cleanJSONResponse() = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestValidateResults(t *testing.T) {
	tests := []struct {
		name    string
		results []TranslationResult
		want    bool
	}{
		{"empty slice", []TranslationResult{}, false},
		{"nil slice", nil, false},
		{
			"result with text",
			[]TranslationResult{{Index: 0, Text: "hello"}},
			true,
		},
		{
			"result with empty text",
			[]TranslationResult{{Index: 0, Text: ""}},
			false,
		},
		{
			"multiple results one valid",
			[]TranslationResult{
				{Index: 0, Text: ""},
				{Index: 1, Text: "valid"},
			},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateResults(tt.results); got != tt.want {
				t.Errorf("validateResults() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestBuildPrompt(t *testing.T) {
	translator := &GeminiTranslator{
		options: Options{
			InputLanguage:  "English",
			TargetLanguage: "Japanese",
		},
	}

	items := []TranslationItem{
		{Index: 0, Text: "Hello world"},
		{Index: 1, Text: "Goodbye"},
	}

	prompt := BuildPrompt(translator.options, items)

	if !contains(prompt, "English subtitle texts") {
		t.Error("prompt should contain input language")
	}
	if !contains(prompt, "to Japanese") {
		t.Error("prompt should contain target language")
	}
	if !contains(prompt, "Hello world") {
		t.Error("prompt should contain input text")
	}
	if !contains(prompt, `"index": 0`) {
		t.Error("prompt should contain index")
	}
}

func TestBuildPromptWithoutInputLanguage(t *testing.T) {
	translator := &GeminiTranslator{
		options: Options{
			TargetLanguage: "Spanish",
		},
	}

	items := []TranslationItem{
		{Index: 0, Text: "Hello"},
	}

	prompt := BuildPrompt(translator.options, items)

	if contains(prompt, "English") || contains(prompt, "from ") {
		t.Error("prompt should not contain input language when not specified")
	}
	if !contains(prompt, "to Spanish") {
		t.Error("prompt should contain target language")
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) &&
		(s == substr || len(s) > 0 && containsHelper(s, substr))
}

func containsHelper(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}
