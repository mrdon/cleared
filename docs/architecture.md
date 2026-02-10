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
│  │  journal_add_double, journal_query, journal_void ...  │  │
│  │  accounts_list, accounts_get, accounts_exists ...     │  │
│  │  importer_scan, importer_parse, importer_mark_...     │  │
│  │  rules_match, rules_add, rules_update, rules_list     │  │
│  │  git_commit, git_log                                  │  │
│  │  queue_add_review, queue_pending                      │  │
│  │  config_get                                           │  │
│  │  ctx_log, ctx_emit                                    │  │
│  │  llm_classify, llm_summarize (optional, future)       │  │
│  └─────────────────────────────────────────────────────┘  │
│                                                            │
│  ┌─────────────────────────────────────────────────────┐  │
│  │  Validation Layer (the "constitution")                │  │
│  │                                                       │  │
│  │  • 6 double-entry invariants (always enforced)        │  │
│  │  • Monty type_check (arg types, missing params)        │  │
│  │  • Dry run (execute against synthetic data)           │  │
│  │  • Behavioral diff (compare before/after)             │  │
│  │  • Resource limits (time, memory, tx count)           │  │
│  │  • Auto-revert on anomaly detection                   │  │
│  └─────────────────────────────────────────────────────┘  │
└────────────┬─────────────────────────────────────────────┘
             │ persists to
             ▼
┌──────────────────────────────────────────────────────────┐
│  DATA: Git Repository (system of record)                   │
│                                                            │
│  cleared.yaml              — business config + schedules   │
│  accounts/coa.csv          — chart of accounts             │
│  rules/categorization.yaml — learned vendor→category rules │
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
│   ├── gitops/gitops.go                # Git operations (go-git)
│   ├── sandbox/                         # Python execution
│   │   ├── sandbox.go                  # ScriptRunner interface
│   │   ├── monty.go                    # Monty subprocess implementation
│   │   ├── primitives.go              # Go→Python primitive bindings
│   │   └── validate.go                # Script validation/linting
│   ├── scheduler/scheduler.go          # Cron + file watch + event triggers
│   ├── commands/                        # Cobra CLI (sx pattern)
│   │   ├── root.go
│   │   ├── init_cmd.go                 # cleared init
│   │   ├── agent_cmd.go               # cleared agent {run|start|status|log}
│   │   ├── import_cmd.go              # cleared import (manual override)
│   │   ├── add_cmd.go                 # cleared add (manual entry)
│   │   ├── list_cmd.go                # cleared list
│   │   └── status_cmd.go              # cleared status
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

```go
// internal/sandbox/sandbox.go

// ScriptRunner executes Python agent scripts in a sandbox.
// Implementation-agnostic — swap Monty for alternatives without changing callers.
type ScriptRunner interface {
    // Run executes a script. Primitives are registered separately;
    // externalFunctions lists which ones this script is allowed to call.
    Run(ctx context.Context, script string, externalFunctions []string) (*Result, error)

    // Validate runs Monty's type_check against primitive type stubs.
    // Also checks for forbidden constructs (open, eval, exec, __import__).
    Validate(script string) []ValidationIssue

    // DryRun executes against synthetic data with recording primitives.
    DryRun(ctx context.Context, script string) (*DryRunResult, error)
}

// PrimitiveHandler is a Go function exposed to Python scripts.
// Primitives are flat functions (journal_add_double, not journal.add).
type PrimitiveHandler func(args []any, kwargs map[string]any) (any, error)
```

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
| `github.com/go-git/go-git/v5` | Git operations (pure Go) |
| `github.com/shopspring/decimal` | Exact decimal arithmetic |
| `gopkg.in/yaml.v3` | Config YAML |
| `github.com/robfig/cron/v3` | Cron scheduler |
| `github.com/fsnotify/fsnotify` | File watch for import/ dir |
| `encoding/csv` (stdlib) | CSV read/write |
| `pydantic-monty` (via uv) | Python sandbox runtime |
| `uv` | Python environment + dependency management |

Future: `excelize`, `chi`, Anthropic/OpenAI Go SDKs, wazero (Monty→WASM)

## Build & CI

Adapted from `/home/mrdon/dev/sx/`:

**Makefile**: `build` (CGO_ENABLED=0, ldflags), `install`, `test` (-race -cover), `lint` (golangci-lint v2.8.0), `format`, `prepush`, `clean`

**CI**: `test.yml` (push/PR → fmt check → lint → vet → test → build), `release.yml` (tag → test gate → goreleaser, 6 platform builds)

**.golangci.yml**: Same linter set as sx (staticcheck, errorlint, nilnil, bodyclose, exhaustive, misspell, unused, modernize, etc.)

**.goreleaser.yml**: CGO_ENABLED=0, linux/darwin/windows × amd64/arm64 (requires uv + Python on target for now; future: WASM bundle)
