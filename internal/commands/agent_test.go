package commands_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func requireUV(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("uv"); err != nil {
		t.Skip("uv not available, skipping agent test")
	}
}

func TestAgentRun_Ingest(t *testing.T) {
	requireUV(t)

	dir := t.TempDir()

	// Init a repo.
	_, err := runCleared(t, "init", dir, "--name", "Test Corp")
	require.NoError(t, err)

	// Copy test CSV to import/.
	csvSrc := filepath.Join("..", "..", "testdata", "chase_checking.csv")
	csvData, err := os.ReadFile(csvSrc)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "import", "chase_checking.csv"), csvData, 0o644)
	require.NoError(t, err)

	// Copy test agent to agents/.
	agentSrc := filepath.Join("..", "..", "testdata", "ingest.py")
	agentData, err := os.ReadFile(agentSrc)
	require.NoError(t, err)
	err = os.WriteFile(filepath.Join(dir, "agents", "ingest.py"), agentData, 0o644)
	require.NoError(t, err)

	// Run agent.
	out, err := runCleared(t, "agent", "run", "ingest", "--repo", dir)
	require.NoError(t, err, "agent run failed: %s", out)

	// Verify journal.csv exists.
	journalPath := filepath.Join(dir, "2025", "01", "journal.csv")
	_, err = os.Stat(journalPath)
	require.NoError(t, err, "journal.csv should exist")

	journalData, err := os.ReadFile(journalPath)
	require.NoError(t, err)
	lines := strings.Split(strings.TrimSpace(string(journalData)), "\n")
	// Header + 12 legs (6 transactions * 2 legs each).
	assert.Len(t, lines, 13, "expected header + 12 legs")

	// Verify CSV moved to processed.
	processedFiles, err := os.ReadDir(filepath.Join(dir, "import", "processed"))
	require.NoError(t, err)
	assert.Len(t, processedFiles, 1, "one file should be in processed/")

	// Verify no CSVs left in import/.
	importFiles, err := os.ReadDir(filepath.Join(dir, "import"))
	require.NoError(t, err)
	csvCount := 0
	for _, f := range importFiles {
		if strings.HasSuffix(f.Name(), ".csv") {
			csvCount++
		}
	}
	assert.Equal(t, 0, csvCount, "no CSVs should remain in import/")

	// Verify git commit with import: prefix.
	log := exec.Command("git", "log", "--format=%s", "-5")
	log.Dir = dir
	logOut, err := log.Output()
	require.NoError(t, err)
	assert.Contains(t, string(logOut), "import:")

	// Verify agent log was written.
	logPath := filepath.Join(dir, "logs", "agent-log.csv")
	_, err = os.Stat(logPath)
	require.NoError(t, err, "agent-log.csv should exist")
}

func TestAgentRun_MissingAgent(t *testing.T) {
	dir := t.TempDir()

	// Init a repo.
	_, err := runCleared(t, "init", dir, "--name", "Test Corp")
	require.NoError(t, err)

	// Run nonexistent agent.
	_, err = runCleared(t, "agent", "run", "nonexistent", "--repo", dir)
	require.Error(t, err, "should fail for missing agent")
}
