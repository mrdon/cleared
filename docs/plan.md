# Cleared v0.2 — Master Plan

## What is Cleared?

A fully agentic small business accounting system. Agents do the bookkeeping autonomously. The human approves decisions via a swipe UI and customizes the system through natural language conversation. The system continuously improves itself.

**Core principles:**
- **Agent-first**: The human spends <5 min/day. Agents handle 95%, swipe UI handles 5%.
- **LLM for customization, not execution**: Daily workflows are deterministic code. LLM is for understanding user intent, writing/modifying agents, classifying edge cases, and generating insights.
- **Self-modifying**: The system writes its own agent scripts, rules, and tests. Users direct changes in natural language. The LLM is the developer; the user is the product manager.
- **Self-improving**: Nightly agents analyze patterns, optimize workflows, write tests, and propose improvements.
- **Git is the audit trail**: Every data change and every agent modification is a git commit. Fully reversible.
- **Invariants are the constitution**: No matter how much agents modify themselves, the Go runtime enforces accounting rules (debits=credits, valid accounts, etc.)

## Architecture

Three layers — see [Architecture](./architecture.md):

| Layer | What | Changes how? |
|-------|------|-------------|
| **Runtime** (Go binary) | Primitives, sandbox, scheduler, web UI, validation | Compiled releases only |
| **Agents** (Python scripts) | Business logic, workflows, improvement loops | LLM writes/modifies, user approves |
| **Data** (CSV/YAML in git) | Accounting records, rules, config | Agents write, git tracks |

## Key Decisions

| Decision | Choice | Why |
|----------|--------|-----|
| Host language | Go 1.25 | Single binary, sx patterns, ~10 txns/day makes IPC overhead irrelevant |
| Agent scripts | Python (via Monty sandbox) | LLMs write Python best; Monty provides security |
| Agent runtime | Monty subprocess via `uv run` | uv manages Python + pydantic-monty dependency; `ScriptRunner` interface allows swapping later |
| IPC protocol | JSON-RPC 2.0 over stdio | Standard protocol (same as MCP), supports pipelining for concurrent scripts |
| Agent format | Top-level scripts, flat primitives | No `run(ctx)`, no `from cleared import`. Primitives are global functions (`journal_add`, not `journal.add`). Last expression = output |
| Rules vs logic | Rules as data (YAML), agents as logic (Python) | Different change cadences, multiple agents share rules, better auditability |
| Validation | Monty type_check + dry run + behavioral diff | Monty's built-in type checker replaces external linting (ruff, etc.) |
| Data format | CSV in git repo | Portable, auditable, no vendor lock-in |
| Orchestration | Python control flow + event emission | if/for is clearer than formal behavior trees; `ctx_emit()` for inter-agent coordination |
| LLM role | Customization + edge cases only | Daily workflow is 100% code, no LLM dependency |
| Approval model | Everything approved via swipe UI | LLM describes intent in plain English, user approves |
| Bank data source | Pluggable, start with CSV import | Tiller has no API; CSV import works for anyone |

## Reference Docs

- **[Architecture](./architecture.md)** — 3-layer system, Go runtime, Python agents, data layer
- **[Agent System](./agent-system.md)** — How agents work, primitives API, sandbox, meta-agent, lifecycle
- **[Self-Improvement](./self-improvement.md)** — Learning, optimization, testing, reconciliation agents
- **[Data Model](./data-model.md)** — Schemas, invariants, git conventions, repo structure
- **[Spike Plan](./spikes.md)** — Ordered technical spikes with success criteria

## Spikes (Complete)

All 4 spikes passed. See [Spike Plan](./spikes.md) for detailed results.

| Spike | Status | Key Finding |
|-------|--------|-------------|
| **1: Go ↔ Monty sandbox** | **PASS** | JSON-RPC 2.0 over stdio, 0.41ms/call, concurrent scripts work |
| **2: LLM writes working agents** | **PASS** | Top-level script pattern, flat primitives, type stubs sufficient |
| **3: Validation pipeline** | **PASS** | Monty type_check + dry run + behavioral diff, 15ms/run |
| **4: End-to-end loop** | **PASS** | Full CSV → journal → git commit in 89ms, all invariants hold |

## Full Roadmap (post-spikes)

| Phase | Status | What |
|-------|--------|------|
| **Foundation** | **DONE** | Go services (journal, accounts, importer, gitops, validation) + CLI (`init`, `agent run`) |
| **Brain** | | Categorization engine (rules + LLM fallback), bootstrap import |
| **Reports** | | P&L, Balance Sheet, Excel export, CPA package |
| **Swipe UI** | | `cleared server`, web swipe UI, chat interface, meta-agent |
| **Intelligence** | | Self-improvement agents (learning, optimization, testing w/ test ratchet, reconciliation, accrual anticipator, month-end close rehearsal) |

### Foundation Phase — Completed

Built in 4 steps:

| Step | What | Commit |
|------|------|--------|
| 1. Project skeleton | Models, CSV I/O, buildinfo, Makefile, CI config | `1f91867` |
| 2. Services + validation | Config, accounts, journal (6 invariants), gitops, `cleared init` | `d976eb1` |
| 3. Importer + agent log | Chase CSV parser, importer registry, agent log CSV I/O | `f11df1e` |
| 4. Agent runtime | Embedded bridge.py, 15 primitives, `cleared agent run`, end-to-end tests | `7ce2df0` |

The Foundation phase delivers a working pipeline: `cleared init` creates a repo, bank CSVs are dropped in `import/`, and `cleared agent run ingest` parses transactions, creates balanced journal entries, moves processed files, commits to git, and logs agent actions.

## Build & CI

Follow sx project patterns — see [Architecture](./architecture.md#build--ci):
- Makefile with ldflags, prepush, install targets
- golangci-lint, goreleaser
- GitHub workflows: test.yml, release.yml
