package gitops

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestInit(t *testing.T) {
	dir := t.TempDir()
	err := Init(dir)
	require.NoError(t, err)

	_, err = os.Stat(filepath.Join(dir, ".git"))
	require.NoError(t, err, ".git directory should exist")
}

func TestIsRepo(t *testing.T) {
	dir := t.TempDir()
	assert.False(t, IsRepo(dir), "empty dir should not be a repo")

	require.NoError(t, Init(dir))
	assert.True(t, IsRepo(dir), "initialized dir should be a repo")
}

func TestCommitAll(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, Init(dir))

	// Create a file to commit.
	require.NoError(t, os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0o644))

	hash, err := CommitAll(dir, "init: test commit", "Test Author", "test@example.com")
	require.NoError(t, err)
	assert.NotEmpty(t, hash)

	// Verify commit message.
	log := exec.Command("git", "log", "--format=%s", "-1")
	log.Dir = dir
	out, err := log.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "init: test commit")

	// Verify author.
	authorLog := exec.Command("git", "log", "--format=%an <%ae>", "-1")
	authorLog.Dir = dir
	out, err = authorLog.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "Test Author <test@example.com>")
}
