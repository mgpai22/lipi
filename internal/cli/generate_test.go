package cli

import "testing"

func TestIsValidOpenAITranscriptLanguage(t *testing.T) {
	tests := []struct {
		lang string
		want bool
	}{
		// Valid cases
		{"", true},
		{"native", true},
		{"Native", true},
		{"NATIVE", true},
		{" native ", true},
		{"english", true},
		{"English", true},
		{"ENGLISH", true},
		{" english ", true},
		{"en", true},
		{"EN", true},
		{" en ", true},

		// Invalid cases - non-English languages
		{"spanish", false},
		{"Spanish", false},
		{"french", false},
		{"german", false},
		{"japanese", false},
		{"chinese", false},
		{"korean", false},
		{"es", false},
		{"fr", false},
		{"de", false},
		{"ja", false},
		{"zh", false},
	}

	for _, tt := range tests {
		t.Run(tt.lang, func(t *testing.T) {
			got := isValidOpenAITranscriptLanguage(tt.lang)
			if got != tt.want {
				t.Errorf(
					"isValidOpenAITranscriptLanguage(%q) = %v, want %v",
					tt.lang,
					got,
					tt.want,
				)
			}
		})
	}
}
