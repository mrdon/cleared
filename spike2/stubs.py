"""
Cleared Primitives API — Type stubs for agent development.

These functions are provided by the Go runtime and available as globals
in the Monty sandbox. Agents call them directly; execution pauses,
Go handles the call, and returns the result.

All monetary values are floats (Go uses exact decimals internally).
All dates are ISO format strings "YYYY-MM-DD".
"""

# --- Types ---

# Transaction: a raw bank transaction from CSV import
# {
#   "date": "2025-01-03",
#   "description": "GITHUB *PRO SUBSCRIPTION",
#   "amount": -4.00,           # negative = debit, positive = credit
#   "reference": "plaid_abc",  # bank's transaction ID
#   "bank_account": "chase_checking"
# }

# Entry: a journal entry leg
# {
#   "entry_id": "2025-01-001a",
#   "date": "2025-01-03",
#   "account_id": 5020,
#   "description": "GitHub Pro subscription",
#   "debit": 4.00,             # exactly one of debit/credit is set
#   "credit": 0,
#   "counterparty": "GitHub",
#   "reference": "plaid_abc",
#   "confidence": 0.98,
#   "status": "auto-confirmed",
#   "evidence": "rule match: GITHUB*",
#   "tags": "recurring;software"
# }

# RuleMatch: result of matching a transaction against categorization rules
# {
#   "pattern": "GITHUB*",
#   "vendor_name": "GitHub",
#   "account_id": 5020,
#   "confidence": 0.98
# }

# Account: a chart of accounts entry
# {
#   "account_id": 5020,
#   "account_name": "Software & SaaS",
#   "account_type": "expense",    # asset|liability|equity|revenue|expense
#   "parent_id": 5000,
#   "tax_line": "schedule_c_18",
#   "description": "Software subscriptions and SaaS tools"
# }


# --- journal ---

def journal_add(*, date: str, account_id: int, description: str,
                debit: float = 0, credit: float = 0,
                counterparty: str = "", reference: str = "",
                confidence: float = 0.0, status: str = "pending-review",
                evidence: str = "", tags: str = "", notes: str = "") -> dict:
    """Add a journal entry leg. Go validates and writes to CSV.

    Status values: auto-confirmed | pending-review | user-confirmed |
                   user-corrected | voided | bootstrap-confirmed

    Returns: {"entry_id": "2025-01-001a", "success": True}
    """
    ...

def journal_add_double(*, date: str, description: str,
                       debit_account: int, credit_account: int,
                       amount: float, counterparty: str = "",
                       reference: str = "", confidence: float = 0.0,
                       status: str = "pending-review", evidence: str = "",
                       tags: str = "", notes: str = "") -> dict:
    """Add a balanced double-entry (debit + credit legs).

    Shorthand for two journal_add calls that always balance.
    Returns: {"entry_id": "2025-01-001", "success": True}
    """
    ...

def journal_query(*, status: str = "", since: str = "",
                  month: str = "", account_id: int = 0,
                  limit: int = 0) -> list:
    """Query journal entries. All filters are optional.

    month format: "2025-01"
    Returns: list of Entry dicts
    """
    ...

def journal_void(entry_id: str, reason: str) -> dict:
    """Void an entry by creating a reversing entry.

    Returns: {"reversal_id": "2025-01-002", "success": True}
    """
    ...

def journal_update_status(entry_id: str, status: str) -> dict:
    """Update the status of an entry.

    Returns: {"success": True}
    """
    ...

def journal_balance(*, account_id: int = 0, month: str = "") -> dict:
    """Get account balance(s).

    Returns: {"account_id": balance} or {"balance": amount} if single account
    """
    ...


# --- accounts ---

def accounts_list() -> list:
    """All accounts in chart of accounts. Returns: list of Account dicts"""
    ...

def accounts_get(account_id: int) -> dict:
    """Single account lookup. Returns: Account dict or None"""
    ...

def accounts_exists(account_id: int) -> bool:
    """Check if account exists."""
    ...

def accounts_by_type(account_type: str) -> list:
    """Filter accounts. account_type: asset|liability|equity|revenue|expense"""
    ...


# --- importer ---

def importer_scan() -> list:
    """List new CSV files in the import/ watch directory.

    Returns: list of {"name": "chase_jan.csv", "path": "import/chase_jan.csv", "size": 1234}
    """
    ...

def importer_parse(file_name: str, *, format: str = "") -> list:
    """Parse a bank CSV file into transactions.

    format: auto-detected if empty. Supported: "chase", "generic"
    Returns: list of Transaction dicts
    """
    ...

def importer_mark_processed(file_name: str) -> dict:
    """Move file to import/processed/. Returns: {"success": True}"""
    ...

def importer_deduplicate(transactions: list) -> list:
    """Filter out transactions already in the journal (by reference ID).

    Returns: list of Transaction dicts (only new ones)
    """
    ...


# --- rules ---

def rules_match(*, description: str, amount: float = 0) -> dict:
    """Find best matching categorization rule for a transaction.

    Returns: RuleMatch dict or None if no match
    """
    ...

def rules_add(*, pattern: str, vendor_name: str, account_id: int,
              confidence: float = 0.90, source: str = "agent") -> dict:
    """Create a new categorization rule.

    Returns: {"rule_id": "...", "success": True}
    """
    ...

def rules_update(rule_id: str, **kwargs) -> dict:
    """Update an existing rule. Pass only fields to change.

    Returns: {"success": True}
    """
    ...

def rules_list() -> list:
    """All categorization rules. Returns: list of rule dicts"""
    ...


# --- git ---

def git_commit(message: str) -> dict:
    """Stage all changes and commit.

    Use standard prefixes: import:, categorize:, confirm:, correct:,
    void:, learn:, agent:, test:, optimize:
    Returns: {"commit_hash": "abc123", "success": True}
    """
    ...

def git_log(*, n: int = 10) -> list:
    """Recent commits. Returns: list of {"hash", "message", "date"}"""
    ...


# --- queue ---

def queue_add_review(*, entry_id: str, description: str,
                     suggested_account: int = 0,
                     confidence: float = 0.0) -> dict:
    """Add a transaction to the swipe review queue.

    Returns: {"item_id": "...", "success": True}
    """
    ...

def queue_pending() -> list:
    """List pending review items. Returns: list of queue item dicts"""
    ...


# --- config ---

def config_get(key: str) -> any:
    """Read a config value from cleared.yaml.

    Keys: "business.name", "business.entity_type", "thresholds.auto_confirm",
          "thresholds.high_confidence", "thresholds.review", "agent.schedule"
    """
    ...


# --- ctx (execution context) ---

def ctx_log(message: str) -> None:
    """Write to the agent execution log."""
    ...

def ctx_emit(event_name: str, data: dict = None) -> None:
    """Emit an event to trigger other agents."""
    ...

def ctx_dry_run() -> bool:
    """Returns True if this is a dry run (no writes will persist)."""
    ...
