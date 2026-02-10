package importer

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestChaseParser_Parse(t *testing.T) {
	data, err := os.ReadFile("../../testdata/chase_checking.csv")
	require.NoError(t, err)

	p := &ChaseParser{}
	txns, err := p.Parse(strings.NewReader(string(data)))
	require.NoError(t, err)
	assert.Len(t, txns, 6)

	// First: GITHUB subscription
	assert.Equal(t, "GITHUB *PRO SUBSCRIPTION", txns[0].Description)
	assert.Equal(t, "-4.00", txns[0].Amount.StringFixed(2))
	assert.Equal(t, "ACH_DEBIT", txns[0].Type)
	assert.Equal(t, 2025, txns[0].Date.Year())
	assert.Equal(t, 1, int(txns[0].Date.Month()))
	assert.Equal(t, 3, txns[0].Date.Day())

	// Fourth: ACME income (positive)
	assert.Equal(t, "ACME CONSULTING INVOICE 1042", txns[3].Description)
	assert.True(t, txns[3].Amount.IsPositive())
	assert.Equal(t, "3500.00", txns[3].Amount.StringFixed(2))
}

func TestChaseParser_DateParsing(t *testing.T) {
	data, err := os.ReadFile("../../testdata/chase_checking.csv")
	require.NoError(t, err)

	p := &ChaseParser{}
	txns, err := p.Parse(strings.NewReader(string(data)))
	require.NoError(t, err)

	// Jan 22
	last := txns[5]
	assert.Equal(t, 2025, last.Date.Year())
	assert.Equal(t, 1, int(last.Date.Month()))
	assert.Equal(t, 22, last.Date.Day())
}

func TestChaseParser_NegativePositiveAmounts(t *testing.T) {
	data, err := os.ReadFile("../../testdata/chase_checking.csv")
	require.NoError(t, err)

	p := &ChaseParser{}
	txns, err := p.Parse(strings.NewReader(string(data)))
	require.NoError(t, err)

	for _, txn := range txns {
		if txn.Description == "ACME CONSULTING INVOICE 1042" {
			assert.True(t, txn.Amount.IsPositive())
		} else {
			assert.True(t, txn.Amount.IsNegative(), "expected negative for %s", txn.Description)
		}
	}
}

func TestChaseParser_EmptyFile(t *testing.T) {
	p := &ChaseParser{}
	txns, err := p.Parse(strings.NewReader("Details,Posting Date,Description,Amount,Type,Balance,Check or Slip #\n"))
	require.NoError(t, err)
	assert.Nil(t, txns)
}

func TestChaseParser_BadDate(t *testing.T) {
	csv := "Details,Posting Date,Description,Amount,Type,Balance,Check or Slip #\nDEBIT,NOTADATE,desc,-4.00,ACH_DEBIT,100.00,\n"
	p := &ChaseParser{}
	_, err := p.Parse(strings.NewReader(csv))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing date")
}

func TestChaseParser_BadAmount(t *testing.T) {
	csv := "Details,Posting Date,Description,Amount,Type,Balance,Check or Slip #\nDEBIT,01/03/2025,desc,NOTANUMBER,ACH_DEBIT,100.00,\n"
	p := &ChaseParser{}
	_, err := p.Parse(strings.NewReader(csv))
	assert.Error(t, err)
	assert.Contains(t, err.Error(), "parsing amount")
}

func TestChaseParser_Format(t *testing.T) {
	p := &ChaseParser{}
	assert.Equal(t, "chase", p.Format())
}

func TestChaseParser_Reference(t *testing.T) {
	data, err := os.ReadFile("../../testdata/chase_checking.csv")
	require.NoError(t, err)

	p := &ChaseParser{}
	txns, err := p.Parse(strings.NewReader(string(data)))
	require.NoError(t, err)

	// Reference format: chase_YYYYMMDD_<prefix>
	assert.Equal(t, "chase_20250103_GITHUBPROS", txns[0].Reference)
}

func TestRegistry_GetUnknown(t *testing.T) {
	r := NewRegistry()
	assert.Nil(t, r.Get("nonexistent"))
}

func TestRegistry_RegisterAndGet(t *testing.T) {
	r := NewRegistry()
	r.Register(&ChaseParser{})
	p := r.Get("chase")
	require.NotNil(t, p)
	assert.Equal(t, "chase", p.Format())
}

func TestRegistry_CaseInsensitive(t *testing.T) {
	r := NewRegistry()
	r.Register(&ChaseParser{})
	assert.NotNil(t, r.Get("Chase"))
	assert.NotNil(t, r.Get("CHASE"))
}

func TestDefaultRegistry(t *testing.T) {
	r := DefaultRegistry()
	assert.NotNil(t, r.Get("chase"))
}

func TestScan_FindsCSVs(t *testing.T) {
	dir := t.TempDir()
	importDir := filepath.Join(dir, "import")
	require.NoError(t, os.MkdirAll(importDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(importDir, "bank.csv"), []byte("data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(importDir, "other.txt"), []byte("data"), 0o644))

	files, err := Scan(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "bank.csv", files[0].Name)
}

func TestScan_IgnoresProcessedDir(t *testing.T) {
	dir := t.TempDir()
	importDir := filepath.Join(dir, "import")
	processedDir := filepath.Join(importDir, "processed")
	require.NoError(t, os.MkdirAll(processedDir, 0o755))

	require.NoError(t, os.WriteFile(filepath.Join(importDir, "new.csv"), []byte("data"), 0o644))
	require.NoError(t, os.WriteFile(filepath.Join(processedDir, "old.csv"), []byte("data"), 0o644))

	files, err := Scan(dir)
	require.NoError(t, err)
	assert.Len(t, files, 1)
	assert.Equal(t, "new.csv", files[0].Name)
}

func TestScan_EmptyDir(t *testing.T) {
	dir := t.TempDir()
	files, err := Scan(dir)
	require.NoError(t, err)
	assert.Nil(t, files)
}

func TestMarkProcessed(t *testing.T) {
	dir := t.TempDir()
	importDir := filepath.Join(dir, "import")
	require.NoError(t, os.MkdirAll(importDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(importDir, "bank.csv"), []byte("data"), 0o644))

	err := MarkProcessed(dir, "bank.csv")
	require.NoError(t, err)

	// Source gone.
	_, err = os.Stat(filepath.Join(importDir, "bank.csv"))
	assert.True(t, os.IsNotExist(err))

	// Destination exists.
	_, err = os.Stat(filepath.Join(dir, "import", "processed", "bank.csv"))
	assert.NoError(t, err)
}

func TestMarkProcessed_CreatesDir(t *testing.T) {
	dir := t.TempDir()
	importDir := filepath.Join(dir, "import")
	require.NoError(t, os.MkdirAll(importDir, 0o755))
	require.NoError(t, os.WriteFile(filepath.Join(importDir, "a.csv"), []byte("data"), 0o644))

	err := MarkProcessed(dir, "a.csv")
	require.NoError(t, err)

	info, err := os.Stat(filepath.Join(dir, "import", "processed"))
	require.NoError(t, err)
	assert.True(t, info.IsDir())
}
