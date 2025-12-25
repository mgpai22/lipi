package transcribe

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/mgpai22/lipi/internal/audio"
	"github.com/mgpai22/lipi/internal/subtitle"
	"google.golang.org/genai"
)

// implements Transcriber interface using Google Gemini
type GeminiTranscriber struct {
	client  *genai.Client
	model   string
	options Options
}

// segment from Gemini's JSON response
type transcriptSegment struct {
	Start float64 `json:"start"`
	End   float64 `json:"end"`
	Text  string  `json:"text"`
}

func NewGeminiTranscriber(ctx context.Context, apiKey string, opts Options) (*GeminiTranscriber, error) {
	client, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey: apiKey,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create Gemini client: %w", err)
	}

	model := opts.Model
	if model == "" {
		model = "gemini-2.5-flash"
	}

	return &GeminiTranscriber{
		client:  client,
		model:   model,
		options: opts,
	}, nil
}

// transcribes single audio file
func (t *GeminiTranscriber) Transcribe(ctx context.Context, audioPath string) (*Result, error) {
	if _, err := os.Stat(audioPath); os.IsNotExist(err) {
		return nil, fmt.Errorf("audio file not found: %s", audioPath)
	}

	uploadedFile, err := t.client.Files.UploadFromPath(ctx, audioPath, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to upload audio file: %w", err)
	}

	defer func() {
		_, _ = t.client.Files.Delete(ctx, uploadedFile.Name, nil)
	}()

	prompt := t.buildTranscriptionPrompt()

	parts := []*genai.Part{
		genai.NewPartFromText(prompt),
		genai.NewPartFromURI(uploadedFile.URI, uploadedFile.MIMEType),
	}
	contents := []*genai.Content{
		genai.NewContentFromParts(parts, genai.RoleUser),
	}

	result, err := t.client.Models.GenerateContent(ctx, t.model, contents, nil)
	if err != nil {
		return nil, fmt.Errorf("transcription failed: %w", err)
	}

	segments, err := t.parseTranscriptionResponse(result)
	if err != nil {
		return nil, fmt.Errorf("failed to parse transcription: %w", err)
	}

	duration, _ := audio.GetDuration(audioPath)

	return &Result{
		Segments: segments,
		Language: t.options.Language,
		Duration: duration,
	}, nil
}

// transcribes a single chunk and adjusts timestamps
func (t *GeminiTranscriber) TranscribeChunk(ctx context.Context, chunk audio.ChunkInfo) ([]subtitle.Segment, error) {
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

// holds the result of transcribing a chunk
type chunkResult struct {
	Index    int
	Segments []subtitle.Segment
	Error    error
}

// transcribes multiple chunks in parallel
func (t *GeminiTranscriber) TranscribeWithChunks(ctx context.Context, chunks []audio.ChunkInfo, concurrency int) (*Result, error) {
	if len(chunks) == 0 {
		return &Result{}, nil
	}

	if concurrency <= 0 {
		concurrency = 3
	}

	workChan := make(chan audio.ChunkInfo, len(chunks))
	resultChan := make(chan chunkResult, len(chunks))

	var wg sync.WaitGroup
	for i := 0; i < concurrency; i++ {
		wg.Go(func() {
			for chunk := range workChan {
				segments, err := t.TranscribeChunk(ctx, chunk)
				resultChan <- chunkResult{
					Index:    chunk.Index,
					Segments: segments,
					Error:    err,
				}
			}
		})
	}

	for _, chunk := range chunks {
		workChan <- chunk
	}
	close(workChan)

	go func() {
		wg.Wait()
		close(resultChan)
	}()

	results := make([]chunkResult, 0, len(chunks))
	for result := range resultChan {
		if result.Error != nil {
			return nil, fmt.Errorf("chunk %d failed: %w", result.Index, result.Error)
		}
		results = append(results, result)
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

	// Calculate total duration from last ch`unk
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

// creates the prompt for transcription
func (t *GeminiTranscriber) buildTranscriptionPrompt() string {
	var sb strings.Builder

	sb.WriteString("Generate a detailed transcript of this audio. ")
	sb.WriteString("For each sentence or phrase, provide the start timestamp, end timestamp, and the exact text spoken. ")
	sb.WriteString("Format your response as a JSON array with objects containing 'start', 'end', and 'text' fields, ")
	sb.WriteString("where 'start' and 'end' are timestamps in seconds (as numbers). ")

	if t.options.Language != "" {
		sb.WriteString(fmt.Sprintf("The audio is in %s. ", t.options.Language))
	}

	if t.options.TranscriptLanguage != "" && t.options.TranscriptLanguage != "native" {
		sb.WriteString(fmt.Sprintf("Output the transcript in %s. ", t.options.TranscriptLanguage))
	}

	if t.options.Prompt != "" {
		sb.WriteString(t.options.Prompt)
		sb.WriteString(" ")
	}

	sb.WriteString("Return ONLY the JSON array, no other text or markdown formatting.")

	return sb.String()
}

// parses Gemini's response into segments
func (t *GeminiTranscriber) parseTranscriptionResponse(result *genai.GenerateContentResponse) ([]subtitle.Segment, error) {
	if result == nil || len(result.Candidates) == 0 {
		return nil, fmt.Errorf("empty response from Gemini")
	}

	var responseText string
	for _, candidate := range result.Candidates {
		if candidate.Content != nil {
			for _, part := range candidate.Content.Parts {
				if part.Text != "" {
					responseText += part.Text
				}
			}
		}
	}

	if responseText == "" {
		return nil, fmt.Errorf("no text in Gemini response")
	}

	responseText = cleanJSONResponse(responseText)

	var transcriptSegments []transcriptSegment
	if err := json.Unmarshal([]byte(responseText), &transcriptSegments); err != nil {
		return nil, fmt.Errorf("failed to parse JSON response: %w (response: %s)", err, truncateString(responseText, 200))
	}

	// convert to subtitle segments
	segments := make([]subtitle.Segment, len(transcriptSegments))
	for i, ts := range transcriptSegments {
		segments[i] = subtitle.Segment{
			StartTime: time.Duration(ts.Start * float64(time.Second)),
			EndTime:   time.Duration(ts.End * float64(time.Second)),
			Text:      strings.TrimSpace(ts.Text),
		}
	}

	return segments, nil
}

// removes markdown formatting from the response
func cleanJSONResponse(s string) string {
	s = strings.TrimSpace(s)

	// remove ```json and ``` markers
	jsonBlockRegex := regexp.MustCompile("```(?:json)?\\s*")
	s = jsonBlockRegex.ReplaceAllString(s, "")
	s = strings.ReplaceAll(s, "```", "")

	s = strings.TrimSpace(s)

	return s
}

// truncates a string to maxLen characters
func truncateString(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "..."
}

// Close closes the Gemini client
func (t *GeminiTranscriber) Close() error {
	// The genai client doesn't have a Close method in the current SDK
	// but we include this for future compatibility
	return nil
}
