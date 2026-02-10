# Cleared — Architecture

## System Overview

```
┌──────────────────────────────────────────────────────────┐
│  USER INTERFACE                                            │
│                                                            │
│  Swipe UI (mobile web)     Chat (natural language)         │
│  • Approve/correct txns    • "Batch my notifications"      │
│  • Review agent changes    • "How much did I spend on X?"  │
│  • View status/reports     • "Add a rule for Amazon"       │
└────────────┬──────────────────────┬──────────────────────┘
             │                      │
             ▼                      ▼
┌──────────────────────────────────────────────────────────┐
│  LAYER 3: Meta-Agent (LLM)                                │
│                                                            │
│  Interprets natural language → reads current agents →      │
│  writes/modifies Python scripts → validates → proposes     │
│  changes as swipe cards → user approves → git commit       │
│                                                            │
│  NOT in the execution path. Only for customization,        │
│  edge-case classification, and insight generation.         │
└────────────┬─────────────────────────────────────────────┘
             │ modifies
             ▼
┌──────────────────────────────────────────────────────────┐
│  LAYER 2: Agents (Python scripts in git repo)              │
│                                                            │
│  agents/                                                   │
│    ingest.py          — import bank data, classify, route  │
│    digest.py          — morning email summary              │
│    learning.py        — analyze corrections, improve rules │
│    optimizer.py       — consolidate rules, tune workflows  │
│    tester.py          — generate edge-case tests           │
│    reconcile.py       — check balances against bank        │
│    [user-created].py  — whatever the user asks for         │
│                                                            │
│  Each agent: metadata (trigger, schedule) + top-level script│
│  Pure code. LLM writes them, but they execute without LLM. │
└────────────┬─────────────────────────────────────────────┘
             │ calls primitives
             ▼
┌──────────────────────────────────────────────────────────┐
│  LAYER 1: Runtime (Go binary, bundled with Monty)          │
│                                                            │
│  ┌─────────────┐  ┌─────────────┐  ┌──────────────────┐  │
│  │  Scheduler   │  │  Sandbox    │  │  Web Server      │  │
│  │  cron +      │  │  Monty      │  │  swipe UI +      │  │
│  │  file watch  │  │  subprocess │  │  chat + API      │  │
│  └──────┬──────┘  └──────┬──────┘  └────────┬─────────┘  │
│         │                │                   │             │
│         ▼                ▼                   ▼             │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  Primitives (Go, exposed as flat Python functions)    │  │
│  │                                                       │  │
│  │  journal_add_double, journal_query ...                 │  │
│  │  accounts_list, accounts_get, accounts_exists ...     │  │
│  │  importer_scan, importer_parse, importer_mark_...     │  │
│  │  git_commit                                           │  │
│  │  queue_add_review                                     │  │
│  │  config_get, ctx_log, ctx_dry_run                     │  │
│  │  llm_classify, llm_summarize (future)                 │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                            │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  Validation Layer (the "constitution")                │  │
│  │                                                       │  │
│  │  • 6 double-entry invariants (always enforced)        │  │
│  │  • Monty type_check (future: arg types, params)       │  │
│  │  • Dry run (future: synthetic data execution)         │  │
│  │  • Behavioral diff (future: compare before/after)     │  │
│  └─────────────────────────────────────────────────────┘  │
└────────────┬─────────────────────────────────────────────┘
             │ persists to
             ▼
┌──────────────────────────────────────────────────────────┐
│  DATA: Git Repository (system of record)                   │
│                                                            │
│  cleared.yaml              — business config + schedules   │
│  accounts/coa.csv          — chart of accounts             │
│  rules/                    — agent-managed categorization   │
│  agents/*.py               — agent scripts (version ctrl)  │
│  scripts/*.py              — shared utility scripts        │
│  templates/*.html          — email/report templates        │
│  tests/*.py                — agent-generated tests         │
│  logs/agent-log.csv        — append-only agent actions     │
│  YYYY/MM/journal.csv       — the ledger                    │
│  YYYY/MM/reconciliation.csv                                │
│                                                            │
│  GITIGNORED: receipts/, exports/, queue/, .cleared-cache/  │
└──────────────────────────────────────────────────────────┘
```

## Go Binary — Project Structure

```
cleared/
├── cmd/cleared/
│   └── main.go                          # Entrypoint (sx pattern)
├── internal/
│   ├── buildinfo/info.go                # Version/Commit/Date (ldflags)
│   ├── config/config.go                 # cleared.yaml load/save/defaults
│   ├── model/                           # Domain models
│   │   ├── account.go                   # Account, AccountType
│   │   ├── journal.go                   # JournalEntry, Leg, EntryStatus
│   │   └── transaction.go              # BankTransaction
│   ├── journal/                         # Journal service
│   │   ├── service.go                   # Add, List, Import, Validate+Write
│   │   ├── validate.go                 # 6 invariants
│   │   └── csv.go                       # CSV read/write/marshal
│   ├── accounts/                        # Chart of accounts
│   │   ├── accounts.go                 # Service
│   │   ├── csv.go                       # CSV read/write
│   │   └── defaults.go                 # Default chart per entity type
│   ├── importer/                        # Bank CSV parsers
│   │   ├── importer.go                 # BankImporter interface + registry
│   │   └── chase.go                    # Chase parser
│   ├── gitops/gitops.go                # Git operations (exec.Command)
│   ├── sandbox/                         # Python execution
│   │   ├── bridge.py                  # Monty JSON-RPC bridge (embedded)
│   │   ├── bridge.go                  # Bridge subprocess + JSON-RPC
│   │   └── primitives.go              # Runtime + Go→Python primitive bindings
│   ├── commands/                        # Cobra CLI (sx pattern)
│   │   ├── root.go
│   │   ├── init.go                    # cleared init
│   │   └── agent.go                   # cleared agent run
│   └── id/id.go                        # Entry ID generation
├── testdata/
├── Makefile                             # From sx patterns
├── .golangci.yml                        # From sx
├── .goreleaser.yml                      # From sx
├── .github/workflows/
│   ├── test.yml
│   └── release.yml
├── go.mod                               # Go 1.25
└── go.sum
```

## Sandbox Interface

Currently implemented as `Bridge` directly (ScriptRunner interface deferred until Validate/DryRun are wired in):

```go
// internal/sandbox/bridge.go

// Bridge manages the Python bridge subprocess and JSON-RPC communication.
// bridge.py is embedded via //go:embed and written to a temp dir at runtime.
type Bridge struct { ... }

func NewBridge() (*Bridge, error)
func (b *Bridge) RegisterPrimitive(name string, handler PrimitiveHandler)
func (b *Bridge) RunScript(script string, externals []string) (any, error)  // 30s timeout
func (b *Bridge) Shutdown() error

// PrimitiveHandler is a Go function exposed to Python scripts.
// Primitives are flat functions (journal_add_double, not journal.add).
type PrimitiveHandler func(args []any, kwargs map[string]any) (any, error)
```

Future: `ScriptRunner` interface wrapping Bridge with `Validate()` and `DryRun()` methods (spike3 proved the approach).

## Go ↔ Python Communication

Go and the Python bridge communicate via **JSON-RPC 2.0** over stdin/stdout of a persistent subprocess. Go is both client (sends `run` requests) and server (handles primitive callbacks). The bridge is the reverse — server for `run`, client for primitive calls.

```
Go → Bridge:  {"jsonrpc":"2.0","method":"run","params":{"script":"...","external_functions":["journal_query","journal_add_double"]},"id":1}
Bridge → Go:  {"jsonrpc":"2.0","method":"journal_query","params":{"kwargs":{"status":"pending"}},"id":100}
Go → Bridge:  {"jsonrpc":"2.0","result":[{"id":"001","confidence":0.97}],"id":100}
Bridge → Go:  {"jsonrpc":"2.0","method":"journal_add_double","params":{"kwargs":{"date":"2025-01-03",...}},"id":101}
Go → Bridge:  {"jsonrpc":"2.0","result":{"success":true},"id":101}
Bridge → Go:  {"jsonrpc":"2.0","result":{"confirmed":1,"review":1},"id":1}
```

ID correlation enables pipelining — multiple scripts can run concurrently over a single bridge subprocess. Monty's external function feature pauses execution on primitive calls, the bridge sends a JSON-RPC request to Go, Go executes, returns the result, and Monty resumes.

**Python dependency:** The bridge requires Python + `pydantic-monty`, managed via `uv`. Future path to single binary: compile Monty to WASM, run via wazero (pure Go).

## Dependencies

| Library | Purpose |
|---------|---------|
| `github.com/spf13/cobra` | CLI framework |
| `github.com/shopspring/decimal` | Exact decimal arithmetic |
| `gopkg.in/yaml.v3` | Config YAML |
| `encoding/csv` (stdlib) | CSV read/write |
| `pydantic-monty` (via uv) | Python sandbox runtime |
| `uv` | Python environment + dependency management |

Future: `excelize`, `chi`, `robfig/cron`, `fsnotify`, Anthropic/OpenAI Go SDKs, wazero (Monty→WASM)

## Build & CI

Adapted from `/home/mrdon/dev/sx/`:

**Makefile**: `build` (CGO_ENABLED=0, ldflags), `install`, `test` (-race -cover), `lint` (golangci-lint v2.8.0), `format`, `prepush`, `clean`

**CI**: `test.yml` (push/PR → fmt check → lint → vet → test → build), `release.yml` (tag → test gate → goreleaser, 6 platform builds)

**.golangci.yml**: Same linter set as sx (staticcheck, errorlint, nilnil, bodyclose, exhaustive, misspell, unused, modernize, etc.)

**.goreleaser.yml**: CGO_ENABLED=0, linux/darwin/windows × amd64/arm64 (requires uv + Python on target for now; future: WASM bundle)
