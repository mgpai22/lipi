package cli

import (
	"context"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/mgpai22/lipi/internal/audio"
	"github.com/mgpai22/lipi/internal/subtitle"
	"github.com/mgpai22/lipi/internal/transcribe"
	"github.com/mgpai22/lipi/internal/video"
	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate [media_file]",
	Short: "Generate subtitles for an audio or video file",
	Long: `Generate subtitles for the specified audio or video file using AI transcription.

The command accepts both audio files (mp3, wav, aac, etc.) and video files (mp4, mkv, etc.).
For video files, audio is automatically extracted before transcription.

The audio is split into chunks (default 1 minute) and transcribed in parallel using Google Gemini.
Generated subtitles can be output in SRT, VTT, or ASS format.

Examples:
  lipi generate video.mp4
  lipi generate audio.mp3 --format vtt
  lipi generate video.mp4 --api-key YOUR_KEY --chunk-duration 2
  lipi generate podcast.mp3 -f srt -d 1 --concurrency 5`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().
		Bool("embed", false, "Embed subtitles directly into the video (not yet implemented)")
	generateCmd.Flags().
		StringP("api-key", "k", "", "Gemini API key (or set GEMINI_API_KEY env var)")
	generateCmd.Flags().
		IntP("chunk-duration", "d", 1, "Chunk duration in minutes for splitting audio")
	generateCmd.Flags().
		StringP("format", "f", "srt", "Output subtitle format (srt, vtt, ass)")
	generateCmd.Flags().
		Int("concurrency", 3, "Number of parallel transcription workers")
	generateCmd.Flags().
		String("model", "gemini-2.5-flash", "Gemini model to use for transcription")
	generateCmd.Flags().
		String("transcript-language", "native", "Output language for transcript (e.g., 'english', 'spanish', or 'native' for original language)")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	mediaPath := args[0]
	ctx := context.Background()

	if _, err := os.Stat(mediaPath); os.IsNotExist(err) {
		return fmt.Errorf("file not found: %s", mediaPath)
	}
	if !audio.IsMediaFile(mediaPath) {
		return fmt.Errorf("unsupported file type: %s (expected audio or video file)", filepath.Ext(mediaPath))
	}

	apiKey, _ := cmd.Flags().GetString("api-key")
	chunkDuration, _ := cmd.Flags().GetInt("chunk-duration")
	formatStr, _ := cmd.Flags().GetString("format")
	concurrency, _ := cmd.Flags().GetInt("concurrency")
	model, _ := cmd.Flags().GetString("model")
	outputPath, _ := cmd.Flags().GetString("output")
	language, _ := cmd.Flags().GetString("language")
	transcriptLang, _ := cmd.Flags().GetString("transcript-language")

	if apiKey == "" {
		apiKey = os.Getenv("GEMINI_API_KEY")
	}
	if apiKey == "" {
		return fmt.Errorf("Gemini API key is required: use --api-key flag or set GEMINI_API_KEY environment variable")
	}

	var format subtitle.Format
	switch strings.ToLower(formatStr) {
	case "srt":
		format = subtitle.FormatSRT
	case "vtt":
		format = subtitle.FormatVTT
	case "ass":
		format = subtitle.FormatASS
	default:
		return fmt.Errorf("unsupported format %q: use srt, vtt, or ass", formatStr)
	}

	if outputPath == "" {
		baseName := strings.TrimSuffix(mediaPath, filepath.Ext(mediaPath))
		outputPath = baseName + subtitle.GetExtensionForFormat(format)
	}

	logger.Infow("Starting subtitle generation",
		"input", mediaPath,
		"output", outputPath,
		"format", formatStr,
		"chunk_duration", chunkDuration,
		"concurrency", concurrency,
	)

	tempDir, err := os.MkdirTemp("", "lipi-*")
	if err != nil {
		return fmt.Errorf("failed to create temp directory: %w", err)
	}
	defer os.RemoveAll(tempDir)

	var audioPath string
	compressionOpts := audio.DefaultCompressionOptions()

	if audio.IsVideoFile(mediaPath) {
		logger.Infow("Extracting audio from video")
		audioPath = filepath.Join(tempDir, "audio.mp3")

		processor := video.NewProcessor(tempDir)
		extractOpts := video.ExtractAudioOptions{
			Format:     compressionOpts.Format,
			SampleRate: compressionOpts.SampleRate,
			Channels:   compressionOpts.Channels,
			Bitrate:    compressionOpts.Bitrate,
		}

		if err := processor.ExtractAudio(ctx, mediaPath, audioPath, extractOpts); err != nil {
			return fmt.Errorf("failed to extract audio: %w", err)
		}
	} else {
		logger.Infow("Compressing audio for transcription")
		audioPath = filepath.Join(tempDir, "audio.mp3")

		if err := audio.CompressAudio(ctx, mediaPath, audioPath, compressionOpts); err != nil {
			return fmt.Errorf("failed to compress audio: %w", err)
		}
	}

	duration, err := audio.GetDuration(audioPath)
	if err != nil {
		return fmt.Errorf("failed to get audio duration: %w", err)
	}

	logger.Infow("Audio prepared",
		"duration", duration.String(),
	)

	chunkDir := filepath.Join(tempDir, "chunks")
	chunkDur := time.Duration(chunkDuration) * time.Minute

	logger.Infow("Splitting audio into chunks",
		"chunk_duration", chunkDur.String(),
	)

	chunks, err := audio.ChunkAudio(ctx, audioPath, chunkDur, chunkDir)
	if err != nil {
		return fmt.Errorf("failed to split audio: %w", err)
	}

	logger.Infow("Created audio chunks",
		"count", len(chunks),
	)

	transcribeOpts := transcribe.Options{
		Language:           language,
		TranscriptLanguage: transcriptLang,
		Model:              model,
	}

	transcriber, err := transcribe.Factory(ctx, transcribe.ProviderGemini, apiKey, transcribeOpts)
	if err != nil {
		return fmt.Errorf("failed to create transcriber: %w", err)
	}

	geminiTranscriber, ok := transcriber.(*transcribe.GeminiTranscriber)
	if !ok {
		return fmt.Errorf("expected GeminiTranscriber")
	}

	logger.Infow("Transcribing audio",
		"concurrency", concurrency,
	)

	result, err := geminiTranscriber.TranscribeWithChunks(ctx, chunks, concurrency)
	if err != nil {
		return fmt.Errorf("transcription failed: %w", err)
	}

	logger.Infow("Transcription complete",
		"segments", len(result.Segments),
	)

	generator := subtitle.NewDefaultGenerator()
	subs, err := generator.Generate(result.Segments)
	if err != nil {
		return fmt.Errorf("failed to generate subtitles: %w", err)
	}

	subs.Language = language
	subs.Format = string(format)

	writer, err := subtitle.NewWriter(format)
	if err != nil {
		return fmt.Errorf("failed to create subtitle writer: %w", err)
	}

	if err := writer.Write(subs, outputPath); err != nil {
		return fmt.Errorf("failed to write subtitles: %w", err)
	}

	absOutput, _ := filepath.Abs(outputPath)
	fmt.Printf("Subtitles generated successfully: %s\n", absOutput)
	fmt.Printf("  Entries: %d\n", len(subs.Entries))
	fmt.Printf("  Duration: %s\n", duration.String())

	return nil
}
