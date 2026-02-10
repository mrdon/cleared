# Cleared

Agentic small business accounting. Agents do the bookkeeping autonomously — the human spends less than 5 minutes a day.

## How It Works

Three layers:

| Layer | What | Technology |
|-------|------|------------|
| **Runtime** | Primitives, sandbox, validation, CLI | Go binary |
| **Agents** | Business logic, workflows, improvement loops | Python scripts (Monty sandbox) |
| **Data** | Accounting records, rules, config | CSV/YAML in git |

Agents are top-level Python scripts that call Go primitives as flat global functions (`journal_add_double`, `importer_scan`, etc.). They run in a [Monty](https://github.com/pydantic/pydantic-monty) sandbox — no filesystem, no network, no imports. The Go runtime enforces 6 double-entry accounting invariants on every write. Every data change is a git commit.

## Quick Start

### Prerequisites

- Go 1.25+
- [uv](https://docs.astral.sh/uv/) (Python package manager)
- git

### Install

```bash
go install github.com/cleared-dev/cleared/cmd/cleared@latest
```

Or build from source:

```bash
make build        # ./dist/cleared
make install      # ~/.local/bin/cleared
```

### Initialize a Project

```bash
cleared init my-business --name "My Business LLC"
```

This creates a git repo with chart of accounts, config, and directory structure.

### Run an Agent

```bash
# Copy bank CSV to import/
cp chase_checking.csv my-business/import/

# Copy or create an agent script
cp agents/ingest.py my-business/agents/

# Run the agent
cleared agent run ingest --repo my-business
```

The agent parses transactions, creates journal entries, moves processed files, and commits to git.

## Architecture

```
Go Runtime (primitives + validation)
    ↕ JSON-RPC 2.0 over stdio
Python Bridge (Monty sandbox)
    ↕ external function calls
Agent Scripts (flat primitives, no imports)
    ↕ reads/writes
Git Repository (CSV data, YAML config)
```

Agents call primitives like `journal_add_double(date=..., debit_account=5020, credit_account=1010, amount=4.00)`. The Go runtime validates every entry against 6 invariants (balanced debits/credits, valid accounts, sequential IDs, dates within month, exact decimals, unique IDs) before writing.

## Project Structure

```
my-business/
├── cleared.yaml                    # Business config
├── accounts/chart-of-accounts.csv  # Chart of accounts
├── rules/categorization-rules.yaml # Vendor categorization rules
├── agents/*.py                     # Agent scripts
├── logs/agent-log.csv              # Agent execution log
├── import/                         # Drop bank CSVs here
│   └── processed/                  # Processed files moved here
└── YYYY/MM/journal.csv             # Monthly double-entry ledger
```

## Development

```bash
make test      # Run tests
make lint      # Run linters
make prepush   # Format + lint + test + build
```

## Roadmap

See [docs/plan.md](docs/plan.md) for the full roadmap. Current status: Foundation phase complete (services, CLI, agent runtime).

## License

Apache License 2.0 — see [LICENSE](LICENSE).
