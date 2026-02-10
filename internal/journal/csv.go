package journal

import (
	"encoding/csv"
	"fmt"
	"io"
	"strconv"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/cleared-dev/cleared/internal/model"
)

// Header is the CSV header for journal.csv.
const Header = "entry_id,date,account_id,description,debit,credit,counterparty,reference,confidence,status,evidence,receipt_hash,tags,notes"

const (
	numFields   = 14
	dateFormat  = "2006-01-02"
	colEntryID  = 0
	colDate     = 1
	colAcctID   = 2
	colDesc     = 3
	colDebit    = 4
	colCredit   = 5
	colCparty   = 6
	colRef      = 7
	colConf     = 8
	colStatus   = 9
	colEvidence = 10
	colReceipt  = 11
	colTags     = 12
	colNotes    = 13
)

// ReadLegs reads all legs from a journal.csv reader.
func ReadLegs(r io.Reader) ([]model.Leg, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = numFields

	records, err := cr.ReadAll()
	if err != nil {
		return nil, fmt.Errorf("reading journal CSV: %w", err)
	}

	if len(records) == 0 {
		return nil, nil
	}

	// Skip header row.
	var legs []model.Leg
	for i, rec := range records[1:] {
		leg, err := UnmarshalLeg(rec)
		if err != nil {
			return nil, fmt.Errorf("row %d: %w", i+2, err)
		}
		legs = append(legs, leg)
	}
	return legs, nil
}

// WriteLegs writes legs to a journal.csv writer (including header).
func WriteLegs(w io.Writer, legs []model.Leg) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	if err := cw.Write(strings.Split(Header, ",")); err != nil {
		return fmt.Errorf("writing header: %w", err)
	}

	for i, leg := range legs {
		if err := cw.Write(MarshalLeg(leg)); err != nil {
			return fmt.Errorf("writing row %d: %w", i+2, err)
		}
	}
	return cw.Error()
}

// AppendLegs appends legs to an existing journal.csv writer (no header).
func AppendLegs(w io.Writer, legs []model.Leg) error {
	cw := csv.NewWriter(w)
	defer cw.Flush()

	for i, leg := range legs {
		if err := cw.Write(MarshalLeg(leg)); err != nil {
			return fmt.Errorf("writing row %d: %w", i, err)
		}
	}
	return cw.Error()
}

// MarshalLeg converts a Leg to a CSV row ([]string).
func MarshalLeg(leg model.Leg) []string {
	row := make([]string, numFields)
	row[colEntryID] = leg.EntryID
	row[colDate] = leg.Date.Format(dateFormat)
	row[colAcctID] = strconv.Itoa(leg.AccountID)
	row[colDesc] = leg.Description

	if !leg.Debit.IsZero() {
		row[colDebit] = leg.Debit.StringFixed(2)
	}
	if !leg.Credit.IsZero() {
		row[colCredit] = leg.Credit.StringFixed(2)
	}

	row[colCparty] = leg.Counterparty
	row[colRef] = leg.Reference

	if !leg.Confidence.IsZero() {
		row[colConf] = leg.Confidence.String()
	}

	row[colStatus] = string(leg.Status)
	row[colEvidence] = leg.Evidence
	row[colReceipt] = leg.ReceiptHash
	row[colTags] = leg.Tags
	row[colNotes] = leg.Notes

	return row
}

// UnmarshalLeg converts a CSV row to a Leg.
func UnmarshalLeg(record []string) (model.Leg, error) {
	if len(record) != numFields {
		return model.Leg{}, fmt.Errorf("expected %d fields, got %d", numFields, len(record))
	}

	date, err := time.Parse(dateFormat, record[colDate])
	if err != nil {
		return model.Leg{}, fmt.Errorf("parsing date %q: %w", record[colDate], err)
	}

	accountID, err := strconv.Atoi(record[colAcctID])
	if err != nil {
		return model.Leg{}, fmt.Errorf("parsing account_id %q: %w", record[colAcctID], err)
	}

	var debit, credit, confidence decimal.Decimal

	if record[colDebit] != "" {
		debit, err = decimal.NewFromString(record[colDebit])
		if err != nil {
			return model.Leg{}, fmt.Errorf("parsing debit %q: %w", record[colDebit], err)
		}
	}

	if record[colCredit] != "" {
		credit, err = decimal.NewFromString(record[colCredit])
		if err != nil {
			return model.Leg{}, fmt.Errorf("parsing credit %q: %w", record[colCredit], err)
		}
	}

	if record[colConf] != "" {
		confidence, err = decimal.NewFromString(record[colConf])
		if err != nil {
			return model.Leg{}, fmt.Errorf("parsing confidence %q: %w", record[colConf], err)
		}
	}

	return model.Leg{
		EntryID:      record[colEntryID],
		Date:         date,
		AccountID:    accountID,
		Description:  record[colDesc],
		Debit:        debit,
		Credit:       credit,
		Counterparty: record[colCparty],
		Reference:    record[colRef],
		Confidence:   confidence,
		Status:       model.EntryStatus(record[colStatus]),
		Evidence:     record[colEvidence],
		ReceiptHash:  record[colReceipt],
		Tags:         record[colTags],
		Notes:        record[colNotes],
	}, nil
}
