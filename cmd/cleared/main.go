package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/cleared-dev/cleared/internal/buildinfo"
)

func main() {
	rootCmd := &cobra.Command{
		Use:     "cleared",
		Short:   "Agentic small business accounting",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", buildinfo.Version, buildinfo.Commit, buildinfo.Date),
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		SilenceUsage: true,
	}

	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
