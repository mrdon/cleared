package journal

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cleared-dev/cleared/internal/model"
)

func TestAddDouble_NewMonth(t *testing.T) {
	dir := t.TempDir()
	accts := newMockAccounts(1010, 5020)
	svc := NewService(dir, accts)

	entryID, err := svc.AddDouble(AddDoubleParams{
		Date:          date(2025, 1, 15),
		Description:   "GitHub subscription",
		DebitAccount:  5020,
		CreditAccount: 1010,
		Amount:        dec("4.00"),
		Counterparty:  "GitHub",
		Status:        model.StatusAutoConfirmed,
		Confidence:    dec("0.98"),
	})
	require.NoError(t, err)
	assert.Equal(t, "2025-01-001", entryID)

	// Verify file was created.
	path := filepath.Join(dir, "2025", "01", "journal.csv")
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Verify legs were written.
	legs, err := svc.ReadMonth(2025, 1)
	require.NoError(t, err)
	require.Len(t, legs, 2)
	assert.True(t, legs[0].Debit.Equal(dec("4.00")))
	assert.True(t, legs[1].Credit.Equal(dec("4.00")))
}

func TestAddDouble_ExistingMonth(t *testing.T) {
	dir := t.TempDir()
	accts := newMockAccounts(1010, 5020)
	svc := NewService(dir, accts)

	// First entry.
	_, err := svc.AddDouble(AddDoubleParams{
		Date:          date(2025, 1, 10),
		Description:   "First entry",
		DebitAccount:  5020,
		CreditAccount: 1010,
		Amount:        dec("10.00"),
		Status:        model.StatusAutoConfirmed,
		Confidence:    dec("0.95"),
	})
	require.NoError(t, err)

	// Second entry.
	entryID, err := svc.AddDouble(AddDoubleParams{
		Date:          date(2025, 1, 20),
		Description:   "Second entry",
		DebitAccount:  5020,
		CreditAccount: 1010,
		Amount:        dec("20.00"),
		Status:        model.StatusAutoConfirmed,
		Confidence:    dec("0.90"),
	})
	require.NoError(t, err)
	assert.Equal(t, "2025-01-002", entryID)

	legs, err := svc.ReadMonth(2025, 1)
	require.NoError(t, err)
	require.Len(t, legs, 4, "two entries x 2 legs")
}

func TestAddDouble_ValidationFailure(t *testing.T) {
	dir := t.TempDir()
	accts := newMockAccounts(1010) // 5020 does NOT exist
	svc := NewService(dir, accts)

	_, err := svc.AddDouble(AddDoubleParams{
		Date:          date(2025, 1, 15),
		Description:   "Bad entry",
		DebitAccount:  5020, // unknown account
		CreditAccount: 1010,
		Amount:        dec("50.00"),
		Status:        model.StatusAutoConfirmed,
		Confidence:    dec("0.80"),
	})
	require.Error(t, err)
	assert.Contains(t, err.Error(), "validation failed")

	// Verify nothing was written.
	legs, err := svc.ReadMonth(2025, 1)
	require.NoError(t, err)
	assert.Empty(t, legs)
}

func TestAddDouble_Balance(t *testing.T) {
	dir := t.TempDir()
	accts := newMockAccounts(1010, 5020)
	svc := NewService(dir, accts)

	_, err := svc.AddDouble(AddDoubleParams{
		Date:          date(2025, 3, 1),
		Description:   "Balance test",
		DebitAccount:  5020,
		CreditAccount: 1010,
		Amount:        dec("999.99"),
		Status:        model.StatusAutoConfirmed,
		Confidence:    dec("0.99"),
	})
	require.NoError(t, err)

	legs, err := svc.ReadMonth(2025, 3)
	require.NoError(t, err)
	require.Len(t, legs, 2)
	assert.True(t, legs[0].Debit.Equal(legs[1].Credit), "entry must balance")
}

func TestAddDouble_DirectoryCreation(t *testing.T) {
	dir := t.TempDir()
	accts := newMockAccounts(1010, 5020)
	svc := NewService(dir, accts)

	_, err := svc.AddDouble(AddDoubleParams{
		Date:          date(2025, 12, 25),
		Description:   "December entry",
		DebitAccount:  5020,
		CreditAccount: 1010,
		Amount:        dec("25.00"),
		Status:        model.StatusAutoConfirmed,
		Confidence:    dec("0.95"),
	})
	require.NoError(t, err)

	// Verify directory structure was created.
	journalDir := filepath.Join(dir, "2025", "12")
	info, err := os.Stat(journalDir)
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}

func TestNextEntrySeq(t *testing.T) {
	dir := t.TempDir()
	accts := newMockAccounts(1010, 5020)
	svc := NewService(dir, accts)

	// Empty month — seq should be 1.
	seq, err := svc.NextEntrySeq(2025, 1)
	require.NoError(t, err)
	assert.Equal(t, 1, seq)

	// Add an entry, next seq should be 2.
	_, err = svc.AddDouble(AddDoubleParams{
		Date:          date(2025, 1, 1),
		Description:   "First",
		DebitAccount:  5020,
		CreditAccount: 1010,
		Amount:        dec("1.00"),
		Status:        model.StatusAutoConfirmed,
		Confidence:    dec("0.95"),
	})
	require.NoError(t, err)

	seq, err = svc.NextEntrySeq(2025, 1)
	require.NoError(t, err)
	assert.Equal(t, 2, seq)
}

func TestReadMonth_NonExistent(t *testing.T) {
	dir := t.TempDir()
	accts := newMockAccounts()
	svc := NewService(dir, accts)

	legs, err := svc.ReadMonth(2025, 6)
	require.NoError(t, err)
	assert.Empty(t, legs)
}
