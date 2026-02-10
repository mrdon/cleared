package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cleared-dev/cleared/internal/accounts"
	"github.com/cleared-dev/cleared/internal/config"
	"github.com/cleared-dev/cleared/internal/gitops"
)

func newInitCommand() *cobra.Command {
	var name string
	var entityType string

	cmd := &cobra.Command{
		Use:   "init [directory]",
		Short: "Initialize a new Cleared project",
		Args:  cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			dir := "."
			if len(args) > 0 {
				dir = args[0]
			}

			absDir, err := filepath.Abs(dir)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}

			return runInit(absDir, name, entityType)
		},
	}

	cmd.Flags().StringVar(&name, "name", "", "business name (required)")
	_ = cmd.MarkFlagRequired("name")
	cmd.Flags().StringVar(&entityType, "entity-type", "llc_single_member", "entity type")

	return cmd
}

func runInit(dir, name, entityType string) error {
	// Create directory structure.
	dirs := []string{
		"accounts",
		"rules",
		"agents",
		"scripts",
		"templates",
		"tests",
		"logs",
		"import",
		filepath.Join("import", "processed"),
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			return fmt.Errorf("creating directory %s: %w", d, err)
		}
	}

	// Write cleared.yaml.
	cfg := config.Default(name, entityType)
	if err := config.Save(filepath.Join(dir, "cleared.yaml"), cfg); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}

	// Write chart of accounts.
	chart := accounts.DefaultChart(entityType)
	svc := accounts.NewService(chart)
	if err := svc.Save(dir); err != nil {
		return fmt.Errorf("writing chart of accounts: %w", err)
	}

	// Write empty categorization rules.
	rulesContent := "rules: []\n"
	if err := os.WriteFile(filepath.Join(dir, "rules", "categorization-rules.yaml"), []byte(rulesContent), 0o644); err != nil {
		return fmt.Errorf("writing rules: %w", err)
	}

	// Write .gitignore.
	gitignore := "receipts/\nexports/\nqueue/\n.cleared-cache/\n"
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return fmt.Errorf("writing .gitignore: %w", err)
	}

	// Write import/.gitkeep.
	if err := os.WriteFile(filepath.Join(dir, "import", ".gitkeep"), []byte{}, 0o644); err != nil {
		return fmt.Errorf("writing .gitkeep: %w", err)
	}

	// Initialize git and create initial commit.
	if err := gitops.Init(dir); err != nil {
		return fmt.Errorf("git init: %w", err)
	}

	hash, err := gitops.CommitAll(dir, "init: Initialize "+name, cfg.Git.AuthorName, cfg.Git.AuthorEmail)
	if err != nil {
		return fmt.Errorf("initial commit: %w", err)
	}

	fmt.Printf("Initialized Cleared project at %s (%s)\n", dir, hash)
	return nil
}
