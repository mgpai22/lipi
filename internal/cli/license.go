package cli

import (
	_ "embed"
	"fmt"

	"github.com/spf13/cobra"
)

//go:embed license.txt
var licenseText string

var licenseCmd = &cobra.Command{
	Use:   "license",
	Short: "Print license information",
	Run: func(cmd *cobra.Command, args []string) {
		fmt.Print(licenseText)
	},
}

func init() {
	rootCmd.AddCommand(licenseCmd)
}
