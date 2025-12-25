package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

var generateCmd = &cobra.Command{
	Use:   "generate [video_file]",
	Short: "Generate subtitles for a video file",
	Long: `Generate subtitles for the specified video file using AI transcription.

The generated subtitles will be saved as a separate file or embedded
into the video depending on the flags provided.`,
	Args: cobra.ExactArgs(1),
	RunE: runGenerate,
}

func init() {
	rootCmd.AddCommand(generateCmd)

	generateCmd.Flags().
		Bool("embed", false, "Embed subtitles directly into the video")
	generateCmd.Flags().String("model", "", "AI model to use for transcription")
}

func runGenerate(cmd *cobra.Command, args []string) error {
	videoPath := args[0]
	embed, _ := cmd.Flags().GetBool("embed")

	logger.Infow("Starting subtitle generation",
		"video", videoPath,
		"embed", embed,
	)

	// TODO: Implement subtitle generation pipeline
	// 1 Extract audio from video
	// 2 Transcribe audio using AI
	// 3 Generate subtitle file

	fmt.Printf("Generating subtitles for: %s\n", videoPath)

	return nil
}
