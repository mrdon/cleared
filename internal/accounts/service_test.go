package accounts

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cleared-dev/cleared/internal/model"
)

func TestNewService(t *testing.T) {
	chart := DefaultChart("llc_single_member")
	svc := NewService(chart)

	assert.Len(t, svc.All(), len(chart))
}

func TestGetExists(t *testing.T) {
	chart := DefaultChart("llc_single_member")
	svc := NewService(chart)

	acct, ok := svc.Get(1010)
	assert.True(t, ok)
	assert.Equal(t, "Business Checking", acct.Name)

	_, ok = svc.Get(9999)
	assert.False(t, ok)

	assert.True(t, svc.Exists(1010))
	assert.False(t, svc.Exists(9999))
}

func TestByType(t *testing.T) {
	chart := DefaultChart("llc_single_member")
	svc := NewService(chart)

	assets := svc.ByType(model.AccountTypeAsset)
	assert.Len(t, assets, 2, "expected Business Checking + Business Savings")
	for _, a := range assets {
		assert.Equal(t, model.AccountTypeAsset, a.Type)
	}

	expenses := svc.ByType(model.AccountTypeExpense)
	assert.Len(t, expenses, 5)
}

func TestLoadFromTestdata(t *testing.T) {
	// testdata is at ../../testdata relative to internal/accounts/
	svc, err := Load("../../testdata/..")
	// The testdata chart-of-accounts.csv is at testdata/chart-of-accounts.csv,
	// but Load expects repoRoot/accounts/chart-of-accounts.csv. Use a temp dir.
	_ = svc
	_ = err

	// Set up a proper repo structure in a temp dir.
	dir := t.TempDir()
	acctDir := filepath.Join(dir, "accounts")
	require.NoError(t, os.MkdirAll(acctDir, 0o755))

	// Copy testdata chart to temp dir.
	src, err := os.ReadFile("../../testdata/chart-of-accounts.csv")
	require.NoError(t, err)
	require.NoError(t, os.WriteFile(filepath.Join(acctDir, "chart-of-accounts.csv"), src, 0o644))

	svc, err = Load(dir)
	require.NoError(t, err)
	assert.Len(t, svc.All(), 11)
	assert.True(t, svc.Exists(1010))
}

func TestSaveRoundTrip(t *testing.T) {
	chart := DefaultChart("llc_single_member")
	svc := NewService(chart)

	dir := t.TempDir()
	err := svc.Save(dir)
	require.NoError(t, err)

	// Verify file was created.
	path := filepath.Join(dir, "accounts", "chart-of-accounts.csv")
	_, err = os.Stat(path)
	require.NoError(t, err)

	// Load it back.
	svc2, err := Load(dir)
	require.NoError(t, err)
	assert.Len(t, svc2.All(), len(chart))

	for _, orig := range chart {
		got, ok := svc2.Get(orig.ID)
		require.True(t, ok, "account %d should exist", orig.ID)
		assert.Equal(t, orig.Name, got.Name)
		assert.Equal(t, orig.Type, got.Type)
	}
}
