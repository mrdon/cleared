package journal

import (
	"fmt"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cleared-dev/cleared/internal/model"
)

// mockAccounts implements AccountChecker for testing.
type mockAccounts struct {
	ids map[int]bool
}

func (m *mockAccounts) Exists(id int) bool {
	return m.ids[id]
}

func newMockAccounts(ids ...int) *mockAccounts {
	m := &mockAccounts{ids: make(map[int]bool)}
	for _, id := range ids {
		m.ids[id] = true
	}
	return m
}

func balancedEntry(seq int, debitAcct, creditAcct int, amount string) []model.Leg {
	d, _ := decimal.NewFromString(amount)
	entryID := "2025-01-" + padSeq(seq)
	return []model.Leg{
		{
			EntryID:   entryID + "a",
			Date:      time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			AccountID: debitAcct,
			Debit:     d,
			Status:    model.StatusAutoConfirmed,
		},
		{
			EntryID:   entryID + "b",
			Date:      time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC),
			AccountID: creditAcct,
			Credit:    d,
			Status:    model.StatusAutoConfirmed,
		},
	}
}

func padSeq(seq int) string {
	return fmt.Sprintf("%03d", seq)
}

var defaultAccounts = newMockAccounts(1010, 1020, 2010, 3010, 4010, 5020)

func TestValidate_Balanced(t *testing.T) {
	legs := balancedEntry(1, 5020, 1010, "100.00")
	errs := ValidateLegs(legs, defaultAccounts, 2025, 1)
	assert.Empty(t, errs)
}

func TestValidate_Invariant1_Unbalanced(t *testing.T) {
	legs := []model.Leg{
		{
			EntryID:   "2025-01-001a",
			Date:      date(2025, 1, 15),
			AccountID: 5020,
			Debit:     dec("100.00"),
			Status:    model.StatusAutoConfirmed,
		},
		{
			EntryID:   "2025-01-001b",
			Date:      date(2025, 1, 15),
			AccountID: 1010,
			Credit:    dec("99.00"),
			Status:    model.StatusAutoConfirmed,
		},
	}
	errs := ValidateLegs(legs, defaultAccounts, 2025, 1)
	require.NotEmpty(t, errs)
	assert.Equal(t, 1, errs[0].Invariant)
}

func TestValidate_Invariant2_BothDebitAndCredit(t *testing.T) {
	legs := []model.Leg{
		{
			EntryID:   "2025-01-001a",
			Date:      date(2025, 1, 15),
			AccountID: 5020,
			Debit:     dec("100.00"),
			Credit:    dec("100.00"),
			Status:    model.StatusAutoConfirmed,
		},
	}
	errs := ValidateLegs(legs, defaultAccounts, 2025, 1)
	has2 := false
	for _, e := range errs {
		if e.Invariant == 2 {
			has2 = true
		}
	}
	assert.True(t, has2, "should have invariant 2 violation")
}

func TestValidate_Invariant2_NeitherDebitNorCredit(t *testing.T) {
	legs := []model.Leg{
		{
			EntryID:   "2025-01-001a",
			Date:      date(2025, 1, 15),
			AccountID: 5020,
			Status:    model.StatusAutoConfirmed,
		},
	}
	errs := ValidateLegs(legs, defaultAccounts, 2025, 1)
	has2 := false
	for _, e := range errs {
		if e.Invariant == 2 {
			has2 = true
		}
	}
	assert.True(t, has2, "should have invariant 2 violation")
}

func TestValidate_Invariant3_UnknownAccount(t *testing.T) {
	legs := balancedEntry(1, 9999, 1010, "50.00")
	errs := ValidateLegs(legs, defaultAccounts, 2025, 1)
	has3 := false
	for _, e := range errs {
		if e.Invariant == 3 {
			has3 = true
		}
	}
	assert.True(t, has3, "should have invariant 3 violation")
}

func TestValidate_Invariant4_WrongMonth(t *testing.T) {
	legs := []model.Leg{
		{
			EntryID:   "2025-01-001a",
			Date:      date(2025, 2, 15), // February, not January
			AccountID: 5020,
			Debit:     dec("50.00"),
			Status:    model.StatusAutoConfirmed,
		},
		{
			EntryID:   "2025-01-001b",
			Date:      date(2025, 2, 15),
			AccountID: 1010,
			Credit:    dec("50.00"),
			Status:    model.StatusAutoConfirmed,
		},
	}
	errs := ValidateLegs(legs, defaultAccounts, 2025, 1)
	has4 := false
	for _, e := range errs {
		if e.Invariant == 4 {
			has4 = true
		}
	}
	assert.True(t, has4, "should have invariant 4 violation")
}

func TestValidate_Invariant5_NonContiguousSeq(t *testing.T) {
	// Entry 1 and 3, but missing 2.
	legs := append(balancedEntry(1, 5020, 1010, "50.00"), balancedEntry(3, 5020, 1010, "75.00")...)
	errs := ValidateLegs(legs, defaultAccounts, 2025, 1)
	has5 := false
	for _, e := range errs {
		if e.Invariant == 5 {
			has5 = true
		}
	}
	assert.True(t, has5, "should have invariant 5 violation for missing seq 2")
}

func TestValidate_Invariant6_TooManyDecimals(t *testing.T) {
	legs := []model.Leg{
		{
			EntryID:   "2025-01-001a",
			Date:      date(2025, 1, 15),
			AccountID: 5020,
			Debit:     dec("10.123"),
			Status:    model.StatusAutoConfirmed,
		},
		{
			EntryID:   "2025-01-001b",
			Date:      date(2025, 1, 15),
			AccountID: 1010,
			Credit:    dec("10.123"),
			Status:    model.StatusAutoConfirmed,
		},
	}
	errs := ValidateLegs(legs, defaultAccounts, 2025, 1)
	has6 := false
	for _, e := range errs {
		if e.Invariant == 6 {
			has6 = true
		}
	}
	assert.True(t, has6, "should have invariant 6 violation")
}

func TestValidate_MultiError(t *testing.T) {
	// Unbalanced + unknown account + wrong date — multiple errors.
	legs := []model.Leg{
		{
			EntryID:   "2025-01-001a",
			Date:      date(2025, 2, 1), // wrong month
			AccountID: 9999,             // unknown account
			Debit:     dec("100.00"),
			Status:    model.StatusAutoConfirmed,
		},
		{
			EntryID:   "2025-01-001b",
			Date:      date(2025, 1, 1),
			AccountID: 1010,
			Credit:    dec("50.00"), // unbalanced
			Status:    model.StatusAutoConfirmed,
		},
	}
	errs := ValidateLegs(legs, defaultAccounts, 2025, 1)
	assert.Greater(t, len(errs), 1, "should have multiple errors")
}

func TestValidate_EmptyLegs(t *testing.T) {
	errs := ValidateLegs(nil, defaultAccounts, 2025, 1)
	assert.Empty(t, errs)
}

func TestValidate_MultiLegBalanced(t *testing.T) {
	// 3-leg entry: split expense across two accounts.
	legs := []model.Leg{
		{
			EntryID:   "2025-01-001a",
			Date:      date(2025, 1, 15),
			AccountID: 5020,
			Debit:     dec("60.00"),
			Status:    model.StatusAutoConfirmed,
		},
		{
			EntryID:   "2025-01-001b",
			Date:      date(2025, 1, 15),
			AccountID: 5020,
			Debit:     dec("40.00"),
			Status:    model.StatusAutoConfirmed,
		},
		{
			EntryID:   "2025-01-001c",
			Date:      date(2025, 1, 15),
			AccountID: 1010,
			Credit:    dec("100.00"),
			Status:    model.StatusAutoConfirmed,
		},
	}
	errs := ValidateLegs(legs, defaultAccounts, 2025, 1)
	assert.Empty(t, errs)
}
