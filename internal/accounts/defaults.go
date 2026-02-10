package accounts

import "github.com/cleared-dev/cleared/internal/model"

// DefaultChart returns the default chart of accounts for an entity type.
func DefaultChart(entityType string) []model.Account {
	switch entityType {
	case "llc_single_member":
		return llcSingleMemberChart()
	default:
		return llcSingleMemberChart()
	}
}

func llcSingleMemberChart() []model.Account {
	return []model.Account{
		{ID: 1010, Name: "Business Checking", Type: model.AccountTypeAsset, Description: "Primary checking account"},
		{ID: 1020, Name: "Business Savings", Type: model.AccountTypeAsset, Description: "Savings account"},
		{ID: 2010, Name: "Credit Card", Type: model.AccountTypeLiability, Description: "Business credit card"},
		{ID: 3010, Name: "Owner's Equity", Type: model.AccountTypeEquity, Description: "Owner's equity"},
		{ID: 4010, Name: "Service Revenue", Type: model.AccountTypeRevenue},
		{ID: 4020, Name: "Product Revenue", Type: model.AccountTypeRevenue},
		{ID: 5010, Name: "Advertising & Marketing", Type: model.AccountTypeExpense, TaxLine: "schedule_c_8", Description: "Advertising costs"},
		{ID: 5020, Name: "Software & SaaS", Type: model.AccountTypeExpense, TaxLine: "schedule_c_18", Description: "Software subscriptions"},
		{ID: 5030, Name: "Office Supplies", Type: model.AccountTypeExpense, TaxLine: "schedule_c_18", Description: "Office supplies and expenses"},
		{ID: 5040, Name: "Professional Services", Type: model.AccountTypeExpense, TaxLine: "schedule_c_17", Description: "Legal, accounting, consulting"},
		{ID: 5050, Name: "Shipping & Postage", Type: model.AccountTypeExpense, TaxLine: "schedule_c_18", Description: "Postage and shipping costs"},
	}
}
