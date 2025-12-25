package transcribe

import (
	"testing"
)

func TestExtractTranscriptSegments(t *testing.T) {
	tests := []struct {
		name      string
		input     string
		wantCount int
		wantErr   bool
	}{
		{
			name: "plain valid array",
			input: `[
				{"start": 0.0, "end": 2.5, "text": "Hello world"},
				{"start": 2.5, "end": 5.0, "text": "How are you"}
			]`,
			wantCount: 2,
		},
		{
			name: "preamble with valid array",
			input: `Here is the JSON transcript:
			[
				{"start": 0.0, "end": 2.5, "text": "Hello world"},
				{"start": 2.5, "end": 5.0, "text": "How are you"}
			]`,
			wantCount: 2,
		},
		{
			name: "valid array with trailing text",
			input: `[
				{"start": 0.0, "end": 2.5, "text": "Hello world"}
			]
			I hope this helps! Let me know if you need anything else.`,
			wantCount: 1,
		},
		{
			name: "preamble and trailing text",
			input: `Here is your transcript:
			[{"start": 1.0, "end": 3.0, "text": "Test segment"}]
			That's all!`,
			wantCount: 1,
		},
		{
			name:      "code fenced JSON (after cleanJSONResponse)",
			input:     `[{"start": 0.0, "end": 1.5, "text": "Fenced content"}]`,
			wantCount: 1,
		},
		{
			name: "wrapper object with segments key",
			input: `{"segments": [
				{"start": 0.0, "end": 2.0, "text": "Wrapped segment"}
			]}`,
			wantCount: 1,
		},
		{
			name: "wrapper object with transcript key",
			input: `{"transcript": [
				{"start": 0.0, "end": 2.0, "text": "From transcript key"}
			]}`,
			wantCount: 1,
		},
		{
			name: "wrapper object with data key",
			input: `{"data": [
				{"start": 0.0, "end": 2.0, "text": "From data key"}
			]}`,
			wantCount: 1,
		},
		{
			name: "wrapper object with unknown key",
			input: `{"myCustomKey": [
				{"start": 0.0, "end": 2.0, "text": "From unknown key"}
			]}`,
			wantCount: 1,
		},
		{
			name: "unrelated object first then transcript array",
			input: `{"status": "ok", "count": 5}
			[{"start": 0.0, "end": 2.0, "text": "Real transcript"}]`,
			wantCount: 1,
		},
		{
			name: "multiple arrays picks first valid",
			input: `[1, 2, 3]
			[{"start": 0.0, "end": 2.0, "text": "Actual transcript"}]`,
			wantCount: 1,
		},
		{
			name:    "empty array",
			input:   `[]`,
			wantErr: true,
		},
		{
			name:    "no JSON at all",
			input:   `This is just plain text with no JSON content.`,
			wantErr: true,
		},
		{
			name:    "invalid JSON",
			input:   `[{"start": 0.0, "end": 2.0, "text": "incomplete"`,
			wantErr: true,
		},
		{
			name:    "array with empty segments",
			input:   `[{"start": 0, "end": 0, "text": ""}]`,
			wantErr: true,
		},
		{
			name:      "array with valid timestamps but empty text",
			input:     `[{"start": 1.0, "end": 2.0, "text": ""}]`,
			wantCount: 1,
		},
		{
			name: "complex preamble with explanation",
			input: `I've analyzed the audio and created a transcript for you. The audio appears to be in English. Here is the formatted JSON output:

			[
				{"start": 0.0, "end": 3.5, "text": "Welcome to the show"},
				{"start": 3.5, "end": 7.2, "text": "Today we'll be discussing AI"}
			]

			Note: Timestamps are in seconds. Let me know if you need any adjustments!`,
			wantCount: 2,
		},
		{
			name: "nested wrapper object",
			input: `{
				"response": {
					"segments": [{"start": 0.0, "end": 1.0, "text": "Nested"}]
				}
			}`,
			wantCount: 1,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := extractTranscriptSegments(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Error("expected error but got none")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if len(segments) != tt.wantCount {
				t.Errorf(
					"got %d segments, want %d",
					len(segments),
					tt.wantCount,
				)
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
			input: `[{"start": 0, "end": 1, "text": "hello"}]`,
			want:  `[{"start": 0, "end": 1, "text": "hello"}]`,
		},
		{
			name:  "json code fence",
			input: "```json\n[{\"start\": 0, \"end\": 1, \"text\": \"hello\"}]\n```",
			want:  `[{"start": 0, "end": 1, "text": "hello"}]`,
		},
		{
			name:  "plain code fence",
			input: "```\n[{\"start\": 0, \"end\": 1, \"text\": \"hello\"}]\n```",
			want:  `[{"start": 0, "end": 1, "text": "hello"}]`,
		},
		{
			name:  "with leading/trailing whitespace",
			input: "  \n\n```json\n[{\"start\": 0}]\n```\n\n  ",
			want:  `[{"start": 0}]`,
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

func TestValidateSegments(t *testing.T) {
	tests := []struct {
		name     string
		segments []transcriptSegment
		want     bool
	}{
		{"empty slice", []transcriptSegment{}, false},
		{"nil slice", nil, false},
		{"segment with text", []transcriptSegment{{Text: "hello"}}, true},
		{"segment with start time", []transcriptSegment{{Start: 1.0}}, true},
		{"segment with end time", []transcriptSegment{{End: 2.0}}, true},
		{
			"all zero segment",
			[]transcriptSegment{{Start: 0, End: 0, Text: ""}},
			false,
		},
		{
			"multiple segments one valid",
			[]transcriptSegment{{}, {Start: 1.0, End: 2.0, Text: "valid"}},
			true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := validateSegments(tt.segments); got != tt.want {
				t.Errorf("validateSegments() = %v, want %v", got, tt.want)
			}
		})
	}
}
