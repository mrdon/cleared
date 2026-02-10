# Cleared — Self-Improvement System

## Three Feedback Loops

```
LOOP 1: Execution (daily, no LLM, code only)
  Ingest → Classify via rules → Route by confidence → Commit
  Also: detect missing recurring transactions (accruals), month-end close rehearsal
  Deterministic. Fast. Reliable.

LOOP 2: Learning (nightly, uses LLM for analysis)
  Review today's approvals/corrections →
  Identify patterns → Generate new rules or modify agents →
  Validate + generate regression tests (test ratchet) → Propose or auto-apply

LOOP 3: Meta-improvement (nightly, uses LLM)
  Evaluate all agents, rules, and tests →
  Find inefficiencies, gaps, edge cases →
  Write new tests, optimize workflows, consolidate rules →
  Validate + regression tests must pass → Propose changes
```

Loop 1 runs every day without LLM. Loops 2 and 3 use LLM but are nightly, async, and non-critical. An LLM outage doesn't stop bookkeeping — it just pauses improvement.

**Test ratchet principle**: Every nightly modification (Loop 2 or 3) generates a regression test alongside the change. Future modifications must pass all existing tests before deployment. The test suite only grows, creating a one-way quality ratchet.

## Improvement Agents

### Learning Agent — Turn corrections into rules

- Reviews user-confirmed and user-corrected transactions daily
- Groups corrections by vendor pattern
- Creates or updates categorization rules
- Proposes via swipe: "I learned 3 new vendor rules from your corrections"

### Optimization Agent — Keep the system lean

- Reviews rule set for overlaps, conflicts, dead rules (no match in 90+ days)
- Analyzes agent execution logs for bottlenecks
- Tunes confidence thresholds based on actual approval rates
- Proposes rule consolidation and workflow simplification

### Testing Agent — Continuous QA + Test Ratchet

- Uses LLM to generate edge-case test data (malformed CSVs, boundary amounts, special characters, duplicates, date edge cases)
- Dry-runs agents against test data in sandbox
- Finds failures, proposes fixes
- Checks for regressions after agent modifications
- Security scanning of agent scripts
- **Test ratchet**: Every time any nightly agent modifies a rule or rewrites an agent script, the testing agent generates a regression test alongside it — a snapshot of specific transactions and expected output. These tests run *before* any future modification is deployed. If a proposed change breaks an existing test, it's blocked. Over months, the test suite grows into an immune system — the system can only get better, never silently regress.

### Reconciliation Agent — Catch accounting errors

- Compares journal balances against bank statement balances
- Identifies orphaned entries (in journal but not bank)
- Detects missing entries (in bank but not journal)
- Proposes corrections or flags for review

### Accrual Anticipator — Record what *should* happen, not just what did

- Scans journal for recurring transactions (rent, SaaS, payroll, insurance) and builds frequency/amount profiles
- Detects when an expected recurring transaction is *missing* and creates a draft accrual entry (marked pending-review)
- Identifies prepaid expenses that need monthly amortization entries and generates them
- No LLM required — pure pattern detection on journal history
- Example: "Gusto Payroll expected on the 15th, now the 16th — accruing $4,200 pending actual post. Will auto-reverse when actual transaction appears."

### Month-End Close Rehearsal — Proactive close preparation

- Runs on the 25th of each month (configurable) as a dry-run of the month-end close
- Produces a punch list: unreconciled transactions, missing recurring entries, accounts with unusual balances, aging receivables, unclassified transactions
- Gives the user 5 days of lead time instead of a scramble on the 1st
- No LLM required — invariant checks, pattern detection, and balance calculations
- Example output: "Month-end readiness: 87%. Issues: 3 unclassified transactions from Jan 12-14, no depreciation entry this month (last month was $125), $2,400 invoice open since Nov 15 — may need write-off."

### Cleanup Agent — Keep shared scripts healthy

- Scans `scripts/` directory for shared sub-scripts
- Checks which scripts are still referenced by agents
- Flags unused scripts for deletion
- Reports on script usage patterns and dependencies
- Runs weekly or after agent modifications

## Shared Sub-Scripts

Learning agents can create reusable Monty scripts in `scripts/` that multiple agents call via a `script_run(name)` primitive. This keeps core agent scripts small and focused while extracting common patterns.

Example: if the learning agent notices multiple agents doing the same vendor normalization logic, it can extract that into `scripts/normalize_vendor.py` and have agents call `script_run("normalize_vendor")` instead.

Sub-scripts:
- Live in `scripts/` directory (version controlled in git)
- Are themselves Monty-sandboxed (same constraints as agents)
- Are created/modified by learning agents, validated by the same pipeline
- Are monitored by the cleanup agent for continued usage

## Safety Guardrails

### Rate Limiting
- Each agent can propose at most **5 changes per night**
- Total system modifications per day: **20 max**
- After a change is deployed, that agent/rule has a **24-hour cooldown**

### Regression Detection
After deploying a change, monitor for 24 hours:
- Did the review queue spike? (2x normal = revert)
- Did any invariant violations occur? (any = revert)
- Did error rates increase? (50% increase = revert)

### Change Budget (in cleared.yaml)
```yaml
self_improvement:
  enabled: true
  max_changes_per_night: 20
  cooldown_hours: 24
  auto_revert_on:
    review_queue_spike: 2.0
    invariant_violations: 1
    error_rate_increase: 0.5
```

### Immutable Invariants (Go-enforced, agents cannot bypass)
1. Debits = Credits (every entry group)
2. Valid account references
3. Sequential entry IDs
4. Date within file's month
5. Exact decimal arithmetic
6. Git commit on every data write
