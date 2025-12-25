package subtitle

import (
	"fmt"
	"path/filepath"
	"strings"
)

// parsed subtitle file that preserves format specific metadata
type File interface {
	Format() Format
	Subtitle() *Subtitle
	SetText(index int, text string) error
	Write(path string) error
}

func Open(path string) (File, error) {
	ext := strings.ToLower(filepath.Ext(path))
	switch ext {
	case ".srt":
		return parseSRTFile(path)
	case ".vtt":
		return parseVTTFile(path)
	case ".ass", ".ssa":
		return parseASSFile(path)
	default:
		return nil, fmt.Errorf("unsupported subtitle format: %s", ext)
	}
}
