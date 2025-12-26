package transcribe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mgpai22/lipi/internal/audio"
	"github.com/mgpai22/lipi/internal/subtitle"
	"github.com/openai/openai-go"
	"github.com/openai/openai-go/option"
)

// implements Transcriber interface using OpenAI Audio API
type OpenAITranscriber struct {
	client  openai.Client
	model   string
	options Options
}

// segment from OpenAI Whisper verbose_json response
type whisperSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

// verbose_json response structure from Whisper
type whisperVerboseResponse struct {
	Text     string           `json:"text"`
	Segments []whisperSegment `json:"segments"`
	Language string           `json:"language"`
	Duration float64          `json:"duration"`
}

func NewOpenAITranscriber(
	ctx context.Context,
	apiKey string,
	opts Options,
) (*OpenAITranscriber, error) {
	if apiKey == "" {
		return nil, fmt.Errorf("API key is required")
	}

	client := openai.NewClient(option.WithAPIKey(apiKey))

	model := opts.Model
	if model == "" {
		model = "whisper-1"
	}

	return &OpenAITranscriber{
		client:  client,
		model:   model,
		options: opts,
	}, nil
}

// transcribes single audio file
func (t *OpenAITranscriber) Transcribe(
	ctx context.Context,
	audioPath string,
) (*Result, error) {
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("audio file not found: %s", audioPath)
	}

	file, err := os.Open(audioPath)
	if err != nil {
		return nil, fmt.Errorf("failed to open audio file: %w", err)
	}
	defer file.Close()

	duration, _ := audio.GetDuration(audioPath)

	if t.shouldUseTranslation() {
		return t.transcribeWithTranslation(ctx, file, duration)
	}

	return t.transcribeWithTimestamps(ctx, file, duration)
}

func (t *OpenAITranscriber) shouldUseTranslation() bool {
	lang := strings.ToLower(strings.TrimSpace(t.options.TranscriptLanguage))
	return lang == "english" || lang == "en"
}

func (t *OpenAITranscriber) transcribeWithTranslation(
	ctx context.Context,
	file *os.File,
	duration time.Duration,
) (*Result, error) {
	params := openai.AudioTranslationNewParams{
		File:           file,
		Model:          openai.AudioModel(t.model),
		ResponseFormat: openai.AudioTranslationNewParamsResponseFormatVerboseJSON,
	}

	if t.options.Prompt != "" {
		params.Prompt = openai.String(t.options.Prompt)
	}

	resp, err := t.client.Audio.Translations.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("translation failed: %w", err)
	}

	segments, err := t.parseVerboseJSONResponse(resp.RawJSON(), duration)
	if err != nil {
		segments = []subtitle.Segment{{
			StartTime: 0,
			EndTime:   duration,
			Text:      strings.TrimSpace(resp.Text),
		}}
	}

	return &Result{
		Segments: segments,
		Language: "en",
		Duration: duration,
	}, nil
}

func (t *OpenAITranscriber) transcribeWithTimestamps(
	ctx context.Context,
	file *os.File,
	duration time.Duration,
) (*Result, error) {
	params := openai.AudioTranscriptionNewParams{
		File:                   file,
		Model:                  openai.AudioModel(t.model),
		ResponseFormat:         openai.AudioResponseFormatVerboseJSON,
		TimestampGranularities: []string{"segment"},
	}

	if t.options.Language != "" {
		params.Language = openai.String(t.options.Language)
	}

	if t.options.Prompt != "" {
		params.Prompt = openai.String(t.options.Prompt)
	}

	resp, err := t.client.Audio.Transcriptions.New(ctx, params)
	if err != nil {
		return nil, fmt.Errorf("transcription failed: %w", err)
	}

	segments, err := t.parseVerboseJSONResponse(resp.RawJSON(), duration)
	if err != nil {
		segments = []subtitle.Segment{{
			StartTime: 0,
			EndTime:   duration,
			Text:      strings.TrimSpace(resp.Text),
		}}
	}

	return &Result{
		Segments: segments,
		Language: t.options.Language,
		Duration: duration,
	}, nil
}

func (t *OpenAITranscriber) parseVerboseJSONResponse(
	rawJSON string,
	fallbackDuration time.Duration,
) ([]subtitle.Segment, error) {
	if rawJSON == "" {
		return nil, fmt.Errorf("empty response")
	}

	var verboseResp whisperVerboseResponse
	if err := json.Unmarshal([]byte(rawJSON), &verboseResp); err != nil {
		return nil, fmt.Errorf("failed to parse verbose_json response: %w", err)
	}

	if len(verboseResp.Segments) == 0 {
		if verboseResp.Text == "" {
			return nil, fmt.Errorf("no segments or text in response")
		}
		dur := fallbackDuration
		if verboseResp.Duration > 0 {
			dur = time.Duration(verboseResp.Duration * float64(time.Second))
		}
		return []subtitle.Segment{{
			StartTime: 0,
			EndTime:   dur,
			Text:      strings.TrimSpace(verboseResp.Text),
		}}, nil
	}

	segments := make([]subtitle.Segment, 0, len(verboseResp.Segments))
	for _, seg := range verboseResp.Segments {
		text := strings.TrimSpace(seg.Text)
		if text == "" {
			continue
		}
		segments = append(segments, subtitle.Segment{
			StartTime: time.Duration(seg.Start * float64(time.Second)),
			EndTime:   time.Duration(seg.End * float64(time.Second)),
			Text:      text,
		})
	}

	return segments, nil
}

// transcribes a single chunk and adjusts timestamps
func (t *OpenAITranscriber) TranscribeChunk(
	ctx context.Context,
	chunk audio.ChunkInfo,
) ([]subtitle.Segment, error) {
	result, err := t.Transcribe(ctx, chunk.Path)
	if err != nil {
		return nil, err
	}

	// adjust timestamps based on chunk offset
	adjustedSegments := make([]subtitle.Segment, len(result.Segments))
	for i, seg := range result.Segments {
		adjustedSegments[i] = subtitle.Segment{
			StartTime: seg.StartTime + chunk.StartTime,
			EndTime:   seg.EndTime + chunk.StartTime,
			Text:      seg.Text,
		}
	}

	return adjustedSegments, nil
}

// transcribes multiple chunks in parallel
func (t *OpenAITranscriber) TranscribeWithChunks(
	ctx context.Context,
	chunks []audio.ChunkInfo,
	concurrency int,
) (*Result, error) {
	if len(chunks) == 0 {
		return &Result{}, nil
	}

	if concurrency <= 0 {
		concurrency = 3
	}

	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	workChan := make(chan audio.ChunkInfo)
	resultChan := make(chan chunkResult, len(chunks))

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for {
				select {
				case <-ctx.Done():
					return
				case chunk, ok := <-workChan:
					if !ok {
						return
					}
					if ctx.Err() != nil {
						return
					}

					segments, err := t.TranscribeChunk(ctx, chunk)
					if err != nil {
						cancel()
					}
					resultChan <- chunkResult{
						Index:    chunk.Index,
						Segments: segments,
						Error:    err,
					}
				}
			}
		}()
	}

	go func() {
		defer close(workChan)
		for _, chunk := range chunks {
			select {
			case <-ctx.Done():
				return
			case workChan <- chunk:
			}
		}
	}()

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	results := make([]chunkResult, 0, len(chunks))
	var firstErr error
	for result := range resultChan {
		if result.Error != nil && firstErr == nil {
			firstErr = fmt.Errorf(
				"chunk %d failed: %w",
				result.Index,
				result.Error,
			)
			cancel()
		}
		if result.Error == nil {
			results = append(results, result)
		}
	}
	if firstErr != nil {
		return nil, firstErr
	}

	// sort by index to maintain order
	sort.Slice(results, func(i, j int) bool {
		return results[i].Index < results[j].Index
	})

	// merge
	var allSegments []subtitle.Segment
	for _, r := range results {
		allSegments = append(allSegments, r.Segments...)
	}

	// Calculate total duration from last chunk
	var totalDuration time.Duration
	if len(chunks) > 0 {
		totalDuration = chunks[len(chunks)-1].EndTime
	}

	return &Result{
		Segments: allSegments,
		Language: t.options.Language,
		Duration: totalDuration,
	}, nil
}

func (t *OpenAITranscriber) Close() error {
	return nil
}
