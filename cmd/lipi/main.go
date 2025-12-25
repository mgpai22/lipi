package main

import (
	"os"

	"github.com/mgpai22/lipi/internal/cli"
)

func main() {
	if err := cli.Execute(); err != nil {
		os.Exit(1)
	}
}
