// Spike 4: End-to-end loop.
//
// Real Go primitives + real Monty sandbox + real agent + real validation.
// init → import CSV → agent run → journal.csv + git commit + processed/
package main

import (
	"bufio"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"
)

// ============================================================
// JSON-RPC bridge (from Spike 1)
// ============================================================

type Request struct {
	JSONRPC string `json:"jsonrpc"`
	Method  string `json:"method"`
	Params  any    `json:"params,omitempty"`
	ID      any    `json:"id,omitempty"`
}

type Response struct {
	JSONRPC string    `json:"jsonrpc"`
	Result  any       `json:"result"`
	Error   *RPCError `json:"error,omitempty"`
	ID      any       `json:"id"`
}

type RPCError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
	Data    any    `json:"data,omitempty"`
}

type rawMessage struct {
	JSONRPC string          `json:"jsonrpc"`
	Method  string          `json:"method,omitempty"`
	Params  json.RawMessage `json:"params,omitempty"`
	Result  json.RawMessage `json:"result,omitempty"`
	Error   *RPCError       `json:"error,omitempty"`
	ID      any             `json:"id,omitempty"`
}

type PrimitiveParams struct {
	Args   []any          `json:"args,omitempty"`
	Kwargs map[string]any `json:"kwargs,omitempty"`
}

type PrimitiveHandler func(args []any, kwargs map[string]any) (any, error)

type Bridge struct {
	cmd      *exec.Cmd
	stdin    io.WriteCloser
	reader   *bufio.Reader
	mu       sync.Mutex
	nextID   int
	pending  map[int]chan *Response
	handlers map[string]PrimitiveHandler
}

func NewBridge(bridgePath string, workDir string) (*Bridge, error) {
	cmd := exec.Command("uv", "run", "python3", bridgePath)
	cmd.Dir = filepath.Dir(bridgePath) // bridge needs its venv
	cmd.Stderr = os.Stderr

	stdin, err := cmd.StdinPipe()
	if err != nil {
		return nil, fmt.Errorf("stdin pipe: %w", err)
	}
	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, fmt.Errorf("stdout pipe: %w", err)
	}
	if err := cmd.Start(); err != nil {
		return nil, fmt.Errorf("start bridge: %w", err)
	}

	b := &Bridge{
		cmd:      cmd,
		stdin:    stdin,
		reader:   bufio.NewReader(stdout),
		pending:  make(map[int]chan *Response),
		handlers: make(map[string]PrimitiveHandler),
	}
	go b.readLoop()
	return b, nil
}

func (b *Bridge) RegisterPrimitive(name string, handler PrimitiveHandler) {
	b.handlers[name] = handler
}

func (b *Bridge) send(msg any) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return fmt.Errorf("marshal: %w", err)
	}
	b.mu.Lock()
	_, err = fmt.Fprintf(b.stdin, "%s\n", data)
	b.mu.Unlock()
	return err
}

func (b *Bridge) readLoop() {
	for {
		line, err := b.reader.ReadString('\n')
		if err != nil {
			return
		}
		var msg rawMessage
		if err := json.Unmarshal([]byte(line), &msg); err != nil {
			continue
		}
		if msg.Method == "" && (msg.Result != nil || msg.Error != nil) {
			id := toInt(msg.ID)
			b.mu.Lock()
			ch, ok := b.pending[id]
			if ok {
				delete(b.pending, id)
			}
			b.mu.Unlock()
			if ok {
				resp := &Response{ID: msg.ID, Error: msg.Error}
				if msg.Result != nil {
					var result any
					_ = json.Unmarshal(msg.Result, &result)
					resp.Result = result
				}
				ch <- resp
			}
			continue
		}
		if msg.Method != "" {
			go b.handleCallback(msg)
		}
	}
}

func (b *Bridge) handleCallback(msg rawMessage) {
	var params PrimitiveParams
	if msg.Params != nil {
		_ = json.Unmarshal(msg.Params, &params)
	}
	handler, ok := b.handlers[msg.Method]
	if !ok {
		_ = b.send(Response{JSONRPC: "2.0", Error: &RPCError{Code: -32601, Message: "unknown: " + msg.Method}, ID: msg.ID})
		return
	}
	result, err := handler(params.Args, params.Kwargs)
	if err != nil {
		_ = b.send(Response{JSONRPC: "2.0", Error: &RPCError{Code: -32000, Message: err.Error()}, ID: msg.ID})
		return
	}
	_ = b.send(Response{JSONRPC: "2.0", Result: result, ID: msg.ID})
}

func (b *Bridge) RunScript(script string, externalFunctions []string) (any, error) {
	b.mu.Lock()
	b.nextID++
	id := b.nextID
	ch := make(chan *Response, 1)
	b.pending[id] = ch
	b.mu.Unlock()
	if err := b.send(Request{JSONRPC: "2.0", Method: "run", Params: map[string]any{"script": script, "external_functions": externalFunctions}, ID: id}); err != nil {
		return nil, err
	}
	select {
	case resp := <-ch:
		if resp.Error != nil {
			return nil, fmt.Errorf("%s", resp.Error.Message)
		}
		return resp.Result, nil
	case <-time.After(30 * time.Second):
		return nil, fmt.Errorf("script execution timed out after 30s")
	}
}

func (b *Bridge) Shutdown() error {
	_ = b.send(Request{JSONRPC: "2.0", Method: "shutdown"})
	return b.cmd.Wait()
}

func toInt(v any) int {
	switch n := v.(type) {
	case float64:
		return int(n)
	case int:
		return n
	}
	return 0
}

// ============================================================
// Real primitives
// ============================================================

type Runtime struct {
	repoDir      string
	entryCounter int
	agentLog     [][]string // rows for agent-log.csv
	queueItems   []map[string]any
}

func NewRuntime(repoDir string) *Runtime {
	return &Runtime{repoDir: repoDir}
}

func (rt *Runtime) Register(bridge *Bridge) {
	bridge.RegisterPrimitive("importer_scan", rt.importerScan)
	bridge.RegisterPrimitive("importer_parse", rt.importerParse)
	bridge.RegisterPrimitive("importer_mark_processed", rt.importerMarkProcessed)
	bridge.RegisterPrimitive("importer_deduplicate", rt.importerDeduplicate)
	bridge.RegisterPrimitive("journal_add_double", rt.journalAddDouble)
	bridge.RegisterPrimitive("journal_query", rt.journalQuery)
	bridge.RegisterPrimitive("rules_match", rt.rulesMatch)
	bridge.RegisterPrimitive("git_commit", rt.gitCommit)
	bridge.RegisterPrimitive("config_get", rt.configGet)
	bridge.RegisterPrimitive("ctx_log", rt.ctxLog)
	bridge.RegisterPrimitive("queue_add_review", rt.queueAddReview)
}

func (rt *Runtime) importerScan(_ []any, _ map[string]any) (any, error) {
	importDir := filepath.Join(rt.repoDir, "import")
	entries, err := os.ReadDir(importDir)
	if err != nil {
		return []any{}, nil
	}
	var files []map[string]any
	for _, e := range entries {
		if e.IsDir() || !strings.HasSuffix(e.Name(), ".csv") {
			continue
		}
		info, _ := e.Info()
		files = append(files, map[string]any{
			"name": e.Name(),
			"path": filepath.Join("import", e.Name()),
			"size": info.Size(),
		})
	}
	if files == nil {
		return []any{}, nil
	}
	return files, nil
}

func (rt *Runtime) importerParse(args []any, _ map[string]any) (any, error) {
	fileName, _ := args[0].(string)
	path := filepath.Join(rt.repoDir, "import", fileName)

	f, err := os.Open(path)
	if err != nil {
		return nil, fmt.Errorf("open %s: %w", fileName, err)
	}
	defer f.Close()

	reader := csv.NewReader(f)
	header, err := reader.Read()
	if err != nil {
		return nil, fmt.Errorf("read header: %w", err)
	}

	// Find column indices (Chase format)
	colIdx := map[string]int{}
	for i, h := range header {
		colIdx[strings.TrimSpace(h)] = i
	}

	var txns []map[string]any
	for {
		record, err := reader.Read()
		if err != nil {
			break
		}

		date := ""
		if idx, ok := colIdx["Posting Date"]; ok && idx < len(record) {
			date = parseChaseDate(record[idx])
		}

		desc := ""
		if idx, ok := colIdx["Description"]; ok && idx < len(record) {
			desc = strings.TrimSpace(record[idx])
		}

		amount := 0.0
		if idx, ok := colIdx["Amount"]; ok && idx < len(record) {
			amount, _ = strconv.ParseFloat(strings.TrimSpace(record[idx]), 64)
		}

		ref := fmt.Sprintf("chase_%s_%s", strings.ReplaceAll(date, "-", ""), strings.ReplaceAll(desc[:min(10, len(desc))], " ", ""))

		txns = append(txns, map[string]any{
			"date":         date,
			"description":  desc,
			"amount":       amount,
			"reference":    ref,
			"bank_account": "chase_checking",
		})
	}
	return txns, nil
}

func parseChaseDate(s string) string {
	// MM/DD/YYYY → YYYY-MM-DD
	parts := strings.Split(strings.TrimSpace(s), "/")
	if len(parts) != 3 {
		return s
	}
	return fmt.Sprintf("%s-%s-%s", parts[2], parts[0], parts[1])
}

func (rt *Runtime) importerMarkProcessed(args []any, _ map[string]any) (any, error) {
	fileName, _ := args[0].(string)
	src := filepath.Join(rt.repoDir, "import", fileName)
	dstDir := filepath.Join(rt.repoDir, "import", "processed")
	_ = os.MkdirAll(dstDir, 0o755)
	dst := filepath.Join(dstDir, fileName)
	if err := os.Rename(src, dst); err != nil {
		return nil, fmt.Errorf("move to processed: %w", err)
	}
	return map[string]any{"success": true}, nil
}

func (rt *Runtime) importerDeduplicate(args []any, _ map[string]any) (any, error) {
	// For the spike, just pass through — no existing journal to check against
	if len(args) > 0 {
		return args[0], nil
	}
	return []any{}, nil
}

func (rt *Runtime) journalAddDouble(_ []any, kwargs map[string]any) (any, error) {
	rt.entryCounter++
	entryID := fmt.Sprintf("2025-01-%03d", rt.entryCounter)

	date, _ := kwargs["date"].(string)
	desc, _ := kwargs["description"].(string)
	debitAcct := toInt(kwargs["debit_account"])
	creditAcct := toInt(kwargs["credit_account"])
	amount, _ := kwargs["amount"].(float64)
	counterparty, _ := kwargs["counterparty"].(string)
	reference, _ := kwargs["reference"].(string)
	confidence, _ := kwargs["confidence"].(float64)
	status, _ := kwargs["status"].(string)
	evidence, _ := kwargs["evidence"].(string)
	tags, _ := kwargs["tags"].(string)
	notes, _ := kwargs["notes"].(string)

	// Determine journal file path
	month := date[:7] // YYYY-MM
	parts := strings.Split(month, "-")
	journalDir := filepath.Join(rt.repoDir, parts[0], parts[1])
	_ = os.MkdirAll(journalDir, 0o755)
	journalPath := filepath.Join(journalDir, "journal.csv")

	// Write header if file doesn't exist
	if _, err := os.Stat(journalPath); os.IsNotExist(err) {
		header := "entry_id,date,account_id,description,debit,credit,counterparty,reference,confidence,status,evidence,receipt_hash,tags,notes\n"
		if err := os.WriteFile(journalPath, []byte(header), 0o644); err != nil {
			return nil, fmt.Errorf("write journal header: %w", err)
		}
	}

	// Append debit leg
	f, err := os.OpenFile(journalPath, os.O_APPEND|os.O_WRONLY, 0o644)
	if err != nil {
		return nil, fmt.Errorf("open journal: %w", err)
	}
	defer f.Close()

	w := csv.NewWriter(f)
	confStr := strconv.FormatFloat(confidence, 'f', 2, 64)
	amtStr := strconv.FormatFloat(amount, 'f', 2, 64)

	// Debit leg
	_ = w.Write([]string{
		entryID + "a", date, strconv.Itoa(debitAcct), desc,
		amtStr, "", counterparty, reference, confStr, status, evidence, "", tags, notes,
	})
	// Credit leg
	_ = w.Write([]string{
		entryID + "b", date, strconv.Itoa(creditAcct), desc,
		"", amtStr, counterparty, reference, confStr, status, evidence, "", tags, notes,
	})
	w.Flush()

	return map[string]any{"entry_id": entryID, "success": true}, nil
}

func (rt *Runtime) journalQuery(_ []any, _ map[string]any) (any, error) {
	return []any{}, nil
}

func (rt *Runtime) rulesMatch(_ []any, kwargs map[string]any) (any, error) {
	desc, _ := kwargs["description"].(string)
	upper := strings.ToUpper(desc)

	// Simple rule matching for the spike
	rules := []struct {
		pattern    string
		vendor     string
		accountID  int
		confidence float64
	}{
		{"GITHUB", "GitHub", 5020, 0.98},
		{"AWS", "Amazon Web Services", 5020, 0.96},
		{"DROPBOX", "Dropbox", 5020, 0.95},
	}

	for _, r := range rules {
		if strings.Contains(upper, r.pattern) {
			return map[string]any{
				"pattern":     r.pattern + "*",
				"vendor_name": r.vendor,
				"account_id":  r.accountID,
				"confidence":  r.confidence,
			}, nil
		}
	}
	return nil, nil
}

func (rt *Runtime) gitCommit(args []any, _ map[string]any) (any, error) {
	message, _ := args[0].(string)

	// Real git add + commit
	cmd := exec.Command("git", "add", "-A")
	cmd.Dir = rt.repoDir
	if out, err := cmd.CombinedOutput(); err != nil {
		return nil, fmt.Errorf("git add: %s: %w", out, err)
	}

	cmd = exec.Command("git", "commit", "-m", message, "--author", "Cleared Agent <agent@cleared.dev>")
	cmd.Dir = rt.repoDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return nil, fmt.Errorf("git commit: %s: %w", out, err)
	}

	// Get commit hash
	cmd = exec.Command("git", "rev-parse", "--short", "HEAD")
	cmd.Dir = rt.repoDir
	hashOut, _ := cmd.Output()
	hash := strings.TrimSpace(string(hashOut))

	return map[string]any{"commit_hash": hash, "success": true}, nil
}

func (rt *Runtime) configGet(args []any, _ map[string]any) (any, error) {
	key, _ := args[0].(string)
	configs := map[string]any{
		"business.name":           "Test Corp",
		"business.entity_type":    "llc_single_member",
		"thresholds.auto_confirm": 0.95,
		"thresholds.review":       0.70,
	}
	return configs[key], nil
}

func (rt *Runtime) ctxLog(args []any, _ map[string]any) (any, error) {
	message, _ := args[0].(string)
	rt.agentLog = append(rt.agentLog, []string{
		time.Now().UTC().Format(time.RFC3339),
		"ingest",
		"log",
		message,
		"",
		"",
	})
	fmt.Printf("  [agent] %s\n", message)
	return nil, nil
}

func (rt *Runtime) queueAddReview(_ []any, kwargs map[string]any) (any, error) {
	rt.queueItems = append(rt.queueItems, kwargs)
	return map[string]any{"item_id": fmt.Sprintf("q%03d", len(rt.queueItems)), "success": true}, nil
}

func (rt *Runtime) WriteAgentLog() error {
	logDir := filepath.Join(rt.repoDir, "logs")
	_ = os.MkdirAll(logDir, 0o755)
	logPath := filepath.Join(logDir, "agent-log.csv")

	f, err := os.Create(logPath)
	if err != nil {
		return err
	}
	defer f.Close()

	w := csv.NewWriter(f)
	_ = w.Write([]string{"timestamp", "agent", "action", "details", "entry_id", "commit_hash"})
	for _, row := range rt.agentLog {
		_ = w.Write(row)
	}
	w.Flush()
	return nil
}

// ============================================================
// Init — create repo structure
// ============================================================

func initRepo(dir string) error {
	dirs := []string{
		"accounts",
		"rules",
		"agents",
		"scripts",
		"templates",
		"tests",
		"logs",
		"import",
		"import/processed",
	}
	for _, d := range dirs {
		if err := os.MkdirAll(filepath.Join(dir, d), 0o755); err != nil {
			return err
		}
	}

	// cleared.yaml
	config := `business:
  name: "Test Corp"
  entity_type: "llc_single_member"
  tax_year_end: "12-31"
  currency: "USD"

thresholds:
  auto_confirm: 0.95
  high_confidence: 0.90
  review: 0.70

git:
  author_name: "Cleared Agent"
  author_email: "agent@cleared.dev"
`
	if err := os.WriteFile(filepath.Join(dir, "cleared.yaml"), []byte(config), 0o644); err != nil {
		return err
	}

	// chart-of-accounts.csv
	coa := `account_id,account_name,account_type,parent_id,tax_line,description
1010,Business Checking,asset,,,"Primary checking account"
1020,Business Savings,asset,,,"Savings account"
2010,Credit Card,liability,,,"Business credit card"
3010,Owner's Equity,equity,,,"Owner's equity"
4010,Service Revenue,revenue,,,""
4020,Product Revenue,revenue,,,""
5010,Advertising & Marketing,expense,,schedule_c_8,"Advertising costs"
5020,Software & SaaS,expense,,schedule_c_18,"Software subscriptions"
5030,Office Supplies,expense,,schedule_c_18,"Office supplies and expenses"
5040,Professional Services,expense,,schedule_c_17,"Legal, accounting, consulting"
5050,Shipping & Postage,expense,,schedule_c_18,"Postage and shipping costs"
`
	if err := os.WriteFile(filepath.Join(dir, "accounts", "chart-of-accounts.csv"), []byte(coa), 0o644); err != nil {
		return err
	}

	// .gitignore
	gitignore := `receipts/
exports/
queue/
.cleared-cache/
`
	if err := os.WriteFile(filepath.Join(dir, ".gitignore"), []byte(gitignore), 0o644); err != nil {
		return err
	}

	// categorization-rules.yaml (seed rules)
	rules := `rules:
  - vendor_pattern: "GITHUB*"
    vendor_name: "GitHub"
    account_id: 5020
    confidence: 0.98
    source: "seed"
  - vendor_pattern: "AWS*"
    vendor_name: "Amazon Web Services"
    account_id: 5020
    confidence: 0.96
    source: "seed"
  - vendor_pattern: "DROPBOX*"
    vendor_name: "Dropbox"
    account_id: 5020
    confidence: 0.95
    source: "seed"
`
	if err := os.WriteFile(filepath.Join(dir, "rules", "categorization-rules.yaml"), []byte(rules), 0o644); err != nil {
		return err
	}

	// git init + initial commit
	run := func(args ...string) error {
		cmd := exec.Command(args[0], args[1:]...)
		cmd.Dir = dir
		out, err := cmd.CombinedOutput()
		if err != nil {
			return fmt.Errorf("%s: %s: %w", args, out, err)
		}
		return nil
	}

	if err := run("git", "init"); err != nil {
		return err
	}
	if err := run("git", "add", "-A"); err != nil {
		return err
	}
	if err := run("git", "commit", "-m", "init: Initialize cleared repository for Test Corp",
		"--author", "Cleared Agent <agent@cleared.dev>"); err != nil {
		return err
	}

	return nil
}

// ============================================================
// Verification
// ============================================================

func verify(repoDir string) bool {
	allPassed := true

	check := func(name string, ok bool, detail string) {
		status := "✓"
		if !ok {
			status = "✗"
			allPassed = false
		}
		fmt.Printf("  [%s] %s", status, name)
		if detail != "" {
			fmt.Printf(" — %s", detail)
		}
		fmt.Println()
	}

	// 1. Journal exists with correct format
	journalPath := filepath.Join(repoDir, "2025", "01", "journal.csv")
	journalData, err := os.ReadFile(journalPath)
	journalExists := err == nil && len(journalData) > 0
	check("Journal CSV exists", journalExists, journalPath)

	if journalExists {
		// Count entries
		lines := strings.Split(strings.TrimSpace(string(journalData)), "\n")
		dataLines := len(lines) - 1 // minus header
		check("Journal has entries", dataLines > 0, fmt.Sprintf("%d rows (header + %d legs)", len(lines), dataLines))

		// Check header
		header := lines[0]
		hasCorrectHeader := strings.HasPrefix(header, "entry_id,date,account_id")
		check("Correct CSV header", hasCorrectHeader, "")

		// Validate invariants: for each entry group, debits == credits
		reader := csv.NewReader(strings.NewReader(string(journalData)))
		records, _ := reader.ReadAll()
		groups := map[string]struct{ debit, credit float64 }{}
		for _, r := range records[1:] {
			// entry_id like "2025-01-001a" → group "2025-01-001"
			eid := r[0]
			group := eid[:len(eid)-1]
			d, _ := strconv.ParseFloat(r[4], 64)
			c, _ := strconv.ParseFloat(r[5], 64)
			g := groups[group]
			g.debit += d
			g.credit += c
			groups[group] = g
		}
		balanced := true
		for gid, g := range groups {
			if g.debit-g.credit > 0.001 || g.credit-g.debit > 0.001 {
				fmt.Printf("    UNBALANCED: %s debit=%.2f credit=%.2f\n", gid, g.debit, g.credit)
				balanced = false
			}
		}
		check("All entries balanced (debits=credits)", balanced, fmt.Sprintf("%d entry groups", len(groups)))

		// Check valid account IDs
		validAccounts := map[string]bool{
			"1010": true, "1020": true, "2010": true, "3010": true,
			"4010": true, "4020": true, "5010": true, "5020": true,
			"5030": true, "5040": true, "5050": true,
		}
		allValid := true
		for _, r := range records[1:] {
			if !validAccounts[r[2]] {
				fmt.Printf("    INVALID ACCOUNT: %s in entry %s\n", r[2], r[0])
				allValid = false
			}
		}
		check("All account IDs valid", allValid, "")

		// Check dates within month
		allDatesOK := true
		for _, r := range records[1:] {
			if !strings.HasPrefix(r[1], "2025-01") {
				allDatesOK = false
			}
		}
		check("All dates within month", allDatesOK, "")

		// Check unique entry IDs
		ids := map[string]bool{}
		uniqueIDs := true
		for _, r := range records[1:] {
			if ids[r[0]] {
				uniqueIDs = false
			}
			ids[r[0]] = true
		}
		check("Unique entry IDs", uniqueIDs, fmt.Sprintf("%d unique IDs", len(ids)))
	}

	// 2. Git commit with correct prefix
	cmd := exec.Command("git", "log", "--oneline", "-5")
	cmd.Dir = repoDir
	gitOut, _ := cmd.Output()
	gitLog := string(gitOut)
	hasImportCommit := strings.Contains(gitLog, "import:")
	hasInitCommit := strings.Contains(gitLog, "init:")
	check("Git commit with 'import:' prefix", hasImportCommit, strings.TrimSpace(gitLog))
	check("Git commit with 'init:' prefix", hasInitCommit, "")

	// 3. Agent log exists
	logPath := filepath.Join(repoDir, "logs", "agent-log.csv")
	logData, err := os.ReadFile(logPath)
	check("Agent log exists", err == nil && len(logData) > 0, logPath)

	// 4. CSV moved to processed
	processedFiles, _ := os.ReadDir(filepath.Join(repoDir, "import", "processed"))
	check("CSV moved to processed/", len(processedFiles) > 0,
		fmt.Sprintf("%d files in processed/", len(processedFiles)))

	// 5. No CSVs left in import/ root
	importFiles, _ := os.ReadDir(filepath.Join(repoDir, "import"))
	csvCount := 0
	for _, f := range importFiles {
		if strings.HasSuffix(f.Name(), ".csv") {
			csvCount++
		}
	}
	check("No CSVs left in import/", csvCount == 0, "")

	return allPassed
}

// ============================================================
// Main
// ============================================================

func main() {
	fmt.Println("=== Spike 4: End-to-End Loop ===")
	fmt.Println()

	// Use source file location to find sibling directories
	_, thisFile, _, _ := runtime.Caller(0)
	spikeDir := filepath.Dir(thisFile)
	bridgePath := filepath.Join(spikeDir, "..", "spike1", "bridge.py")

	// Create a temp directory for the test repo
	repoDir, err := os.MkdirTemp("", "cleared-spike4-*")
	if err != nil {
		fmt.Printf("FAILED: create temp dir: %v\n", err)
		os.Exit(1)
	}
	defer os.RemoveAll(repoDir)
	fmt.Printf("Test repo: %s\n", repoDir)

	totalStart := time.Now()

	// Step 1: Init
	fmt.Println("\n--- Step 1: cleared init ---")
	initStart := time.Now()
	if err := initRepo(repoDir); err != nil {
		fmt.Printf("FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Init complete (%dms)\n", time.Since(initStart).Milliseconds())

	// Step 2: Copy CSV to import/
	fmt.Println("\n--- Step 2: Copy CSV to import/ ---")
	csvSrc := filepath.Join(spikeDir, "testdata", "chase_checking.csv")
	csvDst := filepath.Join(repoDir, "import", "chase_checking.csv")
	csvData, _ := os.ReadFile(csvSrc)
	_ = os.WriteFile(csvDst, csvData, 0o644)
	fmt.Printf("  Copied %d bytes\n", len(csvData))

	// Step 3: Run agent
	fmt.Println("\n--- Step 3: cleared agent run ---")
	agentScript, _ := os.ReadFile(filepath.Join(spikeDir, "testdata", "ingest.py"))

	bridge, err := NewBridge(bridgePath, repoDir)
	if err != nil {
		fmt.Printf("FAILED: start bridge: %v\n", err)
		os.Exit(1)
	}

	rt := NewRuntime(repoDir)
	rt.Register(bridge)

	allPrimitives := []string{
		"importer_scan", "importer_parse", "importer_mark_processed", "importer_deduplicate",
		"journal_add_double", "journal_query", "rules_match", "git_commit",
		"config_get", "ctx_log", "queue_add_review",
	}

	agentStart := time.Now()
	result, err := bridge.RunScript(string(agentScript), allPrimitives)
	agentElapsed := time.Since(agentStart)

	if err != nil {
		fmt.Printf("  FAILED: %v\n", err)
		os.Exit(1)
	}
	fmt.Printf("  Agent completed in %dms\n", agentElapsed.Milliseconds())
	fmt.Printf("  Output: %v\n", result)

	// Write agent log
	if err := rt.WriteAgentLog(); err != nil {
		fmt.Printf("  Warning: failed to write agent log: %v\n", err)
	}

	_ = bridge.Shutdown()

	// Step 4: Verify
	fmt.Println("\n--- Step 4: Verify ---")
	totalElapsed := time.Since(totalStart)
	allPassed := verify(repoDir)

	fmt.Printf("\n  Total time: %dms\n", totalElapsed.Milliseconds())
	underLimit := totalElapsed.Seconds() < 10
	if underLimit {
		fmt.Println("  [✓] Full cycle < 10 seconds")
	} else {
		fmt.Println("  [✗] Full cycle > 10 seconds")
	}

	fmt.Println("\n=== Spike 4 Results ===")
	if allPassed && underLimit {
		fmt.Println("  ALL CRITERIA MET")
	} else {
		fmt.Println("  SOME CRITERIA FAILED")
		os.Exit(1)
	}
}
