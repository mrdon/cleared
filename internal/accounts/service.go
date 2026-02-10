package accounts

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/cleared-dev/cleared/internal/model"
)

// Service provides in-memory lookup over the chart of accounts.
type Service struct {
	accounts []model.Account
	byID     map[int]model.Account
}

// NewService creates a Service from a slice of accounts.
func NewService(accounts []model.Account) *Service {
	byID := make(map[int]model.Account, len(accounts))
	for _, a := range accounts {
		byID[a.ID] = a
	}
	return &Service{accounts: accounts, byID: byID}
}

// Load reads chart-of-accounts.csv from a repo root and returns a Service.
func Load(repoRoot string) (*Service, error) {
	path := filepath.Join(repoRoot, "accounts", "chart-of-accounts.csv")
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("opening chart of accounts: %w", err)
	}
	defer f.Close()

	accts, err := ReadAccounts(f)
	if err != nil {
		return nil, fmt.Errorf("reading chart of accounts: %w", err)
	}
	return NewService(accts), nil
}

// All returns all accounts.
func (s *Service) All() []model.Account {
	return s.accounts
}

// Get returns an account by ID.
func (s *Service) Get(id int) (model.Account, bool) {
	a, ok := s.byID[id]
	return a, ok
}

// Exists reports whether an account ID exists.
func (s *Service) Exists(id int) bool {
	_, ok := s.byID[id]
	return ok
}

// ByType returns all accounts of the given type.
func (s *Service) ByType(accountType model.AccountType) []model.Account {
	var result []model.Account
	for _, a := range s.accounts {
		if a.Type == accountType {
			result = append(result, a)
		}
	}
	return result
}

// Save writes the chart of accounts to accounts/chart-of-accounts.csv.
func (s *Service) Save(repoRoot string) error {
	dir := filepath.Join(repoRoot, "accounts")
	if err := os.MkdirAll(dir, 0o755); err != nil {
		return fmt.Errorf("creating accounts dir: %w", err)
	}

	path := filepath.Join(dir, "chart-of-accounts.csv")
	f, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("creating chart of accounts file: %w", err)
	}
	defer f.Close()

	if err := WriteAccounts(f, s.accounts); err != nil {
		return fmt.Errorf("writing chart of accounts: %w", err)
	}
	return nil
}
