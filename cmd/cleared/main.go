package main

import (
	"os"

	"github.com/cleared-dev/cleared/internal/commands"
)

func main() {
	if err := commands.NewRootCommand().Execute(); err != nil {
		os.Exit(1)
	}
}
