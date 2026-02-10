package agentlog

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var testTime = time.Date(2025, 1, 15, 10, 30, 0, 0, time.UTC)

func testEntry() Entry {
	return Entry{
		Timestamp:  testTime,
		Agent:      "categorize",
		Action:     "categorize_transaction",
		Details:    "Categorized GITHUB as software_expense",
		EntryID:    "TXN-20250115-001",
		CommitHash: "abc1234",
	}
}

func TestAppend_NewFile(t *testing.T) {
	dir := t.TempDir()
	err := Append(dir, []Entry{testEntry()})
	require.NoError(t, err)

	entries, err := Read(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 1)
	assert.Equal(t, "categorize", entries[0].Agent)
}

func TestAppend_ExistingFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, Append(dir, []Entry{testEntry()}))

	e2 := testEntry()
	e2.Agent = "import"
	e2.Action = "import_bank_csv"
	require.NoError(t, Append(dir, []Entry{e2}))

	entries, err := Read(dir)
	require.NoError(t, err)
	assert.Len(t, entries, 2)
	assert.Equal(t, "categorize", entries[0].Agent)
	assert.Equal(t, "import", entries[1].Agent)
}

func TestRead_RoundTrip(t *testing.T) {
	dir := t.TempDir()
	original := testEntry()
	require.NoError(t, Append(dir, []Entry{original}))

	entries, err := Read(dir)
	require.NoError(t, err)
	require.Len(t, entries, 1)

	got := entries[0]
	assert.True(t, original.Timestamp.Equal(got.Timestamp))
	assert.Equal(t, original.Agent, got.Agent)
	assert.Equal(t, original.Action, got.Action)
	assert.Equal(t, original.Details, got.Details)
	assert.Equal(t, original.EntryID, got.EntryID)
	assert.Equal(t, original.CommitHash, got.CommitHash)
}

func TestRead_NotFound(t *testing.T) {
	dir := t.TempDir()
	entries, err := Read(dir)
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestRead_EmptyFile(t *testing.T) {
	dir := t.TempDir()
	require.NoError(t, os.MkdirAll(filepath.Join(dir, "logs"), 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(dir, "logs", "agent-log.csv"), []byte(Header+"\n"), 0o644))

	entries, err := Read(dir)
	require.NoError(t, err)
	assert.Nil(t, entries)
}

func TestMarshalUnmarshal(t *testing.T) {
	e := testEntry()
	row := MarshalEntry(e)
	assert.Len(t, row, 6)

	got, err := UnmarshalEntry(row)
	require.NoError(t, err)
	assert.True(t, e.Timestamp.Equal(got.Timestamp))
	assert.Equal(t, e.Agent, got.Agent)
	assert.Equal(t, e.Action, got.Action)
	assert.Equal(t, e.Details, got.Details)
	assert.Equal(t, e.EntryID, got.EntryID)
	assert.Equal(t, e.CommitHash, got.CommitHash)
}

func TestUnmarshalEntry_BadFieldCount(t *testing.T) {
	_, err := UnmarshalEntry([]string{"one", "two"})
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "expected 6 fields")
}

func TestTimestampFormat(t *testing.T) {
	e := testEntry()
	row := MarshalEntry(e)
	assert.Equal(t, "2025-01-15T10:30:00Z", row[0])
}

func TestAppend_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	// logs/ dir does not exist yet
	err := Append(dir, []Entry{testEntry()})
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "logs"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
