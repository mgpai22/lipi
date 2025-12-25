package cli

import (
	"github.com/mgpai22/lipi/internal/logging"
	"github.com/spf13/cobra"
)

var (
	verbose bool
	logger  *logging.Logger
)

var rootCmd = &cobra.Command{
	Use:   "lipi",
	Short: "AI-powered subtitle generator for videos",
	Long: `Lipi is a CLI tool that uses AI to automatically generate
subtitles for video files.

It supports multiple transcription providers and subtitle formats.`,
	PersistentPreRun: func(cmd *cobra.Command, args []string) {
		logger = logging.NewLogger(verbose)
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	rootCmd.PersistentFlags().
		BoolVarP(&verbose, "verbose", "v", false, "Enable verbose output")
	rootCmd.PersistentFlags().StringP("output", "o", "", "Output file path")
	rootCmd.PersistentFlags().
		StringP("language", "l", "", "Language code (e.g., en, es, fr)")
}
