package sandbox

import (
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/shopspring/decimal"

	"github.com/cleared-dev/cleared/internal/accounts"
	"github.com/cleared-dev/cleared/internal/agentlog"
	"github.com/cleared-dev/cleared/internal/config"
	"github.com/cleared-dev/cleared/internal/gitops"
	"github.com/cleared-dev/cleared/internal/importer"
	"github.com/cleared-dev/cleared/internal/journal"
	"github.com/cleared-dev/cleared/internal/model"
)

// Runtime holds references to all services and registers primitives on a Bridge.
type Runtime struct {
	repoRoot   string
	cfg        *config.Config
	accounts   *accounts.Service
	journal    *journal.Service
	agentLog   []agentlog.Entry
	agentName  string
	dryRun     bool
	queueItems []map[string]any
}

// NewRuntime loads config, accounts, and journal services from a repo root.
func NewRuntime(repoRoot, agentName string, dryRun bool) (*Runtime, error) {
	cfg, err := config.Load(filepath.Join(repoRoot, "cleared.yaml"))
	if err != nil {
		return nil, fmt.Errorf("loading config: %w", err)
	}

	accts, err := accounts.Load(repoRoot)
	if err != nil {
		return nil, fmt.Errorf("loading accounts: %w", err)
	}

	jrnl := journal.NewService(repoRoot, accts)

	return &Runtime{
		repoRoot:  repoRoot,
		cfg:       cfg,
		accounts:  accts,
		journal:   jrnl,
		agentName: agentName,
		dryRun:    dryRun,
	}, nil
}

// AgentLog returns the collected agent log entries.
func (rt *Runtime) AgentLog() []agentlog.Entry {
	return rt.agentLog
}

// Register registers all primitives on the given bridge.
func (rt *Runtime) Register(b *Bridge) {
	b.RegisterPrimitive("importer_scan", rt.importerScan)
	b.RegisterPrimitive("importer_parse", rt.importerParse)
	b.RegisterPrimitive("importer_mark_processed", rt.importerMarkProcessed)
	b.RegisterPrimitive("importer_deduplicate", rt.importerDeduplicate)
	b.RegisterPrimitive("journal_add_double", rt.journalAddDouble)
	b.RegisterPrimitive("journal_query", rt.journalQuery)
	b.RegisterPrimitive("accounts_list", rt.accountsList)
	b.RegisterPrimitive("accounts_get", rt.accountsGet)
	b.RegisterPrimitive("accounts_exists", rt.accountsExists)
	b.RegisterPrimitive("accounts_by_type", rt.accountsByType)
	b.RegisterPrimitive("config_get", rt.configGet)
	b.RegisterPrimitive("git_commit", rt.gitCommit)
	b.RegisterPrimitive("ctx_log", rt.ctxLog)
	b.RegisterPrimitive("queue_add_review", rt.queueAddReview)
	b.RegisterPrimitive("ctx_dry_run", rt.ctxDryRun)
}

// --- Importer primitives ---

func (rt *Runtime) importerScan(_ []any, _ map[string]any) (any, error) {
	files, err := importer.Scan(rt.repoRoot)
	if err != nil {
		return nil, err
	}
	if len(files) == 0 {
		return []any{}, nil
	}
	result := make([]map[string]any, len(files))
	for i, f := range files {
		result[i] = map[string]any{
			"name": f.Name,
			"path": filepath.Join("import", f.Name),
			"size": f.Size,
		}
	}
	return result, nil
}

func (rt *Runtime) importerParse(args []any, _ map[string]any) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("importer_parse requires a filename argument")
	}
	fileName, _ := args[0].(string)

	path := filepath.Join(rt.repoRoot, "import", fileName)
	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", fileName, err)
	}
	defer f.Close()

	parser := importer.DefaultRegistry().Get("chase")
	if parser == nil {
		return nil, errors.New("no parser for format chase")
	}

	txns, err := parser.Parse(f)
	if err != nil {
		return nil, fmt.Errorf("parsing %s: %w", fileName, err)
	}

	result := make([]map[string]any, len(txns))
	for i, txn := range txns {
		result[i] = transactionToMap(txn)
	}
	return result, nil
}

func (rt *Runtime) importerMarkProcessed(args []any, _ map[string]any) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("importer_mark_processed requires a filename argument")
	}
	fileName, _ := args[0].(string)

	if err := importer.MarkProcessed(rt.repoRoot, fileName); err != nil {
		return nil, err
	}
	return map[string]any{"success": true}, nil
}

func (rt *Runtime) importerDeduplicate(args []any, _ map[string]any) (any, error) {
	if len(args) > 0 {
		return args[0], nil
	}
	return []any{}, nil
}

// --- Journal primitives ---

func (rt *Runtime) journalAddDouble(_ []any, kwargs map[string]any) (any, error) {
	date, err := parseDate(kwargs["date"])
	if err != nil {
		return nil, fmt.Errorf("invalid date: %w", err)
	}

	amount, err := parseDecimal(kwargs["amount"])
	if err != nil {
		return nil, fmt.Errorf("invalid amount: %w", err)
	}

	confidence, _ := parseDecimal(kwargs["confidence"])

	status, _ := kwargs["status"].(string)
	if status == "" {
		status = string(model.StatusPendingReview)
	}

	params := journal.AddDoubleParams{
		Date:          date,
		Description:   stringArg(kwargs, "description"),
		DebitAccount:  intArg(kwargs, "debit_account"),
		CreditAccount: intArg(kwargs, "credit_account"),
		Amount:        amount,
		Counterparty:  stringArg(kwargs, "counterparty"),
		Reference:     stringArg(kwargs, "reference"),
		Confidence:    confidence,
		Status:        model.EntryStatus(status),
		Evidence:      stringArg(kwargs, "evidence"),
		Tags:          stringArg(kwargs, "tags"),
		Notes:         stringArg(kwargs, "notes"),
	}

	entryID, err := rt.journal.AddDouble(params)
	if err != nil {
		return nil, err
	}

	return map[string]any{"entry_id": entryID, "success": true}, nil
}

func (rt *Runtime) journalQuery(_ []any, kwargs map[string]any) (any, error) {
	now := time.Now()
	year := intArgDefault(kwargs, "year", now.Year())
	month := intArgDefault(kwargs, "month", int(now.Month()))
	statusFilter := stringArg(kwargs, "status")

	legs, err := rt.journal.ReadMonth(year, month)
	if err != nil {
		return nil, err
	}

	var result []map[string]any
	for _, leg := range legs {
		if statusFilter != "" && string(leg.Status) != statusFilter {
			continue
		}
		result = append(result, legToMap(leg))
	}
	if result == nil {
		return []any{}, nil
	}
	return result, nil
}

// --- Accounts primitives ---

func (rt *Runtime) accountsList(_ []any, _ map[string]any) (any, error) {
	accts := rt.accounts.All()
	result := make([]map[string]any, len(accts))
	for i, a := range accts {
		result[i] = accountToMap(a)
	}
	return result, nil
}

func (rt *Runtime) accountsGet(args []any, _ map[string]any) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("accounts_get requires an account ID")
	}
	id := toInt(args[0])

	acct, ok := rt.accounts.Get(id)
	if !ok {
		return map[string]any{}, nil
	}
	return accountToMap(acct), nil
}

func (rt *Runtime) accountsExists(args []any, _ map[string]any) (any, error) {
	if len(args) == 0 {
		return false, nil
	}
	id := toInt(args[0])
	return rt.accounts.Exists(id), nil
}

func (rt *Runtime) accountsByType(args []any, _ map[string]any) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("accounts_by_type requires a type argument")
	}
	typeName, _ := args[0].(string)

	accts := rt.accounts.ByType(model.AccountType(typeName))
	result := make([]map[string]any, len(accts))
	for i, a := range accts {
		result[i] = accountToMap(a)
	}
	return result, nil
}

// --- Config primitive ---

func (rt *Runtime) configGet(args []any, _ map[string]any) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("config_get requires a key argument")
	}
	key, _ := args[0].(string)
	return configLookup(rt.cfg, key), nil
}

// --- Git primitive ---

func (rt *Runtime) gitCommit(args []any, _ map[string]any) (any, error) {
	if len(args) == 0 {
		return nil, errors.New("git_commit requires a message argument")
	}
	message, _ := args[0].(string)

	hash, err := gitops.CommitAll(
		rt.repoRoot,
		message,
		rt.cfg.Git.AuthorName,
		rt.cfg.Git.AuthorEmail,
	)
	if err != nil {
		return nil, err
	}

	return map[string]any{"commit_hash": hash, "success": true}, nil
}

// --- Context primitives ---

func (rt *Runtime) ctxLog(args []any, _ map[string]any) (any, error) {
	message := ""
	if len(args) > 0 {
		message, _ = args[0].(string)
	}

	rt.agentLog = append(rt.agentLog, agentlog.Entry{
		Timestamp: time.Now().UTC(),
		Agent:     rt.agentName,
		Action:    "log",
		Details:   message,
	})

	fmt.Fprintf(os.Stderr, "  [%s] %s\n", rt.agentName, message)
	return true, nil
}

func (rt *Runtime) queueAddReview(_ []any, kwargs map[string]any) (any, error) {
	rt.queueItems = append(rt.queueItems, kwargs)
	return map[string]any{
		"item_id": fmt.Sprintf("q%03d", len(rt.queueItems)),
		"success": true,
	}, nil
}

func (rt *Runtime) ctxDryRun(_ []any, _ map[string]any) (any, error) {
	return rt.dryRun, nil
}

// --- Type conversion helpers ---

func parseDate(v any) (time.Time, error) {
	s, ok := v.(string)
	if !ok {
		return time.Time{}, fmt.Errorf("expected string, got %T", v)
	}
	t, err := time.Parse("2006-01-02", s)
	if err != nil {
		return time.Time{}, fmt.Errorf("parsing %q: %w", s, err)
	}
	return t, nil
}

func parseDecimal(v any) (decimal.Decimal, error) {
	switch n := v.(type) {
	case float64:
		return decimal.NewFromFloat(n), nil
	case string:
		return decimal.NewFromString(n)
	case nil:
		return decimal.Zero, nil
	default:
		return decimal.Zero, fmt.Errorf("cannot convert %T to decimal", v)
	}
}

func configLookup(cfg *config.Config, path string) any {
	switch path {
	case "business.name":
		return cfg.Business.Name
	case "business.entity_type":
		return cfg.Business.EntityType
	case "fiscal.year_start":
		return cfg.Fiscal.YearStart
	case "thresholds.auto_confirm":
		return cfg.Thresholds.AutoConfirm
	case "thresholds.review_flag":
		return cfg.Thresholds.ReviewFlag
	case "git.auto_commit":
		return cfg.Git.AutoCommit
	case "git.author_name":
		return cfg.Git.AuthorName
	case "git.author_email":
		return cfg.Git.AuthorEmail
	default:
		return nil
	}
}

func accountToMap(a model.Account) map[string]any {
	m := map[string]any{
		"id":   a.ID,
		"name": a.Name,
		"type": string(a.Type),
	}
	if a.ParentID != 0 {
		m["parent_id"] = a.ParentID
	}
	if a.TaxLine != "" {
		m["tax_line"] = a.TaxLine
	}
	if a.Description != "" {
		m["description"] = a.Description
	}
	return m
}

func transactionToMap(txn model.BankTransaction) map[string]any {
	amount, _ := txn.Amount.Float64()
	return map[string]any{
		"date":        txn.Date.Format("2006-01-02"),
		"description": txn.Description,
		"amount":      amount,
		"reference":   txn.Reference,
	}
}

func legToMap(leg model.Leg) map[string]any {
	debit, _ := leg.Debit.Float64()
	credit, _ := leg.Credit.Float64()
	conf, _ := leg.Confidence.Float64()
	return map[string]any{
		"entry_id":     leg.EntryID,
		"date":         leg.Date.Format("2006-01-02"),
		"account_id":   leg.AccountID,
		"description":  leg.Description,
		"debit":        debit,
		"credit":       credit,
		"counterparty": leg.Counterparty,
		"reference":    leg.Reference,
		"confidence":   conf,
		"status":       string(leg.Status),
		"evidence":     leg.Evidence,
		"tags":         leg.Tags,
		"notes":        leg.Notes,
	}
}

func stringArg(m map[string]any, key string) string {
	v, _ := m[key].(string)
	return v
}

func intArg(m map[string]any, key string) int {
	return toInt(m[key])
}

func intArgDefault(m map[string]any, key string, def int) int {
	v, ok := m[key]
	if !ok {
		return def
	}
	n := toInt(v)
	if n == 0 {
		return def
	}
	return n
}
