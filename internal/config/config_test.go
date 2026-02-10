package config

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestRoundTrip(t *testing.T) {
	cfg := Default("Test Biz", "llc_single_member")
	cfg.BankAccounts = []BankAccount{
		{Name: "Chase Checking", Type: "checking", LastFour: "1234", AccountID: 1010},
	}

	path := filepath.Join(t.TempDir(), "cleared.yaml")
	err := Save(path, cfg)
	require.NoError(t, err)

	got, err := Load(path)
	require.NoError(t, err)

	assert.Equal(t, cfg.Business.Name, got.Business.Name)
	assert.Equal(t, cfg.Business.EntityType, got.Business.EntityType)
	assert.Equal(t, cfg.Fiscal.YearStart, got.Fiscal.YearStart)
	assert.InDelta(t, cfg.Thresholds.AutoConfirm, got.Thresholds.AutoConfirm, 0.001)
	assert.InDelta(t, cfg.Thresholds.ReviewFlag, got.Thresholds.ReviewFlag, 0.001)
	assert.Equal(t, cfg.Git.AutoCommit, got.Git.AutoCommit)
	assert.Equal(t, cfg.Git.AuthorName, got.Git.AuthorName)
	assert.Equal(t, cfg.Git.AuthorEmail, got.Git.AuthorEmail)
	require.Len(t, got.BankAccounts, 1)
	assert.Equal(t, "Chase Checking", got.BankAccounts[0].Name)
	assert.Equal(t, 1010, got.BankAccounts[0].AccountID)
}

func TestDefaults(t *testing.T) {
	cfg := Default("My Company", "llc_single_member")

	assert.Equal(t, "My Company", cfg.Business.Name)
	assert.Equal(t, "llc_single_member", cfg.Business.EntityType)
	assert.Equal(t, "01-01", cfg.Fiscal.YearStart)
	assert.InDelta(t, 0.95, cfg.Thresholds.AutoConfirm, 0.001)
	assert.InDelta(t, 0.70, cfg.Thresholds.ReviewFlag, 0.001)
	assert.True(t, cfg.Git.AutoCommit)
	assert.Equal(t, "Cleared Agent", cfg.Git.AuthorName)
	assert.Equal(t, "agent@cleared.dev", cfg.Git.AuthorEmail)
	assert.Empty(t, cfg.BankAccounts)
}

func TestLoadNotFound(t *testing.T) {
	_, err := Load(filepath.Join(t.TempDir(), "nonexistent.yaml"))
	require.Error(t, err)
	assert.ErrorIs(t, err, os.ErrNotExist)
}

func TestYAMLFormat(t *testing.T) {
	cfg := Default("Test Biz", "llc_single_member")
	path := filepath.Join(t.TempDir(), "cleared.yaml")
	err := Save(path, cfg)
	require.NoError(t, err)

	data, err := os.ReadFile(path)
	require.NoError(t, err)
	contents := string(data)

	assert.Contains(t, contents, "name: Test Biz")
	assert.Contains(t, contents, "entity_type: llc_single_member")
	assert.Contains(t, contents, "year_start: 01-01")
	assert.Contains(t, contents, "auto_commit: true")
}
