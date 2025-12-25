package subtitle

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestParseSRTFile(t *testing.T) {
	content := `1
00:00:01,000 --> 00:00:04,000
Hello, world!

2
00:00:05,500 --> 00:00:08,200
This is a test.
With multiple lines.

3
00:00:10,000 --> 00:00:12,500
Final subtitle.
`
	tmpDir := t.TempDir()
	srtPath := filepath.Join(tmpDir, "test.srt")
	if err := os.WriteFile(srtPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	file, err := Open(srtPath)
	if err != nil {
		t.Fatalf("failed to open SRT file: %v", err)
	}

	if file.Format() != FormatSRT {
		t.Errorf("expected format SRT, got %s", file.Format())
	}

	sub := file.Subtitle()
	if len(sub.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(sub.Entries))
	}

	if sub.Entries[0].StartTime != 1*time.Second {
		t.Errorf(
			"entry 0: expected start 1s, got %v",
			sub.Entries[0].StartTime,
		)
	}
	if sub.Entries[0].EndTime != 4*time.Second {
		t.Errorf("entry 0: expected end 4s, got %v", sub.Entries[0].EndTime)
	}
	if sub.Entries[0].Text != "Hello, world!" {
		t.Errorf(
			"entry 0: expected 'Hello, world!', got %q",
			sub.Entries[0].Text,
		)
	}

	expectedText := "This is a test.\nWith multiple lines."
	if sub.Entries[1].Text != expectedText {
		t.Errorf(
			"entry 1: expected %q, got %q",
			expectedText,
			sub.Entries[1].Text,
		)
	}

	// Test SetText
	if err := file.SetText(0, "Modified text"); err != nil {
		t.Errorf("SetText failed: %v", err)
	}
	if file.Subtitle().Entries[0].Text != "Modified text" {
		t.Errorf("SetText did not update text")
	}
}

func TestParseVTTFile(t *testing.T) {
	content := `WEBVTT

1
00:00:01.000 --> 00:00:04.000
Hello, world!

2
00:00:05.500 --> 00:00:08.200
This is a test.
With multiple lines.

00:00:10.000 --> 00:00:12.500
No cue identifier.
`
	tmpDir := t.TempDir()
	vttPath := filepath.Join(tmpDir, "test.vtt")
	if err := os.WriteFile(vttPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	file, err := Open(vttPath)
	if err != nil {
		t.Fatalf("failed to open VTT file: %v", err)
	}

	if file.Format() != FormatVTT {
		t.Errorf("expected format VTT, got %s", file.Format())
	}

	sub := file.Subtitle()
	if len(sub.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(sub.Entries))
	}

	if sub.Entries[0].StartTime != 1*time.Second {
		t.Errorf(
			"entry 0: expected start 1s, got %v",
			sub.Entries[0].StartTime,
		)
	}
	if sub.Entries[0].Text != "Hello, world!" {
		t.Errorf(
			"entry 0: expected 'Hello, world!', got %q",
			sub.Entries[0].Text,
		)
	}

	if sub.Entries[2].Text != "No cue identifier." {
		t.Errorf(
			"entry 2: expected 'No cue identifier.', got %q",
			sub.Entries[2].Text,
		)
	}
}

func TestParseASSFile(t *testing.T) {
	content := `[Script Info]
Title: Test Subtitles
ScriptType: v4.00+

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Hello, world!
Dialogue: 0,0:00:05.50,0:00:08.20,Default,,0,0,0,,{\pos(100,200)}This has positioning.
Dialogue: 0,0:00:10.00,0:00:12.50,Default,,0,0,0,,Line with\Nnewline.
`
	tmpDir := t.TempDir()
	assPath := filepath.Join(tmpDir, "test.ass")
	if err := os.WriteFile(assPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	file, err := Open(assPath)
	if err != nil {
		t.Fatalf("failed to open ASS file: %v", err)
	}

	if file.Format() != FormatASS {
		t.Errorf("expected format ASS, got %s", file.Format())
	}

	sub := file.Subtitle()
	if len(sub.Entries) != 3 {
		t.Fatalf("expected 3 entries, got %d", len(sub.Entries))
	}

	// Check first entry
	if sub.Entries[0].StartTime != 1*time.Second {
		t.Errorf(
			"entry 0: expected start 1s, got %v",
			sub.Entries[0].StartTime,
		)
	}
	if sub.Entries[0].Text != "Hello, world!" {
		t.Errorf(
			"entry 0: expected 'Hello, world!', got %q",
			sub.Entries[0].Text,
		)
	}

	// check second entry (has positioning tags - should be in subtitle but text still readable)
	// generic Subtitle() returns text with \N converted to \n
	if !strings.Contains(sub.Entries[1].Text, "This has positioning") {
		t.Errorf(
			"entry 1: expected text containing 'This has positioning', got %q",
			sub.Entries[1].Text,
		)
	}

	// check third entry (has \N which should be converted to newline)
	if sub.Entries[2].Text != "Line with\nnewline." {
		t.Errorf(
			"entry 2: expected 'Line with\\nnewline.', got %q",
			sub.Entries[2].Text,
		)
	}
}

func TestASSFilePreservesStyles(t *testing.T) {
	content := `[Script Info]
Title: Test Subtitles
ScriptType: v4.00+
PlayDepth: 0

[V4+ Styles]
Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding
Style: Default,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1
Style: Italic,Arial,20,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,1,0,0,100,100,0,0,1,2,2,2,10,10,10,1

[Events]
Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text
Dialogue: 0,0:00:01.00,0:00:04.00,Default,,0,0,0,,Original text
Dialogue: 0,0:00:05.00,0:00:08.00,Italic,,0,0,0,,{\pos(100,200)}Tagged text
`
	tmpDir := t.TempDir()
	assPath := filepath.Join(tmpDir, "test.ass")
	if err := os.WriteFile(assPath, []byte(content), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	file, err := Open(assPath)
	if err != nil {
		t.Fatalf("failed to open ASS file: %v", err)
	}

	assFile, ok := file.(*ASSFile)
	if !ok {
		t.Fatalf("expected *ASSFile, got %T", file)
	}

	if err := assFile.SetText(0, "Translated text"); err != nil {
		t.Fatalf("SetText failed: %v", err)
	}

	// set overlay on second entry
	if err := assFile.SetTextWithOverlay(1, "翻訳されたテキスト"); err != nil {
		t.Fatalf("SetTextWithOverlay failed: %v", err)
	}

	outPath := filepath.Join(tmpDir, "output.ass")
	if err := assFile.Write(outPath); err != nil {
		t.Fatalf("Write failed: %v", err)
	}

	outContent, err := os.ReadFile(outPath)
	if err != nil {
		t.Fatalf("failed to read output: %v", err)
	}

	outStr := string(outContent)

	// check that styles are preserved
	if !strings.Contains(outStr, "Style: Default,Arial,20") {
		t.Error("Default style not preserved")
	}
	if !strings.Contains(outStr, "Style: Italic,Arial,20") {
		t.Error("Italic style not preserved")
	}

	// check that first entry was modified
	if !strings.Contains(outStr, "Translated text") {
		t.Error("First entry text not updated")
	}

	// check that second entry has overlay with preserved positioning
	if !strings.Contains(outStr, "{\\pos(100,200)}翻訳されたテキスト\\NTagged text") {
		t.Errorf("Second entry overlay not correct, got: %s", outStr)
	}

	// check that Italic style is still used for second entry
	if !strings.Contains(outStr, "Dialogue: 0,0:00:05.00,0:00:08.00,Italic") {
		t.Error("Second entry style not preserved")
	}
}

func TestExtractLeadingTags(t *testing.T) {
	tests := []struct {
		input       string
		wantTags    string
		wantContent string
	}{
		{
			input:       "Hello world",
			wantTags:    "",
			wantContent: "Hello world",
		},
		{
			input:       "{\\pos(100,200)}Hello world",
			wantTags:    "{\\pos(100,200)}",
			wantContent: "Hello world",
		},
		{
			input:       "{\\an8}{\\fs24}Hello world",
			wantTags:    "{\\an8}{\\fs24}",
			wantContent: "Hello world",
		},
		{
			input:       "{\\pos(100,200)}{\\c&HFFFFFF&}Hello {\\i1}world{\\i0}",
			wantTags:    "{\\pos(100,200)}{\\c&HFFFFFF&}",
			wantContent: "Hello {\\i1}world{\\i0}",
		},
	}

	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			gotTags, gotContent := extractLeadingTags(tt.input)
			if gotTags != tt.wantTags {
				t.Errorf("tags: got %q, want %q", gotTags, tt.wantTags)
			}
			if gotContent != tt.wantContent {
				t.Errorf("content: got %q, want %q", gotContent, tt.wantContent)
			}
		})
	}
}

func TestOpenUnsupportedFormat(t *testing.T) {
	tmpDir := t.TempDir()
	txtPath := filepath.Join(tmpDir, "test.txt")
	if err := os.WriteFile(txtPath, []byte("test"), 0644); err != nil {
		t.Fatalf("failed to write test file: %v", err)
	}

	_, err := Open(txtPath)
	if err == nil {
		t.Error("expected error for unsupported format")
	}
	if !strings.Contains(err.Error(), "unsupported") {
		t.Errorf("expected 'unsupported' in error, got: %v", err)
	}
}
