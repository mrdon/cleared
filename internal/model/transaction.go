package model

import (
	"time"

	"github.com/shopspring/decimal"
)

// BankTransaction represents a parsed bank CSV row.
type BankTransaction struct {
	Date        time.Time
	Description string
	Amount      decimal.Decimal // negative = expense, positive = income
	Reference   string
	Type        string // bank transaction type (ACH_DEBIT, etc.)
}
