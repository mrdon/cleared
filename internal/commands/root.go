package commands

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/cleared-dev/cleared/internal/buildinfo"
)

// NewRootCommand creates the root CLI command with all subcommands registered.
func NewRootCommand() *cobra.Command {
	rootCmd := &cobra.Command{
		Use:     "cleared",
		Short:   "Agentic small business accounting",
		Version: fmt.Sprintf("%s (commit: %s, built: %s)", buildinfo.Version, buildinfo.Commit, buildinfo.Date),
		CompletionOptions: cobra.CompletionOptions{
			DisableDefaultCmd: true,
		},
		SilenceUsage: true,
	}

	rootCmd.AddCommand(newInitCommand())

	return rootCmd
}
