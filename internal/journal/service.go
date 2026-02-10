package journal

import (
	"errors"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
	"time"

	"github.com/shopspring/decimal"

	"github.com/cleared-dev/cleared/internal/id"
	"github.com/cleared-dev/cleared/internal/model"
)

// Service provides business logic for journal entries.
type Service struct {
	repoRoot string
	accounts AccountChecker
}

// NewService creates a journal Service.
func NewService(repoRoot string, accounts AccountChecker) *Service {
	return &Service{repoRoot: repoRoot, accounts: accounts}
}

// AddDoubleParams holds parameters for creating a double-entry journal entry.
type AddDoubleParams struct {
	Date          time.Time
	Description   string
	DebitAccount  int
	CreditAccount int
	Amount        decimal.Decimal
	Counterparty  string
	Reference     string
	Confidence    decimal.Decimal
	Status        model.EntryStatus
	Evidence      string
	Tags          string
	Notes         string
}

// AddDouble creates a balanced double-entry (debit + credit legs), validates,
// and appends to the month's journal.csv. Returns the entry ID.
func (s *Service) AddDouble(params AddDoubleParams) (string, error) {
	year := params.Date.Year()
	month := int(params.Date.Month())

	seq, err := s.NextEntrySeq(year, month)
	if err != nil {
		return "", err
	}

	entryID := id.FormatEntryID(year, month, seq)
	debitLegID := id.FormatLegID(entryID, 0)
	creditLegID := id.FormatLegID(entryID, 1)

	newLegs := []model.Leg{
		{
			EntryID:      debitLegID,
			Date:         params.Date,
			AccountID:    params.DebitAccount,
			Description:  params.Description,
			Debit:        params.Amount,
			Counterparty: params.Counterparty,
			Reference:    params.Reference,
			Confidence:   params.Confidence,
			Status:       params.Status,
			Evidence:     params.Evidence,
			Tags:         params.Tags,
			Notes:        params.Notes,
		},
		{
			EntryID:      creditLegID,
			Date:         params.Date,
			AccountID:    params.CreditAccount,
			Description:  params.Description,
			Credit:       params.Amount,
			Counterparty: params.Counterparty,
			Reference:    params.Reference,
			Confidence:   params.Confidence,
			Status:       params.Status,
			Evidence:     params.Evidence,
			Tags:         params.Tags,
			Notes:        params.Notes,
		},
	}

	// Read existing legs for validation.
	existing, err := s.ReadMonth(year, month)
	if err != nil {
		return "", err
	}

	// Validate ALL legs together.
	allLegs := append(existing, newLegs...)
	if verrs := ValidateLegs(allLegs, s.accounts, year, month); len(verrs) > 0 {
		msgs := make([]string, len(verrs))
		for i, ve := range verrs {
			msgs[i] = ve.Error()
		}
		return "", fmt.Errorf("validation failed: %s", strings.Join(msgs, "; "))
	}

	// Append to journal file (create dir + header if new).
	journalPath := s.monthPath(year, month)
	dir := filepath.Dir(journalPath)
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return "", fmt.Errorf("creating journal dir: %w", err)
	}

	isNew := false
	if _, err := os.Stat(journalPath); errors.Is(err, fs.ErrNotExist) {
		isNew = true
	}

	f, err := os.OpenFile(journalPath, os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0o644)
	if err != nil {
		return "", fmt.Errorf("opening journal: %w", err)
	}
	defer f.Close()

	if isNew {
		if _, err := fmt.Fprintln(f, Header); err != nil {
			return "", fmt.Errorf("writing header: %w", err)
		}
	}

	if err := AppendLegs(f, newLegs); err != nil {
		return "", fmt.Errorf("appending legs: %w", err)
	}

	return entryID, nil
}

// ReadMonth reads all legs for a given year/month.
func (s *Service) ReadMonth(year, month int) ([]model.Leg, error) {
	path := s.monthPath(year, month)
	f, err := os.Open(path)
	if errors.Is(err, fs.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("opening journal %s: %w", path, err)
	}
	defer f.Close()

	legs, err := ReadLegs(f)
	if err != nil {
		return nil, fmt.Errorf("reading journal %s: %w", path, err)
	}
	return legs, nil
}

// NextEntrySeq returns the next available sequence number for a month.
func (s *Service) NextEntrySeq(year, month int) (int, error) {
	legs, err := s.ReadMonth(year, month)
	if err != nil {
		return 0, err
	}

	maxSeq := 0
	for _, leg := range legs {
		_, _, seq, err := id.ParseEntryID(leg.EntryID)
		if err != nil {
			continue
		}
		if seq > maxSeq {
			maxSeq = seq
		}
	}
	return maxSeq + 1, nil
}

func (s *Service) monthPath(year, month int) string {
	return filepath.Join(s.repoRoot, fmt.Sprintf("%04d", year), fmt.Sprintf("%02d", month), "journal.csv")
}
