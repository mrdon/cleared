package model

// AccountType classifies accounts in the chart of accounts.
type AccountType string

const (
	AccountTypeAsset     AccountType = "asset"
	AccountTypeLiability AccountType = "liability"
	AccountTypeEquity    AccountType = "equity"
	AccountTypeRevenue   AccountType = "revenue"
	AccountTypeExpense   AccountType = "expense"
)

// Account represents a row in chart-of-accounts.csv.
type Account struct {
	ID          int
	Name        string
	Type        AccountType
	ParentID    int // 0 = top-level
	TaxLine     string
	Description string
}
