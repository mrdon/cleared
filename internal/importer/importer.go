package importer

import (
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"

	"github.com/cleared-dev/cleared/internal/model"
)

// Parser converts a bank CSV file into BankTransactions.
type Parser interface {
	Parse(r io.Reader) ([]model.BankTransaction, error)
	Format() string
}

// Registry holds named parsers.
type Registry struct {
	parsers map[string]Parser
}

// FileInfo describes a CSV file in the import directory.
type FileInfo struct {
	Name string
	Path string
	Size int64
}

// NewRegistry creates an empty parser registry.
func NewRegistry() *Registry {
	return &Registry{parsers: make(map[string]Parser)}
}

// Register adds a parser. Panics on duplicate format.
func (r *Registry) Register(p Parser) {
	key := strings.ToLower(p.Format())
	if _, ok := r.parsers[key]; ok {
		panic("duplicate parser format: " + key)
	}
	r.parsers[key] = p
}

// Get returns the parser for format, or nil.
func (r *Registry) Get(format string) Parser {
	return r.parsers[strings.ToLower(format)]
}

// DefaultRegistry returns a registry with all built-in parsers.
func DefaultRegistry() *Registry {
	r := NewRegistry()
	r.Register(&ChaseParser{})
	return r
}

// importDir is the subdirectory for import CSVs.
const importDir = "import"

// processedDir is the subdirectory for processed CSVs.
const processedDir = "import/processed"

// Scan returns CSV files in <repoRoot>/import/.
func Scan(repoRoot string) ([]FileInfo, error) {
	dir := filepath.Join(repoRoot, importDir)
	entries, err := os.ReadDir(dir)
	if err != nil {
		if os.IsNotExist(err) {
			return nil, nil
		}
		return nil, fmt.Errorf("reading import dir: %w", err)
	}

	var files []FileInfo
	for _, e := range entries {
		if e.IsDir() {
			continue
		}
		if !strings.HasSuffix(strings.ToLower(e.Name()), ".csv") {
			continue
		}
		info, err := e.Info()
		if err != nil {
			return nil, fmt.Errorf("stat %s: %w", e.Name(), err)
		}
		files = append(files, FileInfo{
			Name: e.Name(),
			Path: filepath.Join(dir, e.Name()),
			Size: info.Size(),
		})
	}
	return files, nil
}

// MarkProcessed moves a file from import/ to import/processed/.
func MarkProcessed(repoRoot, fileName string) error {
	src := filepath.Join(repoRoot, importDir, fileName)
	dstDir := filepath.Join(repoRoot, processedDir)

	if err := os.MkdirAll(dstDir, 0o755); err != nil {
		return fmt.Errorf("creating processed dir: %w", err)
	}

	dst := filepath.Join(dstDir, fileName)
	if err := os.Rename(src, dst); err != nil {
		return fmt.Errorf("moving %s to processed: %w", fileName, err)
	}
	return nil
}
