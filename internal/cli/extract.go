package cli

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"github.com/mgpai22/lipi/internal/video"
	"github.com/spf13/cobra"
)

var extractCmd = &cobra.Command{
	Use:   "extract [video_file]",
	Short: "Extract audio from a video file",
	Long: `Extract the audio track from a video file and save it as a separate audio file.

Supports multiple output formats: wav, mp3, aac, flac.

Examples:
  lipi extract video.mp4
  lipi extract video.mp4 -o audio.mp3 -f mp3
  lipi extract video.mp4 --format wav --sample-rate 44100 --channels 2`,
	Args: cobra.ExactArgs(1),
	RunE: runExtract,
}

func init() {
	rootCmd.AddCommand(extractCmd)

	extractCmd.Flags().
		StringP("format", "f", "wav", "Output audio format (wav, mp3, aac, flac)")
	extractCmd.Flags().
		IntP("sample-rate", "r", 16000, "Sample rate in Hz (e.g., 16000, 44100, 48000)")
	extractCmd.Flags().
		IntP("channels", "c", 1, "Number of audio channels (1=mono, 2=stereo)")
	extractCmd.Flags().
		StringP("bitrate", "b", "", "Bitrate for lossy formats (e.g., 128k, 320k)")
}

func runExtract(cmd *cobra.Command, args []string) error {
	videoPath := args[0]

	format, _ := cmd.Flags().GetString("format")
	sampleRate, _ := cmd.Flags().GetInt("sample-rate")
	channels, _ := cmd.Flags().GetInt("channels")
	bitrate, _ := cmd.Flags().GetString("bitrate")
	outputPath, _ := cmd.Flags().GetString("output")

	if outputPath == "" {
		ext := videoPath[strings.LastIndex(videoPath, ".")+1:]
		outputPath = strings.TrimSuffix(videoPath, "."+ext) + "." + format
	}

	validFormats := map[string]bool{
		"wav":  true,
		"mp3":  true,
		"aac":  true,
		"flac": true,
	}
	if !validFormats[format] {
		return fmt.Errorf(
			"invalid format %q: supported formats are wav, mp3, aac, flac",
			format,
		)
	}

	logger.Infow("Extracting audio",
		"video", videoPath,
		"output", outputPath,
		"format", format,
		"sample_rate", sampleRate,
		"channels", channels,
	)

	processor := video.NewProcessor("")

	opts := video.ExtractAudioOptions{
		Format:     format,
		SampleRate: sampleRate,
		Channels:   channels,
		Bitrate:    bitrate,
	}

	ctx := context.Background()
	if err := processor.ExtractAudio(
		ctx,
		videoPath,
		outputPath,
		opts,
	); err != nil {
		return fmt.Errorf("extraction failed: %w", err)
	}

	absOutput, _ := filepath.Abs(outputPath)
	fmt.Printf("Audio extracted successfully: %s\n", absOutput)

	return nil
}
