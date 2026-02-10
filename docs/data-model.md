# Cleared — Data Model

## Repository Structure

What `cleared init` creates:

```
<business-name>/
├── .gitignore                           # receipts/, exports/, queue/, .cleared-cache/
├── cleared.yaml                         # Business config, agent schedules, thresholds
├── accounts/
│   └── chart-of-accounts.csv            # Account definitions with tax mappings
├── rules/
│   └── categorization-rules.yaml        # Learned vendor→category mappings
├── agents/                              # Python agent scripts (LLM-written, top-level scripts)
│   ├── ingest.py                        # Import + classify + route
│   └── ...                              # More agents added over time
├── scripts/                             # Shared Monty sub-scripts (called via script_run primitive)
│   └── ...                              # Created by learning agents, shared across agents
├── templates/                           # Email/report templates
├── tests/                               # Agent-generated tests
├── logs/
│   └── agent-log.csv                    # Append-only log of all agent actions
├── import/                              # Watch directory: drop CSVs here
│   ├── .gitkeep
│   └── processed/                       # Processed files moved here
├── YYYY/
│   └── MM/
│       ├── journal.csv                  # Monthly transaction journal
│       └── reconciliation.csv           # Bank reconciliation status
├── receipts/                            # ← GITIGNORED
├── exports/                             # ← GITIGNORED
└── queue/                               # ← GITIGNORED
    └── pending.json
```

## Schemas

### journal.csv — Monthly Transaction Journal

One row per journal entry **leg**. Double-entry means each transaction produces 2+ rows.

| Column | Type | Required | Description |
|--------|------|----------|-------------|
| `entry_id` | string | yes | `YYYY-MM-NNNx` — NNN sequential, x = leg (a,b,c) |
| `date` | date | yes | ISO format, must fall within this file's month |
| `account_id` | integer | yes | Must exist in chart-of-accounts.csv |
| `description` | string | yes | Normalized description |
| `debit` | decimal | conditional | Exactly one of debit/credit per row |
| `credit` | decimal | conditional | Exactly one of debit/credit per row |
| `counterparty` | string | no | Who you paid or who paid you |
| `reference` | string | no | Invoice #, check #, bank transaction ID |
| `confidence` | decimal | no | 0.0–1.0, agent confidence in category |
| `status` | enum | yes | See below |
| `evidence` | string | no | "rule match", "invoice match", "LLM classification" |
| `receipt_hash` | string | no | Hash of file in receipts/ |
| `tags` | string | no | Semicolon-separated |
| `notes` | string | no | Free-form |

**Status values:** `auto-confirmed` | `pending-review` | `user-confirmed` | `user-corrected` | `voided` | `bootstrap-confirmed`

**Example:**
```csv
entry_id,date,account_id,description,debit,credit,counterparty,reference,confidence,status,evidence,receipt_hash,tags,notes
2025-01-001a,2025-01-03,5020,GitHub Pro subscription,4.00,,GitHub,plaid_abc123,0.98,auto-confirmed,rule match: GITHUB*,,recurring;software,
2025-01-001b,2025-01-03,1010,GitHub Pro subscription,,4.00,GitHub,plaid_abc123,0.98,auto-confirmed,rule match: GITHUB*,,recurring;software,
```

### chart-of-accounts.csv

| Column | Type | Description |
|--------|------|-------------|
| `account_id` | integer | 1xxx assets, 2xxx liabilities, 3xxx equity, 4xxx revenue, 5xxx expenses |
| `account_name` | string | Human-readable name |
| `account_type` | enum | `asset` / `liability` / `equity` / `revenue` / `expense` |
| `parent_id` | integer | Parent account, empty for top-level |
| `tax_line` | string | Tax form mapping (e.g., `schedule_c_8`) |
| `description` | string | What belongs here |

Ship with ~30 default accounts per entity type.

### categorization-rules.yaml

```yaml
rules:
  - vendor_pattern: "GITHUB*"
    vendor_name: "GitHub"
    account_id: 5020
    confidence: 0.98
    times_seen: 12
    times_confirmed: 12
    times_corrected: 0
    last_seen: "2025-01-03"
    avg_amount: 4.00
    amount_stddev: 0.00
    source: "user_confirmed"
```

### agent-log.csv

| Column | Type | Description |
|--------|------|-------------|
| `timestamp` | datetime | When the action occurred |
| `agent` | string | Which agent |
| `action` | string | What it did |
| `details` | string | Human-readable explanation |
| `entry_id` | string | Related journal entry if applicable |
| `commit_hash` | string | Git commit produced, if any |

### reconciliation.csv

| Column | Description |
|--------|-------------|
| `bank_account_id` | References chart of accounts |
| `date` | End-of-month date |
| `bank_balance` | Balance per bank statement |
| `book_balance` | Calculated from journal |
| `difference` | Should be 0.00 when reconciled |
| `status` | `reconciled` / `pending` / `discrepancy` |
| `notes` | Explanation if discrepancy |

## 6 Journal Invariants

Enforced by the Go runtime on every write. No agent can bypass these.

1. **Every entry group must balance**: sum(debits) == sum(credits) for rows sharing the same `YYYY-MM-NNN` prefix
2. **Exactly one of debit or credit per row**: never both, never neither
3. **Valid account references**: all account_id values must exist in chart-of-accounts.csv
4. **Date within month**: all dates must fall within the file's year/month
5. **Unique sequential IDs**: entry IDs must be unique and sequential within a month
6. **Exact decimals**: all monetary values use shopspring/decimal, never floating point

## Git Conventions

**Every data change is a commit.** No uncommitted changes in normal operation.

**Commit prefixes:**
```
init: Initialize cleared repository for <business>
import: Chase checking 2025-01-01 to 2025-01-31 (47 transactions)
categorize: Updated 5 transaction categories
confirm: User confirmed 3 pending transactions
correct: User corrected category on 2025-01-015 (was Software, now COGS)
void: Voided 2025-01-015 — duplicate payment
reconcile: January 2025 bank reconciliation complete
close: Month-end close January 2025
config: Updated chart of accounts
bootstrap: Imported 6 months of history (312 transactions)
learn: Updated 3 rules from user corrections
agent: Created new agent: Morning Digest
test: Ran 47 tests, 2 failures
optimize: Consolidated 5 overlapping rules
```

**Voiding, not deleting.** Transactions are never removed. Mistakes get reversing entries.

## Config (cleared.yaml)

```yaml
business:
  name: "Acme Consulting LLC"
  entity_type: "llc_single_member"
  tax_year_end: "12-31"
  currency: "USD"

fiscal:
  start_month: 1

bank_accounts:
  - id: "chase_checking"
    name: "Chase Business Checking"
    type: "checking"
    csv_format: "chase"

agent:
  schedule: "0 6 * * *"
  watch_dir: "./import"
  auto_commit: true

thresholds:
  auto_confirm: 0.95
  high_confidence: 0.90
  review: 0.70
  anomaly_stddev: 2.0

llm:
  provider: "anthropic"
  model: "claude-sonnet-4-5-20250929"

git:
  author_name: "Cleared Agent"
  author_email: "agent@cleared.dev"

self_improvement:
  enabled: true
  max_changes_per_night: 20
  cooldown_hours: 24

notifications:
  method: "email"
  address: "owner@example.com"
  daily_digest_time: "06:00"
```
