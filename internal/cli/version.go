package cli

import (
	"fmt"

	"github.com/spf13/cobra"
)

// version information via ldflags during build
var (
	Version   = "dev"
	Commit    = "none"
	BuildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Print version information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Printf("lipi %s\n", Version)
		fmt.Printf("  commit: %s\n", Commit)
		fmt.Printf("  built:  %s\n", BuildDate)
	},
}

func init() {
	rootCmd.AddCommand(versionCmd)
}
