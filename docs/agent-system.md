# Cleared — Agent System

## What is an Agent?

An agent is a top-level Python script in the git repo that:
1. Has metadata (name, trigger, schedule, description) in a docstring
2. Calls Go primitives as flat global functions (no imports, no `run()` wrapper)
3. Returns a result as the last expression
4. Runs in a Monty sandbox (no filesystem, no network, no imports, no classes, no try/except)

```python
# agents/ingest.py
"""
name: Daily Ingest
trigger: schedule
schedule: 0 6 * * *
description: Import new bank transactions, classify via rules, route by confidence
"""
files = importer_scan()
if not files:
    ctx_log("No new files to import")
    {"imported": 0, "confirmed": 0, "review": 0}
else:
    threshold = config_get("thresholds.auto_confirm")
    total_imported = 0
    total_confirmed = 0
    total_review = 0

    for f in files:
        txns = importer_parse(f["name"])
        ctx_log("Parsed " + str(len(txns)) + " transactions from " + f["name"])

        for txn in txns:
            match = rules_match(description=txn["description"], amount=txn["amount"])

            if match and match["confidence"] >= threshold:
                if txn["amount"] < 0:
                    journal_add_double(
                        date=txn["date"], description=txn["description"],
                        debit_account=match["account_id"], credit_account=1010,
                        amount=abs(txn["amount"]), counterparty=match["vendor_name"],
                        reference=txn["reference"], confidence=match["confidence"],
                        status="auto-confirmed", evidence="rule match: " + match["pattern"])
                else:
                    journal_add_double(
                        date=txn["date"], description=txn["description"],
                        debit_account=1010, credit_account=4010,
                        amount=txn["amount"], counterparty=match["vendor_name"],
                        reference=txn["reference"], confidence=match["confidence"],
                        status="auto-confirmed", evidence="rule match: " + match["pattern"])
                total_confirmed = total_confirmed + 1
            else:
                if txn["amount"] < 0:
                    journal_add_double(
                        date=txn["date"], description=txn["description"],
                        debit_account=5030, credit_account=1010,
                        amount=abs(txn["amount"]), reference=txn["reference"],
                        confidence=0.0, status="pending-review", evidence="no confident match")
                else:
                    journal_add_double(
                        date=txn["date"], description=txn["description"],
                        debit_account=1010, credit_account=4010,
                        amount=txn["amount"], reference=txn["reference"],
                        confidence=0.0, status="pending-review", evidence="no confident match")
                queue_add_review(entry_id="pending", description=txn["description"], confidence=0.0)
                total_review = total_review + 1

            total_imported = total_imported + 1

        importer_mark_processed(f["name"])

    git_commit("import: " + str(total_imported) + " transactions from " + str(len(files)) + " files")
    ctx_log("Done: " + str(total_confirmed) + " auto-confirmed, " + str(total_review) + " for review")
    {"imported": total_imported, "confirmed": total_confirmed, "review": total_review}
```

### Monty Sandbox Constraints

Agents run in pydantic-monty, a Rust-based sandboxed Python interpreter. The following are **not available**:
- `import` statements (all functionality comes from primitives)
- `open()`, `eval()`, `exec()`, `__import__()` — blocked by sandbox
- `class` definitions — use dicts instead
- `try`/`except` — errors propagate to the runtime
- Generators, `with` statements, decorators
- f-strings — use string concatenation with `str()`
- Standard library modules (os, sys, subprocess, etc.)

What **is** available: variables, functions, if/elif/else, for/while, list/dict comprehensions, basic types (int, float, str, bool, list, dict, None).

## Agent Lifecycle

```
CREATE          → LLM writes script (or user copies a template)
VALIDATE        → Sandbox lint + dry run + invariant check
APPROVE         → Swipe card: "New agent: Morning Digest. Approve?"
DEPLOY          → Git commit, scheduler picks it up
EXECUTE         → Scheduler ticks agent on trigger
MONITOR         → Agent logger tracks every action
IMPROVE         → Nightly agents analyze performance, propose tweaks
MODIFY          → User requests change via chat → LLM modifies → validate → approve
ROLLBACK        → Git revert if agent causes problems
```

## Triggers

| Trigger | How it works |
|---------|-------------|
| `schedule` | Cron expression, e.g., `0 6 * * *` (6 AM daily) |
| `file_watch` | New file appears in a watched directory (e.g., `import/`) |
| `event` | Another agent emits an event (e.g., "new transactions imported") |
| `on_demand` | `cleared agent run <name>` or triggered from swipe UI |
| `webhook` | HTTP endpoint receives a call (future, via `cleared server`) |

## Primitives API

Go functions exposed as **flat global functions** in the Monty sandbox. When the agent calls a primitive, Monty pauses execution, the bridge sends a JSON-RPC request to Go, Go executes the handler and returns the result, and Monty resumes.

Primitives use `snake_case` with a domain prefix (e.g., `journal_add_double`, not `journal.add`). This is required by Monty's external function mechanism — they must be standalone function names, not module methods.

### Journal
```python
journal_add_double(date, description, debit_account, credit_account, amount,
                   counterparty=None, reference=None, confidence=0.0,
                   status="pending-review", evidence=None)  # balanced by construction
journal_query(status=None, since=None, month=None, account=None)  # read entries
journal_void(entry_id, reason)           # create reversing entry
journal_update_status(entry_id, status)  # confirm, correct, etc.
journal_balance(account=None)            # calculate account balances
```

### Accounts
```python
accounts_list()                    # all accounts
accounts_get(account_id)           # single account
accounts_exists(account_id)        # validation check
accounts_by_type(account_type)     # filter by asset/liability/etc.
```

### Importer
```python
importer_scan()                    # list new files in import/
importer_parse(filename)           # parse bank CSV → list of transaction dicts
importer_mark_processed(filename)  # move to import/processed/
importer_deduplicate(txns)         # filter out already-imported
```

### Rules
```python
rules_match(description, amount=None)  # find best matching rule
rules_add(pattern, account_id, vendor_name, confidence)  # create new rule
rules_update(rule_id, ...)         # modify existing rule
rules_list()                       # all rules
```

### Git
```python
git_commit(message)                # stage all + commit
git_log(n=10)                      # recent commits
```

### Queue
```python
queue_add_review(entry_id, description, confidence=0.0)  # add to swipe queue
queue_pending()                    # list pending items
```

### Config
```python
config_get(key)                    # read config value by dotted key, e.g. "thresholds.auto_confirm"
```

### Context
```python
ctx_log(message)                   # write to agent log
ctx_emit(event_name, data=None)    # trigger event-based agents
```

### LLM (optional, future)
```python
llm_classify(description, context=None)  # categorize a transaction
llm_summarize(data, prompt)              # generate text summary
```

### Transaction Dict Shape
Primitives return and accept transaction dicts:
```python
{
    "date": "2025-01-03",          # ISO format YYYY-MM-DD
    "description": "GITHUB *PRO",  # normalized bank description
    "amount": -4.00,               # negative = expense, positive = income
    "reference": "ACH_abc123",     # bank reference ID
    "type": "ACH_DEBIT",           # bank transaction type
}
```

## Meta-Agent (LLM-Powered Customization)

The meta-agent runs when the user chats in the swipe UI. It is NOT an agent script — it's a Go-side LLM integration that:

1. Receives user's natural language request
2. Reads relevant agent scripts, rules, config from the repo
3. Generates modifications (new/changed scripts, rules, config)
4. Runs validation pipeline on the changes
5. Presents a plain-English description + validation results as a swipe card
6. On approval: git commit, changes take effect

**The user never sees code.** They see:
- What the agent wants to do (plain English)
- Validation results (passed/failed)
- What will change (high-level description)

Example flow:
```
User: "Batch my notifications into a morning email"

Meta-agent reads: agents/ingest.py, config
Meta-agent writes: agents/digest.py (new), modifies agents/ingest.py
Meta-agent validates: dry run passes, no invariant violations
Meta-agent presents:
  "I'll create a Morning Digest agent that emails you a summary
   at 6 AM with an 'Approve All' button. Individual notifications
   will stop. Dry run: ✓ Passed."
User swipes right → git commit → live tomorrow
```

## Agent Execution Pattern (Resolved)

Decided during spikes: **Top-level scripts with event emission.**

- Each agent is a top-level Python script (no `run(ctx)` wrapper, no class)
- Primitives are global functions — the script just calls them
- The last expression is the script's return value
- Inter-agent coordination via `ctx_emit(event_name)` — the scheduler triggers event-listening agents
- Shared logic lives in `scripts/` — reusable Monty scripts that agents can call via a primitive (see [Self-Improvement](./self-improvement.md#shared-sub-scripts))

This is simpler than a framework, LLMs generate it reliably, and Python control flow (if/for) handles all the orchestration we need.
