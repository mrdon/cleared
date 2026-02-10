package sandbox

import (
	"testing"
	"time"

	"github.com/shopspring/decimal"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cleared-dev/cleared/internal/config"
	"github.com/cleared-dev/cleared/internal/model"
)

func TestParseDate(t *testing.T) {
	tests := []struct {
		input    any
		expected time.Time
		wantErr  bool
	}{
		{"2025-01-15", time.Date(2025, 1, 15, 0, 0, 0, 0, time.UTC), false},
		{"2025-12-31", time.Date(2025, 12, 31, 0, 0, 0, 0, time.UTC), false},
		{"not-a-date", time.Time{}, true},
		{42, time.Time{}, true},
		{nil, time.Time{}, true},
	}
	for _, tc := range tests {
		result, err := parseDate(tc.input)
		if tc.wantErr {
			assert.Error(t, err, "input: %v", tc.input)
		} else {
			require.NoError(t, err, "input: %v", tc.input)
			assert.Equal(t, tc.expected, result)
		}
	}
}

func TestParseDecimal(t *testing.T) {
	tests := []struct {
		input    any
		expected string
		wantErr  bool
	}{
		{float64(4.00), "4", false},
		{float64(-127.50), "-127.5", false},
		{float64(0), "0", false},
		{"3.14", "3.14", false},
		{nil, "0", false},
		{true, "", true},
	}
	for _, tc := range tests {
		result, err := parseDecimal(tc.input)
		if tc.wantErr {
			assert.Error(t, err, "input: %v", tc.input)
		} else {
			require.NoError(t, err, "input: %v", tc.input)
			expected, _ := decimal.NewFromString(tc.expected)
			assert.True(t, expected.Equal(result), "expected %s, got %s (input: %v)", tc.expected, result, tc.input)
		}
	}
}

func TestConfigLookup(t *testing.T) {
	cfg := &config.Config{
		Business: config.BusinessConfig{
			Name:       "Test Corp",
			EntityType: "llc_single_member",
		},
		Fiscal: config.FiscalConfig{
			YearStart: "01-01",
		},
		Thresholds: config.ThresholdsConfig{
			AutoConfirm: 0.95,
			ReviewFlag:  0.70,
		},
		Git: config.GitConfig{
			AutoCommit:  true,
			AuthorName:  "Cleared Agent",
			AuthorEmail: "agent@cleared.dev",
		},
	}

	tests := []struct {
		path     string
		expected any
	}{
		{"business.name", "Test Corp"},
		{"business.entity_type", "llc_single_member"},
		{"fiscal.year_start", "01-01"},
		{"thresholds.auto_confirm", 0.95},
		{"thresholds.review_flag", 0.70},
		{"git.auto_commit", true},
		{"git.author_name", "Cleared Agent"},
		{"git.author_email", "agent@cleared.dev"},
		{"nonexistent.path", nil},
		{"", nil},
	}
	for _, tc := range tests {
		result := configLookup(cfg, tc.path)
		assert.Equal(t, tc.expected, result, "path: %s", tc.path)
	}
}

func TestAccountToMap(t *testing.T) {
	acct := model.Account{
		ID:          1010,
		Name:        "Business Checking",
		Type:        model.AccountTypeAsset,
		Description: "Primary checking account",
	}

	m := accountToMap(acct)
	assert.Equal(t, 1010, m["id"])
	assert.Equal(t, "Business Checking", m["name"])
	assert.Equal(t, "asset", m["type"])
	assert.Equal(t, "Primary checking account", m["description"])
	_, hasParent := m["parent_id"]
	assert.False(t, hasParent, "parent_id should be omitted when 0")
}

func TestAccountToMap_WithParent(t *testing.T) {
	acct := model.Account{
		ID:       5010,
		Name:     "Advertising",
		Type:     model.AccountTypeExpense,
		ParentID: 5000,
	}

	m := accountToMap(acct)
	assert.Equal(t, 5000, m["parent_id"])
}

func TestTransactionToMap(t *testing.T) {
	txn := model.BankTransaction{
		Date:        time.Date(2025, 1, 3, 0, 0, 0, 0, time.UTC),
		Description: "GITHUB *PRO",
		Amount:      decimal.NewFromFloat(-4.00),
		Reference:   "chase_20250103_GITHUBPRO",
	}

	m := transactionToMap(txn)
	assert.Equal(t, "2025-01-03", m["date"])
	assert.Equal(t, "GITHUB *PRO", m["description"])
	assert.InDelta(t, -4.0, m["amount"], 0.001)
	assert.Equal(t, "chase_20250103_GITHUBPRO", m["reference"])
}

func TestStringArg(t *testing.T) {
	m := map[string]any{"key": "value", "num": 42}
	assert.Equal(t, "value", stringArg(m, "key"))
	assert.Empty(t, stringArg(m, "num"))
	assert.Empty(t, stringArg(m, "missing"))
}

func TestIntArg(t *testing.T) {
	m := map[string]any{"id": float64(1010), "name": "test"}
	assert.Equal(t, 1010, intArg(m, "id"))
	assert.Equal(t, 0, intArg(m, "name"))
	assert.Equal(t, 0, intArg(m, "missing"))
}
