package importer

import (
	"encoding/csv"
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/cleared-dev/cleared/internal/model"
)

// ChaseParser parses Chase bank checking CSV exports.
type ChaseParser struct{}

const (
	chaseDateFormat = "01/02/2006"
	chaseNumFields  = 7
	chaseColDate    = 1
	chaseColDesc    = 2
	chaseColAmount  = 3
	chaseColType    = 4
)

// Format returns the parser name.
func (p *ChaseParser) Format() string { return "chase" }

// Parse reads a Chase CSV and returns BankTransactions.
func (p *ChaseParser) Parse(r io.Reader) ([]model.BankTransaction, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = chaseNumFields

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading chase CSV: %w", err)
	}

	if len(records) <= 1 {
		return nil, nil
	}

	var txns []model.BankTransaction
	for i, rec := range records[1:] {
		txn, err := parseChaseRow(rec)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+2, err)
		}
		txns = append(txns, txn)
	}
	return txns, nil
}

func parseChaseRow(rec []string) (model.BankTransaction, error) {
	date, err := time.Parse(chaseDateFormat, rec[chaseColDate])
	if err != nil {
		return model.BankTransaction{}, fmt.Errorf("parsing date %q: %w", rec[chaseColDate], err)
	}

	amount, err := decimal.NewFromString(rec[chaseColAmount])
	if err != nil {
		return model.BankTransaction{}, fmt.Errorf("parsing amount %q: %w", rec[chaseColAmount], err)
	}

	desc := rec[chaseColDesc]
	ref := makeChaseRef(date, desc)

	return model.BankTransaction{
		Date:        date,
		Description: desc,
		Amount:      amount,
		Reference:   ref,
		Type:        rec[chaseColType],
	}, nil
}

// makeChaseRef creates a reference like chase_20250103_GITHUB.
func makeChaseRef(date time.Time, desc string) string {
	prefix := strings.Map(func(r rune) rune {
		if r >= 'A' && r <= 'Z' || r >= 'a' && r <= 'z' || r >= '0' && r <= '9' {
			return r
		}
		return -1
	}, desc)
	if len(prefix) > 10 {
		prefix = prefix[:10]
	}
	return fmt.Sprintf("chase_%s_%s", date.Format("20060102"), prefix)
}
