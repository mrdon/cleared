package agentlog

import (
	"encoding/csv"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Entry is one row in the agent log.
type Entry struct {
	Timestamp  time.Time
	Agent      string
	Action     string
	Details    string
	EntryID    string
	CommitHash string
}

// Header is the CSV header for agent-log.csv.
const Header = "timestamp,agent,action,details,entry_id,commit_hash"

const (
	numFields     = 6
	logDir        = "logs"
	logFile       = "logs/agent-log.csv"
	colTimestamp  = 0
	colAgent      = 1
	colAction     = 2
	colDetails    = 3
	colEntryID    = 4
	colCommitHash = 5
)

// MarshalEntry converts an Entry to a CSV row.
func MarshalEntry(e Entry) []string {
	row := make([]string, numFields)
	row[colTimestamp] = e.Timestamp.Format(time.RFC3339)
	row[colAgent] = e.Agent
	row[colAction] = e.Action
	row[colDetails] = e.Details
	row[colEntryID] = e.EntryID
	row[colCommitHash] = e.CommitHash
	return row
}

// UnmarshalEntry converts a CSV row to an Entry.
func UnmarshalEntry(record []string) (Entry, error) {
	if len(record) != numFields {
		return Entry{}, fmt.Errorf("expected %d fields, got %d", numFields, len(record))
	}

	ts, err := time.Parse(time.RFC3339, record[colTimestamp])
	if err != nil {
		return Entry{}, fmt.Errorf("parsing timestamp %q: %w", record[colTimestamp], err)
	}

	return Entry{
		Timestamp:  ts,
		Agent:      record[colAgent],
		Action:     record[colAction],
		Details:    record[colDetails],
		EntryID:    record[colEntryID],
		CommitHash: record[colCommitHash],
	}, nil
}

// Append writes entries to <repoRoot>/logs/agent-log.csv, creating the file and header if needed.
func Append(repoRoot string, entries []Entry) error {
	dir := filepath.Join(repoRoot, logDir)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating logs dir: %w", err)
	}

	path := filepath.Join(repoRoot, logFile)
	needsHeader := false
	if _, err := os.Stat(path); os.IsNotExist(err) {
		needsHeader = true
	}

	f, err := os.OpenFile(path, os.O_APPEND|os.O_CREATE|os.O_WRONLY, 0o644)
	if err != nil {
		return fmt.Errorf("opening agent log: %w", err)
	}
	defer f.Close()

	cw := csv.NewWriter(f)
	defer cw.Flush()

	if needsHeader {
		if err := cw.Write(strings.Split(Header, ",")); err != nil {
			return fmt.Errorf("writing header: %w", err)
		}
	}

	for i, e := range entries {
		if err := cw.Write(MarshalEntry(e)); err != nil {
			return fmt.Errorf("writing entry %d: %w", i, err)
		}
	}

	return cw.Error()
}

// Read returns all entries from <repoRoot>/logs/agent-log.csv.
// Returns an empty slice if the file does not exist.
func Read(repoRoot string) ([]Entry, error) {
	path := filepath.Join(repoRoot, logFile)
	f, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("opening agent log: %w", err)
	}
	defer f.Close()

	return readEntries(f)
}

func readEntries(r io.Reader) ([]Entry, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = numFields

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading agent log CSV: %w", err)
	}

	if len(records) <= 1 {
		return nil, nil
	}

	var entries []Entry
	for i, rec := range records[1:] {
		e, err := UnmarshalEntry(rec)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+2, err)
		}
		entries = append(entries, e)
	}
	return entries, nil
}
