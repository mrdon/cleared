package commands_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	accountsCSV "github.com/cleared-dev/cleared/internal/accounts"
)

var binaryPath string

func TestMain(m *testing.M) {
	// Build the binary once for all tests.
	tmpDir, err := os.MkdirTemp("", "cleared-test-*")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(tmpDir)

	binaryPath = filepath.Join(tmpDir, "cleared")
	cmd := exec.Command("go", "build", "-o", binaryPath, "../../cmd/cleared")
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		panic("failed to build binary: " + err.Error())
	}

	os.Exit(m.Run())
}

func runCleared(t *testing.T, args ...string) (string, error) {
	t.Helper()
	cmd := exec.Command(binaryPath, args...)
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func TestInit_CreatesStructure(t *testing.T) {
	dir := t.TempDir()
	_, err := runCleared(t, "init", dir, "--name", "Test Biz")
	require.NoError(t, err)

	expectedDirs := []string{
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
	for _, d := range expectedDirs {
		info, err := os.Stat(filepath.Join(dir, d))
		require.NoError(t, err, "directory %s should exist", d)
		assert.True(t, info.IsDir(), "%s should be a directory", d)
	}
}

func TestInit_Config(t *testing.T) {
	dir := t.TempDir()
	_, err := runCleared(t, "init", dir, "--name", "My Company")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, "cleared.yaml"))
	require.NoError(t, err)
	contents := string(data)

	assert.Contains(t, contents, "name: My Company")
	assert.Contains(t, contents, "entity_type: llc_single_member")
}

func TestInit_Accounts(t *testing.T) {
	dir := t.TempDir()
	_, err := runCleared(t, "init", dir, "--name", "Test Biz")
	require.NoError(t, err)

	path := filepath.Join(dir, "accounts", "chart-of-accounts.csv")
	f, err := os.Open(path)
	require.NoError(t, err)
	defer f.Close()

	accts, err := accountsCSV.ReadAccounts(f)
	require.NoError(t, err)
	assert.Len(t, accts, 11, "default LLC single member chart has 11 accounts")
}

func TestInit_GitRepo(t *testing.T) {
	dir := t.TempDir()
	_, err := runCleared(t, "init", dir, "--name", "Test Biz")
	require.NoError(t, err)

	// .git directory should exist.
	_, err = os.Stat(filepath.Join(dir, ".git"))
	require.NoError(t, err, ".git should exist")

	// git log should have an init commit.
	log := exec.Command("git", "log", "--format=%s", "-1")
	log.Dir = dir
	out, err := log.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "init:")

	// Verify author.
	authorLog := exec.Command("git", "log", "--format=%an <%ae>", "-1")
	authorLog.Dir = dir
	out, err = authorLog.Output()
	require.NoError(t, err)
	assert.Contains(t, string(out), "Cleared Agent <agent@cleared.dev>")
}

func TestInit_Gitignore(t *testing.T) {
	dir := t.TempDir()
	_, err := runCleared(t, "init", dir, "--name", "Test Biz")
	require.NoError(t, err)

	data, err := os.ReadFile(filepath.Join(dir, ".gitignore"))
	require.NoError(t, err)
	contents := string(data)

	for _, pattern := range []string{"receipts/", "exports/", "queue/", ".cleared-cache/"} {
		assert.Contains(t, contents, pattern, ".gitignore should contain %s", pattern)
	}
}

func TestInit_RequiresName(t *testing.T) {
	dir := t.TempDir()
	_, err := runCleared(t, "init", dir)
	require.Error(t, err, "init without --name should fail")
}

func TestInit_DefaultEntityType(t *testing.T) {
	dir := t.TempDir()
	_, err := runCleared(t, "init", dir, "--name", "Test Biz")
	require.NoError(t, err)

	// Should use llc_single_member by default.
	data, err := os.ReadFile(filepath.Join(dir, "cleared.yaml"))
	require.NoError(t, err)
	assert.Contains(t, string(data), "llc_single_member")

	// Accounts should match llc_single_member chart.
	f, err := os.Open(filepath.Join(dir, "accounts", "chart-of-accounts.csv"))
	require.NoError(t, err)
	defer f.Close()

	accts, err := accountsCSV.ReadAccounts(f)
	require.NoError(t, err)
	assert.Len(t, accts, 11)
}
