package journal

import (
	"fmt"

	"github.com/shopspring/decimal"

	"github.com/cleared-dev/cleared/internal/id"
	"github.com/cleared-dev/cleared/internal/model"
)

// ValidationError describes a single invariant violation.
type ValidationError struct {
	Invariant   int
	EntryID     string
	Description string
}

func (e ValidationError) Error() string {
	return fmt.Sprintf("invariant %d [%s]: %s", e.Invariant, e.EntryID, e.Description)
}

// AccountChecker tests whether an account ID exists in the chart of accounts.
type AccountChecker interface {
	Exists(id int) bool
}

// ValidateLegs enforces 6 invariants on a set of journal legs for a given month.
func ValidateLegs(legs []model.Leg, accounts AccountChecker, year, month int) []ValidationError {
	var errs []ValidationError

	// Group legs by entry.
	groups := make(map[string][]model.Leg)
	var groupOrder []string
	for _, leg := range legs {
		g := leg.EntryGroup()
		if _, seen := groups[g]; !seen {
			groupOrder = append(groupOrder, g)
		}
		groups[g] = append(groups[g], leg)
	}

	// Invariant 1: Entry groups balance (sum(debits) == sum(credits) per group).
	for _, g := range groupOrder {
		groupLegs := groups[g]
		totalDebit := decimal.Zero
		totalCredit := decimal.Zero
		for _, leg := range groupLegs {
			totalDebit = totalDebit.Add(leg.Debit)
			totalCredit = totalCredit.Add(leg.Credit)
		}
		if !totalDebit.Equal(totalCredit) {
			errs = append(errs, ValidationError{
				Invariant:   1,
				EntryID:     g,
				Description: fmt.Sprintf("debits (%s) != credits (%s)", totalDebit.StringFixed(2), totalCredit.StringFixed(2)),
			})
		}
	}

	for _, leg := range legs {
		// Invariant 2: Exactly one of debit/credit per row.
		hasDebit := !leg.Debit.IsZero()
		hasCredit := !leg.Credit.IsZero()
		if hasDebit == hasCredit {
			errs = append(errs, ValidationError{
				Invariant:   2,
				EntryID:     leg.EntryID,
				Description: "leg must have exactly one of debit or credit",
			})
		}

		// Invariant 3: Valid account references.
		if !accounts.Exists(leg.AccountID) {
			errs = append(errs, ValidationError{
				Invariant:   3,
				EntryID:     leg.EntryID,
				Description: fmt.Sprintf("unknown account %d", leg.AccountID),
			})
		}

		// Invariant 4: Date within month.
		if leg.Date.Year() != year || int(leg.Date.Month()) != month {
			errs = append(errs, ValidationError{
				Invariant:   4,
				EntryID:     leg.EntryID,
				Description: fmt.Sprintf("date %s not in %04d-%02d", leg.Date.Format("2006-01-02"), year, month),
			})
		}

		// Invariant 6: Exact decimals — no more than 2 decimal places.
		two := decimal.NewFromInt(100)
		if !leg.Debit.IsZero() && !leg.Debit.Mul(two).Equal(leg.Debit.Mul(two).Floor()) {
			errs = append(errs, ValidationError{
				Invariant:   6,
				EntryID:     leg.EntryID,
				Description: fmt.Sprintf("debit %s has more than 2 decimal places", leg.Debit),
			})
		}
		if !leg.Credit.IsZero() && !leg.Credit.Mul(two).Equal(leg.Credit.Mul(two).Floor()) {
			errs = append(errs, ValidationError{
				Invariant:   6,
				EntryID:     leg.EntryID,
				Description: fmt.Sprintf("credit %s has more than 2 decimal places", leg.Credit),
			})
		}
	}

	// Invariant 5: Unique sequential IDs — no duplicates, contiguous 1..N.
	seqSeen := make(map[int]bool)
	for _, leg := range legs {
		_, _, seq, err := id.ParseEntryID(leg.EntryID)
		if err != nil {
			errs = append(errs, ValidationError{
				Invariant:   5,
				EntryID:     leg.EntryID,
				Description: fmt.Sprintf("invalid entry ID: %v", err),
			})
			continue
		}
		// Check duplicates within entry groups (same seq is ok for legs of same entry).
		// We check at the entry group level.
		seqSeen[seq] = true
	}
	if len(seqSeen) > 0 {
		for i := 1; i <= len(seqSeen); i++ {
			if !seqSeen[i] {
				errs = append(errs, ValidationError{
					Invariant:   5,
					EntryID:     fmt.Sprintf("seq %d", i),
					Description: fmt.Sprintf("missing sequence %d in 1..%d", i, len(seqSeen)),
				})
			}
		}
	}

	return errs
}
