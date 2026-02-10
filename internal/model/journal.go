package model

import (
	"strings"
	"time"

	"github.com/shopspring/decimal"
)

// EntryStatus represents the lifecycle state of a journal entry.
type EntryStatus string

const (
	StatusAutoConfirmed      EntryStatus = "auto-confirmed"
	StatusPendingReview      EntryStatus = "pending-review"
	StatusUserConfirmed      EntryStatus = "user-confirmed"
	StatusUserCorrected      EntryStatus = "user-corrected"
	StatusVoided             EntryStatus = "voided"
	StatusBootstrapConfirmed EntryStatus = "bootstrap-confirmed"
)

// Leg is a single row in journal.csv (one side of a double-entry).
type Leg struct {
	EntryID      string          // "YYYY-MM-NNNx" where x = a,b,c...
	Date         time.Time       //nolint:revive // plain field name is clearest
	AccountID    int             //nolint:revive
	Description  string          //nolint:revive
	Debit        decimal.Decimal // zero if credit side
	Credit       decimal.Decimal // zero if debit side
	Counterparty string
	Reference    string
	Confidence   decimal.Decimal
	Status       EntryStatus
	Evidence     string
	ReceiptHash  string
	Tags         string // semicolon-separated
	Notes        string
}

// EntryGroup returns the base entry ID (without leg suffix).
// "2025-01-001a" -> "2025-01-001"
func (l Leg) EntryGroup() string {
	id := l.EntryID
	if len(id) == 0 {
		return ""
	}
	// Trim trailing letter(s) that form the leg suffix.
	i := len(id)
	for i > 0 && id[i-1] >= 'a' && id[i-1] <= 'z' {
		i--
	}
	return strings.TrimRight(id[:i], "")
}
