package id

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestFormatEntryID(t *testing.T) {
	tests := []struct {
		year, month, seq int
		want             string
	}{
		{2025, 1, 1, "2025-01-001"},
		{2025, 12, 99, "2025-12-099"},
		{2025, 1, 123, "2025-01-123"},
	}
	for _, tt := range tests {
		got := FormatEntryID(tt.year, tt.month, tt.seq)
		assert.Equal(t, tt.want, got)
	}
}

func TestFormatLegID(t *testing.T) {
	tests := []struct {
		entryID string
		leg     int
		want    string
	}{
		{"2025-01-001", 0, "2025-01-001a"},
		{"2025-01-001", 1, "2025-01-001b"},
		{"2025-01-001", 2, "2025-01-001c"},
	}
	for _, tt := range tests {
		got := FormatLegID(tt.entryID, tt.leg)
		assert.Equal(t, tt.want, got)
	}
}

func TestParseEntryID(t *testing.T) {
	tests := []struct {
		input               string
		wantYear, wantMonth int
		wantSeq             int
	}{
		{"2025-01-001", 2025, 1, 1},
		{"2025-12-099", 2025, 12, 99},
		{"2025-01-001a", 2025, 1, 1},
		{"2025-01-001b", 2025, 1, 1},
	}
	for _, tt := range tests {
		year, month, seq, err := ParseEntryID(tt.input)
		require.NoError(t, err, "input: %s", tt.input)
		assert.Equal(t, tt.wantYear, year)
		assert.Equal(t, tt.wantMonth, month)
		assert.Equal(t, tt.wantSeq, seq)
	}
}

func TestParseEntryID_Errors(t *testing.T) {
	badInputs := []string{
		"",
		"not-valid",
		"2025-01",
		"xxxx-01-001",
	}
	for _, input := range badInputs {
		_, _, _, err := ParseEntryID(input)
		assert.Error(t, err, "expected error for input: %s", input)
	}
}

func TestEntryGroup(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"2025-01-001a", "2025-01-001"},
		{"2025-01-001b", "2025-01-001"},
		{"2025-01-001", "2025-01-001"},
		{"", ""},
	}
	for _, tt := range tests {
		got := EntryGroup(tt.input)
		assert.Equal(t, tt.want, got)
	}
}
