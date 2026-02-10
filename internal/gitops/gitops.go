package gitops

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// Init initializes a new git repository at dir.
func Init(dir string) error {
	cmd := exec.Command("git", "init")
	cmd.Dir = dir
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return fmt.Errorf("git init: %w", err)
	}
	return nil
}

// CommitAll stages all files and creates a commit. Returns the short commit hash.
func CommitAll(dir, message, authorName, authorEmail string) (string, error) {
	author := fmt.Sprintf("%s <%s>", authorName, authorEmail)

	// Stage all files.
	add := exec.Command("git", "add", "-A")
	add.Dir = dir
	if out, err := add.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git add: %s: %w", out, err)
	}

	// Commit.
	commit := exec.Command("git", "commit", "-m", message, "--author", author)
	commit.Dir = dir
	if out, err := commit.CombinedOutput(); err != nil {
		return "", fmt.Errorf("git commit: %s: %w", out, err)
	}

	// Get short hash.
	rev := exec.Command("git", "rev-parse", "--short", "HEAD")
	rev.Dir = dir
	out, err := rev.Output()
	if err != nil {
		return "", fmt.Errorf("git rev-parse: %w", err)
	}
	return strings.TrimSpace(string(out)), nil
}

// IsRepo reports whether dir is inside a git repository.
func IsRepo(dir string) bool {
	_, err := os.Stat(filepath.Join(dir, ".git"))
	return err == nil
}
