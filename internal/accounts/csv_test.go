package accounts

import (
	"bytes"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"github.com/cleared-dev/cleared/internal/model"
)

func TestRoundTrip(t *testing.T) {
	accounts := []model.Account{
		{ID: 1010, Name: "Business Checking", Type: model.AccountTypeAsset, Description: "Primary checking account"},
		{ID: 5020, Name: "Software & SaaS", Type: model.AccountTypeExpense, TaxLine: "schedule_c_18", Description: "Software subscriptions"},
	}

	var buf bytes.Buffer
	err := WriteAccounts(&buf, accounts)
	require.NoError(t, err)

	got, err := ReadAccounts(&buf)
	require.NoError(t, err)
	require.Len(t, got, 2)

	assert.Equal(t, accounts[0].ID, got[0].ID)
	assert.Equal(t, accounts[0].Name, got[0].Name)
	assert.Equal(t, accounts[0].Type, got[0].Type)
	assert.Equal(t, accounts[0].Description, got[0].Description)

	assert.Equal(t, accounts[1].ID, got[1].ID)
	assert.Equal(t, accounts[1].TaxLine, got[1].TaxLine)
}

func TestParentID(t *testing.T) {
	accounts := []model.Account{
		{ID: 1010, Name: "Checking", Type: model.AccountTypeAsset},
		{ID: 1011, Name: "Sub-checking", Type: model.AccountTypeAsset, ParentID: 1010},
	}

	var buf bytes.Buffer
	err := WriteAccounts(&buf, accounts)
	require.NoError(t, err)

	got, err := ReadAccounts(&buf)
	require.NoError(t, err)
	require.Len(t, got, 2)

	assert.Equal(t, 0, got[0].ParentID)
	assert.Equal(t, 1010, got[1].ParentID)
}

func TestDefaultChart(t *testing.T) {
	chart := DefaultChart("llc_single_member")
	require.NotEmpty(t, chart)

	// Verify expected accounts exist.
	ids := make(map[int]bool)
	for _, acct := range chart {
		ids[acct.ID] = true
	}
	assert.True(t, ids[1010], "expected Business Checking (1010)")
	assert.True(t, ids[5020], "expected Software & SaaS (5020)")
	assert.True(t, ids[5050], "expected Shipping & Postage (5050)")

	// Verify all accounts have a name and type.
	for _, acct := range chart {
		assert.NotEmpty(t, acct.Name, "account %d missing name", acct.ID)
		assert.NotEmpty(t, acct.Type, "account %d missing type", acct.ID)
	}
}

func TestDefaultChart_UnknownEntityType(t *testing.T) {
	// Unknown entity types fall back to LLC single member.
	chart := DefaultChart("unknown_type")
	assert.NotEmpty(t, chart)
}

func TestReadTestdata(t *testing.T) {
	f, err := os.Open("../../testdata/chart-of-accounts.csv")
	require.NoError(t, err)
	defer f.Close()

	accounts, err := ReadAccounts(f)
	require.NoError(t, err)
	require.Len(t, accounts, 11, "default COA has 11 accounts")

	// Verify account types span all five categories.
	types := make(map[model.AccountType]bool)
	for _, acct := range accounts {
		types[acct.Type] = true
	}
	assert.True(t, types[model.AccountTypeAsset])
	assert.True(t, types[model.AccountTypeLiability])
	assert.True(t, types[model.AccountTypeEquity])
	assert.True(t, types[model.AccountTypeRevenue])
	assert.True(t, types[model.AccountTypeExpense])
}

func TestAllAccountTypes(t *testing.T) {
	accountTypes := []model.AccountType{
		model.AccountTypeAsset,
		model.AccountTypeLiability,
		model.AccountTypeEquity,
		model.AccountTypeRevenue,
		model.AccountTypeExpense,
	}
	for _, at := range accountTypes {
		acct := model.Account{
			ID:   1000,
			Name: "Test",
			Type: at,
		}

		var buf bytes.Buffer
		err := WriteAccounts(&buf, []model.Account{acct})
		require.NoError(t, err)

		got, err := ReadAccounts(&buf)
		require.NoError(t, err)
		require.Len(t, got, 1)
		assert.Equal(t, at, got[0].Type, "account type %q should survive round-trip", at)
	}
}

func TestDefaultChartRoundTrip(t *testing.T) {
	// Write the default chart to CSV and read it back — verify nothing is lost.
	chart := DefaultChart("llc_single_member")

	var buf bytes.Buffer
	err := WriteAccounts(&buf, chart)
	require.NoError(t, err)

	got, err := ReadAccounts(&buf)
	require.NoError(t, err)
	require.Len(t, got, len(chart))

	for i := range chart {
		assert.Equal(t, chart[i].ID, got[i].ID)
		assert.Equal(t, chart[i].Name, got[i].Name)
		assert.Equal(t, chart[i].Type, got[i].Type)
		assert.Equal(t, chart[i].ParentID, got[i].ParentID)
		assert.Equal(t, chart[i].TaxLine, got[i].TaxLine)
		assert.Equal(t, chart[i].Description, got[i].Description)
	}
}
