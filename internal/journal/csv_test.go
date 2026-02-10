package journal

import (
	"bytes"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cleared-dev/cleared/internal/model"
)

func date(y, m, d int) time.Time {
	return time.Date(y, time.Month(m), d, 0, 0, 0, 0, time.UTC)
}

func dec(s string) decimal.Decimal {
	d, _ := decimal.NewFromString(s)
	return d
}

func TestRoundTrip(t *testing.T) {
	legs := []model.Leg{
		{
			EntryID:      "2025-01-001a",
			Date:         date(2025, 1, 3),
			AccountID:    5020,
			Description:  "GitHub Pro subscription",
			Debit:        dec("4.00"),
			Credit:       decimal.Zero,
			Counterparty: "GitHub",
			Reference:    "plaid_abc123",
			Confidence:   dec("0.98"),
			Status:       model.StatusAutoConfirmed,
			Evidence:     "rule match: GITHUB*",
			Tags:         "recurring;software",
		},
		{
			EntryID:      "2025-01-001b",
			Date:         date(2025, 1, 3),
			AccountID:    1010,
			Description:  "GitHub Pro subscription",
			Debit:        decimal.Zero,
			Credit:       dec("4.00"),
			Counterparty: "GitHub",
			Reference:    "plaid_abc123",
			Confidence:   dec("0.98"),
			Status:       model.StatusAutoConfirmed,
			Evidence:     "rule match: GITHUB*",
			Tags:         "recurring;software",
		},
	}

	var buf bytes.Buffer
	err := WriteLegs(&buf, legs)
	require.NoError(t, err)

	// Verify header is present.
	assert.True(t, strings.HasPrefix(buf.String(), "entry_id,"))

	got, err := ReadLegs(&buf)
	require.NoError(t, err)
	require.Len(t, got, 2)

	for i := range legs {
		assert.Equal(t, legs[i].EntryID, got[i].EntryID)
		assert.True(t, legs[i].Date.Equal(got[i].Date))
		assert.Equal(t, legs[i].AccountID, got[i].AccountID)
		assert.Equal(t, legs[i].Description, got[i].Description)
		assert.True(t, legs[i].Debit.Equal(got[i].Debit), "debit mismatch row %d", i)
		assert.True(t, legs[i].Credit.Equal(got[i].Credit), "credit mismatch row %d", i)
		assert.Equal(t, legs[i].Counterparty, got[i].Counterparty)
		assert.Equal(t, legs[i].Reference, got[i].Reference)
		assert.True(t, legs[i].Confidence.Equal(got[i].Confidence))
		assert.Equal(t, legs[i].Status, got[i].Status)
		assert.Equal(t, legs[i].Evidence, got[i].Evidence)
		assert.Equal(t, legs[i].Tags, got[i].Tags)
	}
}

func TestZeroAmounts(t *testing.T) {
	// Debit-only leg — credit should remain zero.
	leg := model.Leg{
		EntryID:   "2025-01-002a",
		Date:      date(2025, 1, 5),
		AccountID: 5020,
		Debit:     dec("127.50"),
		Status:    model.StatusPendingReview,
	}

	row := MarshalLeg(leg)
	assert.Equal(t, "127.50", row[colDebit], "StringFixed(2) should preserve trailing zero")
	assert.Empty(t, row[colCredit])

	// Round-trip through CSV and verify decimal values survive.
	got, err := UnmarshalLeg(row)
	require.NoError(t, err)
	assert.True(t, got.Debit.Equal(dec("127.50")), "debit: got %s", got.Debit)
	assert.True(t, got.Credit.IsZero())
}

func TestEmptyOptionalFields(t *testing.T) {
	leg := model.Leg{
		EntryID:   "2025-01-003a",
		Date:      date(2025, 1, 10),
		AccountID: 5030,
		Debit:     dec("15.00"),
		Status:    model.StatusAutoConfirmed,
	}

	row := MarshalLeg(leg)
	got, err := UnmarshalLeg(row)
	require.NoError(t, err)
	assert.Empty(t, got.Counterparty)
	assert.Empty(t, got.Reference)
	assert.True(t, got.Confidence.IsZero())
	assert.Empty(t, got.Evidence)
	assert.Empty(t, got.ReceiptHash)
	assert.Empty(t, got.Tags)
	assert.Empty(t, got.Notes)
}

func TestSpecialCharactersInDescription(t *testing.T) {
	leg := model.Leg{
		EntryID:     "2025-01-004a",
		Date:        date(2025, 1, 15),
		AccountID:   4010,
		Description: `ACME CONSULTING, "Invoice 1042" — special chars & more`,
		Credit:      dec("3500.00"),
		Status:      model.StatusUserConfirmed,
	}

	var buf bytes.Buffer
	err := WriteLegs(&buf, []model.Leg{leg})
	require.NoError(t, err)

	got, err := ReadLegs(&buf)
	require.NoError(t, err)
	require.Len(t, got, 1)
	assert.Equal(t, leg.Description, got[0].Description)
}

func TestAppendLegs(t *testing.T) {
	var buf bytes.Buffer

	// Write initial legs with header.
	initial := []model.Leg{
		{
			EntryID:   "2025-01-001a",
			Date:      date(2025, 1, 3),
			AccountID: 5020,
			Debit:     dec("4.00"),
			Status:    model.StatusAutoConfirmed,
		},
	}
	err := WriteLegs(&buf, initial)
	require.NoError(t, err)

	// Append more legs (no header).
	extra := []model.Leg{
		{
			EntryID:   "2025-01-002a",
			Date:      date(2025, 1, 5),
			AccountID: 5020,
			Debit:     dec("127.50"),
			Status:    model.StatusAutoConfirmed,
		},
	}
	err = AppendLegs(&buf, extra)
	require.NoError(t, err)

	got, err := ReadLegs(&buf)
	require.NoError(t, err)
	require.Len(t, got, 2)
	assert.Equal(t, "2025-01-001a", got[0].EntryID)
	assert.Equal(t, "2025-01-002a", got[1].EntryID)
}

func TestReadLegs_Empty(t *testing.T) {
	legs, err := ReadLegs(strings.NewReader(""))
	require.NoError(t, err)
	assert.Nil(t, legs)
}

func TestReadLegs_HeaderOnly(t *testing.T) {
	legs, err := ReadLegs(strings.NewReader(Header + "\n"))
	require.NoError(t, err)
	assert.Empty(t, legs)
}

func TestReadTestdata(t *testing.T) {
	f, err := os.Open("../../testdata/journal.csv")
	require.NoError(t, err)
	defer f.Close()

	legs, err := ReadLegs(f)
	require.NoError(t, err)
	require.Len(t, legs, 12, "testdata has 6 transactions x 2 legs")

	// Verify first entry pair balances.
	assert.True(t, legs[0].Debit.Equal(legs[1].Credit), "entry 001 should balance")

	// Verify all legs have required fields.
	for i, leg := range legs {
		assert.NotEmpty(t, leg.EntryID, "leg %d missing entry_id", i)
		assert.False(t, leg.Date.IsZero(), "leg %d missing date", i)
		assert.NotZero(t, leg.AccountID, "leg %d missing account_id", i)
		assert.NotEmpty(t, string(leg.Status), "leg %d missing status", i)
	}
}

func TestDecimalPrecision(t *testing.T) {
	// Verify that decimal arithmetic survives a CSV round-trip.
	// This is the core guarantee of using shopspring/decimal over float64.
	debitLeg := model.Leg{
		EntryID:   "2025-01-010a",
		Date:      date(2025, 1, 10),
		AccountID: 5020,
		Debit:     dec("33.33"),
		Status:    model.StatusAutoConfirmed,
	}
	creditLeg := model.Leg{
		EntryID:   "2025-01-010b",
		Date:      date(2025, 1, 10),
		AccountID: 1010,
		Credit:    dec("33.33"),
		Status:    model.StatusAutoConfirmed,
	}

	var buf bytes.Buffer
	err := WriteLegs(&buf, []model.Leg{debitLeg, creditLeg})
	require.NoError(t, err)

	got, err := ReadLegs(&buf)
	require.NoError(t, err)
	require.Len(t, got, 2)

	// The key test: sum(debits) - sum(credits) must be exactly zero,
	// not a floating-point epsilon.
	balance := got[0].Debit.Sub(got[1].Credit)
	assert.True(t, balance.IsZero(), "balance should be exactly zero, got %s", balance)

	// Also test a value that's famously broken in float64: 0.1 + 0.2
	leg := model.Leg{
		EntryID:   "2025-01-011a",
		Date:      date(2025, 1, 11),
		AccountID: 5020,
		Debit:     dec("0.1").Add(dec("0.2")),
		Status:    model.StatusAutoConfirmed,
	}
	row := MarshalLeg(leg)
	got2, err := UnmarshalLeg(row)
	require.NoError(t, err)
	assert.True(t, got2.Debit.Equal(dec("0.30")), "0.1+0.2 should equal 0.30 exactly, got %s", got2.Debit)
}

func TestStringFixed2Formatting(t *testing.T) {
	tests := []struct {
		input string
		want  string
	}{
		{"4.00", "4.00"},
		{"127.5", "127.50"},
		{"3500", "3500.00"},
		{"0.10", "0.10"},
		{"42.99", "42.99"},
	}
	for _, tt := range tests {
		leg := model.Leg{
			EntryID:   "2025-01-001a",
			Date:      date(2025, 1, 1),
			AccountID: 5020,
			Debit:     dec(tt.input),
			Status:    model.StatusAutoConfirmed,
		}
		row := MarshalLeg(leg)
		assert.Equal(t, tt.want, row[colDebit], "input %q", tt.input)
	}
}

func TestAllStatusValues(t *testing.T) {
	statuses := []model.EntryStatus{
		model.StatusAutoConfirmed,
		model.StatusPendingReview,
		model.StatusUserConfirmed,
		model.StatusUserCorrected,
		model.StatusVoided,
		model.StatusBootstrapConfirmed,
	}
	for _, status := range statuses {
		leg := model.Leg{
			EntryID:   "2025-01-001a",
			Date:      date(2025, 1, 1),
			AccountID: 5020,
			Debit:     dec("1.00"),
			Status:    status,
		}

		var buf bytes.Buffer
		err := WriteLegs(&buf, []model.Leg{leg})
		require.NoError(t, err)

		got, err := ReadLegs(&buf)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, status, got[0].Status, "status %q should survive round-trip", status)
	}
}
