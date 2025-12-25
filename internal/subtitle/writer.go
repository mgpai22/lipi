package subtitle

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// SubRip format
type SRTWriter struct{}

// WebVTT format
type VTTWriter struct{}

// Advanced SubStation Alpha format
type ASSWriter struct {
	Title    string
	FontName string
	FontSize int
}

func NewWriter(format Format) (Writer, error) {
	switch format {
	case FormatSRT:
		return &SRTWriter{}, nil
	case FormatVTT:
		return &VTTWriter{}, nil
	case FormatASS:
		return &ASSWriter{
			Title:    "Lipi Generated Subtitles",
			FontName: "Arial",
			FontSize: 20,
		}, nil
	default:
		return nil, fmt.Errorf("unsupported format: %s", format)
	}
}

// writes the subtitle to an SRT file
func (w *SRTWriter) Write(sub *Subtitle, path string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	var sb strings.Builder
	for i, entry := range sub.Entries {
		// index (1-based)
		sb.WriteString(fmt.Sprintf("%d\n", i+1))

		// timestamps: 00:00:00,000 --> 00:00:00,000
		sb.WriteString(fmt.Sprintf("%s --> %s\n",
			formatSRTTime(entry.StartTime),
			formatSRTTime(entry.EndTime)))

		// text
		sb.WriteString(entry.Text)
		sb.WriteString("\n\n")
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// writes the subtitle to a VTT file
func (w *VTTWriter) Write(sub *Subtitle, path string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	var sb strings.Builder

	// VTT header
	sb.WriteString("WEBVTT\n\n")

	for i, entry := range sub.Entries {
		// optional cue identifier
		sb.WriteString(fmt.Sprintf("%d\n", i+1))

		// timestamps: 00:00:00.000 --> 00:00:00.000
		sb.WriteString(fmt.Sprintf("%s --> %s\n",
			formatVTTTime(entry.StartTime),
			formatVTTTime(entry.EndTime)))

		// text
		sb.WriteString(entry.Text)
		sb.WriteString("\n\n")
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

// writes the subtitle to an ASS file
func (w *ASSWriter) Write(sub *Subtitle, path string) error {
	if err := ensureDir(path); err != nil {
		return err
	}

	var sb strings.Builder

	// script info section
	sb.WriteString("[Script Info]\n")
	sb.WriteString(fmt.Sprintf("Title: %s\n", w.Title))
	sb.WriteString("ScriptType: v4.00+\n")
	sb.WriteString("Collisions: Normal\n")
	sb.WriteString("PlayDepth: 0\n\n")

	// v4+ styles section
	sb.WriteString("[V4+ Styles]\n")
	sb.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	sb.WriteString(fmt.Sprintf("Style: Default,%s,%d,&H00FFFFFF,&H000000FF,&H00000000,&H00000000,0,0,0,0,100,100,0,0,1,2,2,2,10,10,10,1\n\n",
		w.FontName, w.FontSize))

	// events section
	sb.WriteString("[Events]\n")
	sb.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	for _, entry := range sub.Entries {
		// dialogue line
		sb.WriteString(fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n",
			formatASSTime(entry.StartTime),
			formatASSTime(entry.EndTime),
			escapeASSText(entry.Text)))
	}

	return os.WriteFile(path, []byte(sb.String()), 0644)
}

func formatSRTTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	millis := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d,%03d", hours, minutes, seconds, millis)
}

func formatVTTTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	millis := int(d.Milliseconds()) % 1000

	return fmt.Sprintf("%02d:%02d:%02d.%03d", hours, minutes, seconds, millis)
}

func formatASSTime(d time.Duration) string {
	hours := int(d.Hours())
	minutes := int(d.Minutes()) % 60
	seconds := int(d.Seconds()) % 60
	centis := (int(d.Milliseconds()) % 1000) / 10

	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, centis)
}

func escapeASSText(text string) string {
	text = strings.ReplaceAll(text, "\n", "\\N")
	return text
}

func ensureDir(path string) error {
	dir := filepath.Dir(path)
	return os.MkdirAll(dir, 0755)
}

// subtitle format based on file extension
func GetFormatFromExtension(path string) Format {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".srt":
		return FormatSRT
	case ".vtt":
		return FormatVTT
	case ".ass", ".ssa":
		return FormatASS
	default:
		return FormatSRT
	}
}

// file extension for a format
func GetExtensionForFormat(format Format) string {
	switch format {
	case FormatSRT:
		return ".srt"
	case FormatVTT:
		return ".vtt"
	case FormatASS:
		return ".ass"
	default:
		return ".srt"
	}
}
