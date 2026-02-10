package id

import (
	"fmt"
	"strconv"
	"strings"
)

// FormatEntryID returns an entry ID like "2025-01-001".
func FormatEntryID(year, month, seq int) string {
	return fmt.Sprintf("%04d-%02d-%03d", year, month, seq)
}

// FormatLegID returns a leg ID like "2025-01-001a" (leg 0='a', 1='b', etc.).
func FormatLegID(entryID string, leg int) string {
	return entryID + string(rune('a'+leg))
}

// ParseEntryID parses "2025-01-001" into year, month, seq.
func ParseEntryID(id string) (year, month, seq int, err error) {
	// Strip any leg suffix (trailing lowercase letters).
	base := EntryGroup(id)

	parts := strings.SplitN(base, "-", 3)
	if len(parts) != 3 {
		return 0, 0, 0, fmt.Errorf("invalid entry ID format: %q", id)
	}

	year, err = strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid year in entry ID %q: %w", id, err)
	}

	month, err = strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid month in entry ID %q: %w", id, err)
	}

	seq, err = strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, fmt.Errorf("invalid sequence in entry ID %q: %w", id, err)
	}

	return year, month, seq, nil
}

// EntryGroup strips the leg suffix from a leg ID.
// "2025-01-001a" -> "2025-01-001"
func EntryGroup(legID string) string {
	if len(legID) == 0 {
		return ""
	}
	i := len(legID)
	for i > 0 && legID[i-1] >= 'a' && legID[i-1] <= 'z' {
		i--
	}
	return legID[:i]
}
