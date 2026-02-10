package accounts

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"

	"github.com/cleared-dev/cleared/internal/model"
)

const (
	numFields  = 6
	colID      = 0
	colName    = 1
	colType    = 2
	colParent  = 3
	colTaxLine = 4
	colDesc    = 5
)

// ReadAccounts reads chart-of-accounts.csv.
func ReadAccounts(r io.Reader) ([]model.Account, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = numFields

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading accounts CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, nil
	}

	var accounts []model.Account
	for i, rec := range records[1:] {
		acct, err := UnmarshalAccount(rec)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+2, err)
		}
		accounts = append(accounts, acct)
	}
	return accounts, nil
}

// WriteAccounts writes chart-of-accounts.csv.
func WriteAccounts(w io.Writer, accounts []model.Account) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write([]string{"account_id", "account_name", "account_type", "parent_id", "tax_line", "description"}); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	for i, acct := range accounts {
		if err := cw.Write(MarshalAccount(acct)); err != nil {
			return fmt.Errorf("writing row %d: %w", i+2, err)
		}
	}
	return cw.Error()
}

// MarshalAccount converts an Account to a CSV row.
func MarshalAccount(acct model.Account) []string {
	row := make([]string, numFields)
	row[colID] = strconv.Itoa(acct.ID)
	row[colName] = acct.Name
	row[colType] = string(acct.Type)
	if acct.ParentID != 0 {
		row[colParent] = strconv.Itoa(acct.ParentID)
	}
	row[colTaxLine] = acct.TaxLine
	row[colDesc] = acct.Description
	return row
}

// UnmarshalAccount converts a CSV row to an Account.
func UnmarshalAccount(record []string) (model.Account, error) {
	if len(record) != numFields {
		return model.Account{}, fmt.Errorf("expected %d fields, got %d", numFields, len(record))
	}

	id, err := strconv.Atoi(record[colID])
	if err != nil {
		return model.Account{}, fmt.Errorf("parsing account_id %q: %w", record[colID], err)
	}

	var parentID int
	if record[colParent] != "" {
		parentID, err = strconv.Atoi(record[colParent])
		if err != nil {
			return model.Account{}, fmt.Errorf("parsing parent_id %q: %w", record[colParent], err)
		}
	}

	return model.Account{
		ID:          id,
		Name:        record[colName],
		Type:        model.AccountType(record[colType]),
		ParentID:    parentID,
		TaxLine:     record[colTaxLine],
		Description: record[colDesc],
	}, nil
}
