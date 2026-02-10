package model

import (
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestLegEntryGroup(t *testing.T) {
	tests := []struct {
		entryID string
		want    string
	}{
		{"2025-01-001a", "2025-01-001"},
		{"2025-01-001b", "2025-01-001"},
		{"2025-01-001", "2025-01-001"},
		{"2025-12-099abc", "2025-12-099"},
		{"", ""},
	}
	for _, tt := range tests {
		leg := Leg{EntryID: tt.entryID}
		assert.Equal(t, tt.want, leg.EntryGroup(), "EntryGroup(%q)", tt.entryID)
	}
}
