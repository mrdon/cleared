# Cleared

Agentic small business accounting. Agents do the bookkeeping autonomously — the human spends less than 5 minutes a day.

## Why Cleared?

- **Self-improving.** Nightly agents analyze patterns, rewrite their own logic, optimize workflows, and generate regression tests. A test ratchet prevents regressions — the system can only get better.
- **You manage, LLM develops.** Swipe to correct a transaction and the system learns. Customize via natural language ("batch my notifications") and the LLM rewrites agents for you.
- **Git is the audit trail.** Data, logic, rules, and tests all live in one repo. Every change is a commit — fully reviewable, fully reversible.
- **Deterministic by default.** Daily workflows are pure code — no LLM in the loop. The LLM is only used for learning, adapting, and edge cases.
- **Sandboxed.** Agents run in a Monty sandbox. The Go runtime enforces accounting invariants on every write. Agents evolve freely; the books stay correct.

## How It Works

| Layer | What | Technology |
|-------|------|------------|
| **UI** | Swipe to approve/correct, chat to customize | Mobile web |
| **Healing** | Nightly agents that learn from corrections, rewrite logic, generate tests | Agents + LLM |
| **Workflow** | Deterministic agents — ingest, classify, route, commit | Python (Monty sandbox) |
| **Runtime** | Primitives, invariant enforcement, sandbox | Go binary |
| **Data** | Ledger, agent scripts, tests — all version controlled | CSV/YAML in git |

Workflow agents handle the daily bookkeeping — deterministic code that processes bank data, classifies transactions, and routes by confidence. When they hit unknowns, they call LLM primitives for classification. Healing agents run nightly — they analyze corrections, rewrite workflow logic, optimize rules, and generate regression tests. Both are sandboxed Python scripts, but they serve different purposes: one does the work, the other improves it.

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
Go Runtime (primitives + 6 invariants)
    ↕ JSON-RPC 2.0 over stdio
Python Bridge (Monty sandbox)
    ↕ external function calls
Agent Scripts (flat primitives, no imports)
    ↕ reads/writes/rewrites
Git Repository (data + logic + rules + tests)
```

The Go runtime is the constitution — it enforces invariants that no agent can bypass: balanced debits/credits, valid account references, sequential IDs, dates within month, exact decimals, and unique entries. Agents can rewrite themselves, create new rules, and generate tests, but the books always balance.

## Project Structure

Everything is in git — data, logic, rules, and tests:

```
my-business/
├── cleared.yaml                    # Business config
├── accounts/chart-of-accounts.csv  # Chart of accounts
├── agents/*.py                     # Agent scripts — logic + rules live here, LLM-maintained
├── tests/*.py                      # Agent-generated regression tests (ratchet)
├── scripts/*.py                    # Shared sub-scripts extracted by learning agents
├── logs/agent-log.csv              # Append-only agent execution log
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
