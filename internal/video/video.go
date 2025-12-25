package video

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"time"

	ffmpeg "github.com/u2takey/ffmpeg-go"
)

// video file information
type Info struct {
	Path      string
	Duration  time.Duration
	Width     int
	Height    int
	FrameRate float64
	Codec     string
	HasAudio  bool
}

// defines interface for video processing operations
type Processor interface {
	// extracts audio from video file
	ExtractAudio(
		ctx context.Context,
		videoPath, outputPath string,
		opts ExtractAudioOptions,
	) error

	// retrieves video file information
	GetInfo(ctx context.Context, videoPath string) (*Info, error)
}

// holds options for audio extraction
type ExtractAudioOptions struct {
	Format     string // Output format (wav, mp3, aac, flac)
	SampleRate int    // Sample rate in Hz (e.g., 16000, 44100, 48000)
	Channels   int    // Number of channels (1 = mono, 2 = stereo)
	Bitrate    string // Bitrate for lossy formats (e.g., "128k", "320k")
}

// returns sensible defaults for audio extraction
func DefaultExtractAudioOptions() ExtractAudioOptions {
	return ExtractAudioOptions{
		Format:     "wav",
		SampleRate: 16000,
		Channels:   1,
	}
}

// holds options for subtitle embedding
type EmbedOptions struct {
	FontSize  int
	FontColor string
	Position  string
	Opacity   float64
	Style     string
}

// default implementation using ffmpeg
type DefaultProcessor struct {
	tempDir string
}

func NewProcessor(tempDir string) *DefaultProcessor {
	return &DefaultProcessor{
		tempDir: tempDir,
	}
}

// extracts audio from video file
func (p *DefaultProcessor) ExtractAudio(
	ctx context.Context,
	videoPath, outputPath string,
	opts ExtractAudioOptions,
) error {
	if _, err := os.Stat(videoPath); os.IsNotExist(err) {
		return fmt.Errorf("video file not found: %s", videoPath)
	}

	outputDir := filepath.Dir(outputPath)
	if err := os.MkdirAll(outputDir, 0755); err != nil {
		return fmt.Errorf("failed to create output directory: %w", err)
	}

	kwargs := ffmpeg.KwArgs{
		"vn": "",              // No video
		"ar": opts.SampleRate, // Sample rate
		"ac": opts.Channels,   // Channels
		"y":  "",              // Overwrite output
	}

	switch opts.Format {
	case "mp3":
		kwargs["acodec"] = "libmp3lame"
		if opts.Bitrate != "" {
			kwargs["b:a"] = opts.Bitrate
		}
	case "aac":
		kwargs["acodec"] = "aac"
		if opts.Bitrate != "" {
			kwargs["b:a"] = opts.Bitrate
		}
	case "flac":
		kwargs["acodec"] = "flac"
	case "wav":
		kwargs["acodec"] = "pcm_s16le"
	default:
		kwargs["acodec"] = "pcm_s16le"
	}

	err := ffmpeg.Input(videoPath).
		Output(outputPath, kwargs).
		OverWriteOutput().
		Run()

	if err != nil {
		return fmt.Errorf("ffmpeg extraction failed: %w", err)
	}

	return nil
}

// retrieves video file information
func (p *DefaultProcessor) GetInfo(
	ctx context.Context,
	videoPath string,
) (*Info, error) {
	//TODO: Implement
	return nil, nil
}
