package config

import (
	"fmt"
	"os"

	"gopkg.in/yaml.v3"
)

// Config represents the top-level cleared.yaml configuration.
type Config struct {
	Business     BusinessConfig   `yaml:"business"`
	Fiscal       FiscalConfig     `yaml:"fiscal"`
	BankAccounts []BankAccount    `yaml:"bank_accounts,omitempty"`
	Thresholds   ThresholdsConfig `yaml:"thresholds"`
	Git          GitConfig        `yaml:"git"`
}

// BusinessConfig identifies the business entity.
type BusinessConfig struct {
	Name       string `yaml:"name"`
	EntityType string `yaml:"entity_type"`
}

// FiscalConfig defines the fiscal year boundaries.
type FiscalConfig struct {
	YearStart string `yaml:"year_start"` // "MM-DD" format, e.g. "01-01"
}

// BankAccount maps a bank feed to a chart-of-accounts entry.
type BankAccount struct {
	Name      string `yaml:"name"`
	Type      string `yaml:"type"`
	LastFour  string `yaml:"last_four"`
	AccountID int    `yaml:"account_id"`
}

// ThresholdsConfig controls agent auto-confirmation behavior.
type ThresholdsConfig struct {
	AutoConfirm float64 `yaml:"auto_confirm"`
	ReviewFlag  float64 `yaml:"review_flag"`
}

// GitConfig controls git integration.
type GitConfig struct {
	AutoCommit  bool   `yaml:"auto_commit"`
	AuthorName  string `yaml:"author_name"`
	AuthorEmail string `yaml:"author_email"`
}

// Load reads a cleared.yaml file from disk.
func Load(path string) (*Config, error) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, fmt.Errorf("reading config: %w", err)
	}
	var cfg Config
	if err := yaml.Unmarshal(data, &cfg); err != nil {
		return nil, fmt.Errorf("parsing config: %w", err)
	}
	return &cfg, nil
}

// Save writes a Config to a YAML file.
func Save(path string, cfg *Config) error {
	data, err := yaml.Marshal(cfg)
	if err != nil {
		return fmt.Errorf("marshaling config: %w", err)
	}
	if err := os.WriteFile(path, data, 0o644); err != nil {
		return fmt.Errorf("writing config: %w", err)
	}
	return nil
}

// Default returns a Config with sensible defaults for a new project.
func Default(businessName, entityType string) *Config {
	return &Config{
		Business: BusinessConfig{
			Name:       businessName,
			EntityType: entityType,
		},
		Fiscal: FiscalConfig{
			YearStart: "01-01",
		},
		Thresholds: ThresholdsConfig{
			AutoConfirm: 0.95,
			ReviewFlag:  0.70,
		},
		Git: GitConfig{
			AutoCommit:  true,
			AuthorName:  "Cleared Agent",
			AuthorEmail: "agent@cleared.dev",
		},
	}
}
