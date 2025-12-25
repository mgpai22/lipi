package transcribe

import (
	"context"
	"time"

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

// transcription service provider
type Provider string

const (
	ProviderWhisper Provider = "whisper"
	ProviderOpenAI  Provider = "openai"
	ProviderGemini  Provider = "gemini"
)

// transcription options
type Options struct {
	Language string
	Model    string
	Prompt   string
}

// creates transcriber based on provider
func Factory(
	provider Provider,
	apiKey string,
	opts Options,
) (Transcriber, error) {
	// TODO: Implement provider factory
	// switch provider {
	// case ProviderWhisper:
	//     return NewWhisperTranscriber(opts)
	// case ProviderOpenAI:
	//     return NewOpenAITranscriber(apiKey, opts)
	// }
	return nil, nil
}
