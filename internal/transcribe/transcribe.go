package transcribe

import (
	"context"
	"fmt"
	"time"

	"github.com/mgpai22/lipi/internal/audio"
	"github.com/mgpai22/lipi/internal/subtitle"
)

// transcription result
type Result struct {
	Segments []subtitle.Segment
	Language string
	Duration time.Duration
}

// interface for audio transcription
type Transcriber interface {
	Transcribe(ctx context.Context, audioPath string) (*Result, error)
}

type ConcurrentTranscriber interface {
	Transcriber
	TranscribeWithChunks(
		ctx context.Context,
		chunks []audio.ChunkInfo,
		concurrency int,
	) (*Result, error)
}

// transcription service provider
type Provider string

const (
	ProviderWhisper Provider = "whisper"
	ProviderOpenAI  Provider = "openai"
	ProviderGemini  Provider = "gemini"
)

// transcription options
type Options struct {
	Language           string // Source language of audio
	TranscriptLanguage string // Output language for transcript (default: "native")
	Model              string
	Prompt             string
}

// creates transcriber based on provider
func Factory(
	ctx context.Context,
	provider Provider,
	apiKey string,
	opts Options,
) (Transcriber, error) {
	switch provider {
	case ProviderGemini:
		return NewGeminiTranscriber(ctx, apiKey, opts)
	case ProviderWhisper:
		return nil, fmt.Errorf("whisper provider not yet implemented")
	case ProviderOpenAI:
		return NewOpenAITranscriber(ctx, apiKey, opts)
	default:
		return nil, fmt.Errorf("unsupported provider: %s", provider)
	}
}
