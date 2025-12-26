package transcribe

import (
	"testing"
	"time"
)

func TestParseVerboseJSONResponse(t *testing.T) {
	transcriber := &OpenAITranscriber{}

	tests := []struct {
		name             string
		rawJSON          string
		fallbackDuration time.Duration
		wantCount        int
		wantErr          bool
	}{
		{
			name: "valid verbose_json with segments",
			rawJSON: `{
				"text": "Hello world. How are you today?",
				"segments": [
					{"start": 0.0, "end": 1.5, "text": "Hello world."},
					{"start": 1.5, "end": 3.0, "text": "How are you today?"}
				],
				"language": "en",
				"duration": 3.0
			}`,
			fallbackDuration: 5 * time.Second,
			wantCount:        2,
		},
		{
			name: "verbose_json with no segments but has text",
			rawJSON: `{
				"text": "This is a transcription without segments.",
				"segments": [],
				"language": "en",
				"duration": 2.5
			}`,
			fallbackDuration: 5 * time.Second,
			wantCount:        1,
		},
		{
			name: "verbose_json with null segments",
			rawJSON: `{
				"text": "Transcription text only.",
				"segments": null,
				"language": "en",
				"duration": 1.0
			}`,
			fallbackDuration: 5 * time.Second,
			wantCount:        1,
		},
		{
			name: "verbose_json with empty text segments filtered out",
			rawJSON: `{
				"text": "Hello world",
				"segments": [
					{"start": 0.0, "end": 0.5, "text": ""},
					{"start": 0.5, "end": 1.5, "text": "Hello world"},
					{"start": 1.5, "end": 2.0, "text": "   "}
				],
				"language": "en",
				"duration": 2.0
			}`,
			fallbackDuration: 5 * time.Second,
			wantCount:        1,
		},
		{
			name: "verbose_json with whitespace-padded text",
			rawJSON: `{
				"text": "  Trimmed text  ",
				"segments": [
					{"start": 0.0, "end": 1.0, "text": "  Trimmed text  "}
				],
				"language": "en",
				"duration": 1.0
			}`,
			fallbackDuration: 5 * time.Second,
			wantCount:        1,
		},
		{
			name:             "empty response",
			rawJSON:          "",
			fallbackDuration: 5 * time.Second,
			wantErr:          true,
		},
		{
			name:             "invalid JSON",
			rawJSON:          `{"text": "incomplete`,
			fallbackDuration: 5 * time.Second,
			wantErr:          true,
		},
		{
			name: "no segments and no text",
			rawJSON: `{
				"text": "",
				"segments": [],
				"language": "en",
				"duration": 0
			}`,
			fallbackDuration: 5 * time.Second,
			wantErr:          true,
		},
		{
			name: "real whisper response format",
			rawJSON: `{
				"task": "transcribe",
				"language": "english",
				"duration": 8.470000267028809,
				"text": "The stale smell of old beer lingers. It takes heat to bring out the odor.",
				"segments": [
					{
						"id": 0,
						"seek": 0,
						"start": 0.0,
						"end": 3.319999933242798,
						"text": "The stale smell of old beer lingers.",
						"tokens": [50364, 440, 23025, 7966, 295, 1331, 8388, 22949, 404, 13, 50530],
						"temperature": 0.0,
						"avg_logprob": -0.2860786020755768,
						"compression_ratio": 1.2363636493682861,
						"no_speech_prob": 0.009231
					},
					{
						"id": 1,
						"seek": 0,
						"start": 3.319999933242798,
						"end": 6.190000057220459,
						"text": "It takes heat to bring out the odor.",
						"tokens": [50530, 467, 2516, 3738, 281, 1565, 484, 264, 10602, 13, 50673],
						"temperature": 0.0,
						"avg_logprob": -0.2860786020755768,
						"compression_ratio": 1.2363636493682861,
						"no_speech_prob": 0.009231
					}
				]
			}`,
			fallbackDuration: 10 * time.Second,
			wantCount:        2,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			segments, err := transcriber.parseVerboseJSONResponse(
				tt.rawJSON,
				tt.fallbackDuration,
			)
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

			// Verify segments have non-empty text
			for i, seg := range segments {
				if seg.Text == "" {
					t.Errorf("segment %d has empty text", i)
				}
			}
		})
	}
}

func TestParseVerboseJSONResponseTimestamps(t *testing.T) {
	transcriber := &OpenAITranscriber{}

	rawJSON := `{
		"text": "Hello world. Goodbye.",
		"segments": [
			{"start": 1.5, "end": 3.0, "text": "Hello world."},
			{"start": 3.0, "end": 5.5, "text": "Goodbye."}
		],
		"language": "en",
		"duration": 5.5
	}`

	segments, err := transcriber.parseVerboseJSONResponse(
		rawJSON,
		10*time.Second,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(segments) != 2 {
		t.Fatalf("expected 2 segments, got %d", len(segments))
	}

	// Check first segment timestamps
	if segments[0].StartTime != 1500*time.Millisecond {
		t.Errorf(
			"segment 0 start time: got %v, want 1.5s",
			segments[0].StartTime,
		)
	}
	if segments[0].EndTime != 3*time.Second {
		t.Errorf("segment 0 end time: got %v, want 3s", segments[0].EndTime)
	}
	if segments[0].Text != "Hello world." {
		t.Errorf(
			"segment 0 text: got %q, want %q",
			segments[0].Text,
			"Hello world.",
		)
	}

	// Check second segment timestamps
	if segments[1].StartTime != 3*time.Second {
		t.Errorf("segment 1 start time: got %v, want 3s", segments[1].StartTime)
	}
	if segments[1].EndTime != 5500*time.Millisecond {
		t.Errorf("segment 1 end time: got %v, want 5.5s", segments[1].EndTime)
	}
	if segments[1].Text != "Goodbye." {
		t.Errorf(
			"segment 1 text: got %q, want %q",
			segments[1].Text,
			"Goodbye.",
		)
	}
}

func TestShouldUseTranslation(t *testing.T) {
	tests := []struct {
		transcriptLang string
		want           bool
	}{
		{"english", true},
		{"English", true},
		{"ENGLISH", true},
		{"en", true},
		{"EN", true},
		{" english ", true},
		{"native", false},
		{"", false},
		{"spanish", false},
		{"japanese", false},
	}

	for _, tt := range tests {
		t.Run(tt.transcriptLang, func(t *testing.T) {
			transcriber := &OpenAITranscriber{
				options: Options{
					TranscriptLanguage: tt.transcriptLang,
				},
			}
			got := transcriber.shouldUseTranslation()
			if got != tt.want {
				t.Errorf("shouldUseTranslation() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestFallbackSingleSegment(t *testing.T) {
	transcriber := &OpenAITranscriber{}

	// Test case where response has text but no segments array
	rawJSON := `{
		"text": "This is a transcription without segments.",
		"duration": 10.5
	}`

	segments, err := transcriber.parseVerboseJSONResponse(
		rawJSON,
		15*time.Second,
	)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if len(segments) != 1 {
		t.Fatalf("expected 1 fallback segment, got %d", len(segments))
	}

	if segments[0].StartTime != 0 {
		t.Errorf(
			"fallback segment start time should be 0, got %v",
			segments[0].StartTime,
		)
	}

	// Duration from response should be used
	expectedEnd := time.Duration(10.5 * float64(time.Second))
	if segments[0].EndTime != expectedEnd {
		t.Errorf(
			"fallback segment end time: got %v, want %v",
			segments[0].EndTime,
			expectedEnd,
		)
	}

	if segments[0].Text != "This is a transcription without segments." {
		t.Errorf("fallback segment text incorrect: %q", segments[0].Text)
	}
}
