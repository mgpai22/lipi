package subtitle

import (
	"time"
)

// represents single subtitle entry
type Entry struct {
	Index     int
	StartTime time.Duration
	EndTime   time.Duration
	Text      string
}

// represents complete subtitle track
type Subtitle struct {
	Entries  []Entry
	Language string
	Format   string
}

// represents supported subtitle formats
type Format string

const (
	FormatSRT Format = "srt"
	FormatVTT Format = "vtt"
	FormatASS Format = "ass"
)

// interface for subtitle generation
type Generator interface {
	Generate(segments []Segment) (*Subtitle, error)
}

// represents transcribed audio segment
type Segment struct {
	StartTime time.Duration
	EndTime   time.Duration
	Text      string
}

// interface for writing subtitles to files
type Writer interface {
	Write(subtitle *Subtitle, path string) error
}

// interface for parsing subtitle files
type Parser interface {
	Parse(path string) (*Subtitle, error)
}
