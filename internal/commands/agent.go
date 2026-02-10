package commands

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/cleared-dev/cleared/internal/agentlog"
	"github.com/cleared-dev/cleared/internal/sandbox"
)

func newAgentCommand() *cobra.Command {
	agentCmd := &cobra.Command{
		Use:   "agent",
		Short: "Agent operations",
	}
	agentCmd.AddCommand(newAgentRunCommand())
	return agentCmd
}

func newAgentRunCommand() *cobra.Command {
	var dryRun bool
	var repoDir string

	cmd := &cobra.Command{
		Use:   "run <name>",
		Short: "Run an agent script",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			absDir, err := filepath.Abs(repoDir)
			if err != nil {
				return fmt.Errorf("resolving path: %w", err)
			}
			return runAgent(absDir, args[0], dryRun)
		},
	}

	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "run without making changes")
	cmd.Flags().StringVar(&repoDir, "repo", ".", "repository directory")

	return cmd
}

func runAgent(repoRoot, name string, dryRun bool) error {
	// Read agent script.
	scriptPath := filepath.Join(repoRoot, "agents", name+".py")
	script, err := os.ReadFile(scriptPath)
	if err != nil {
		return fmt.Errorf("reading agent %s: %w", name, err)
	}

	// Start bridge.
	bridge, err := sandbox.NewBridge()
	if err != nil {
		return fmt.Errorf("starting bridge: %w", err)
	}
	defer bridge.Shutdown()

	// Create runtime and register primitives.
	rt, err := sandbox.NewRuntime(repoRoot, name, dryRun)
	if err != nil {
		return fmt.Errorf("creating runtime: %w", err)
	}
	rt.Register(bridge)

	// Run script.
	externals := bridge.PrimitiveNames()
	result, err := bridge.RunScript(string(script), externals)
	if err != nil {
		return fmt.Errorf("agent %s failed: %w", name, err)
	}

	// Print result.
	if result != nil {
		fmt.Printf("%v\n", result)
	}

	// Write agent log.
	entries := rt.AgentLog()
	if len(entries) > 0 {
		if err := agentlog.Append(repoRoot, entries); err != nil {
			fmt.Fprintf(os.Stderr, "warning: failed to write agent log: %v\n", err)
		}
	}

	return nil
}
