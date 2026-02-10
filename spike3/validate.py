"""
Spike 3: Validation Pipeline

Three stages:
1. Static validation — Monty type checker + forbidden construct detection
2. Dry run — Execute with recording primitives, check invariants
3. Behavioral diff — Compare old vs new agent, summarize changes
"""

from dataclasses import dataclass

from pydantic_monty import Monty, MontyComplete, MontyRuntimeError, MontySnapshot, MontySyntaxError, MontyTypingError

# All primitives agents can call
ALL_PRIMITIVES = [
    "journal_add", "journal_add_double", "journal_query", "journal_void",
    "journal_update_status", "journal_balance",
    "accounts_list", "accounts_get", "accounts_exists", "accounts_by_type",
    "importer_scan", "importer_parse", "importer_mark_processed", "importer_deduplicate",
    "rules_match", "rules_add", "rules_update", "rules_list",
    "git_commit", "git_log",
    "queue_add_review", "queue_pending",
    "config_get",
    "ctx_log", "ctx_emit", "ctx_dry_run",
]

# Type stubs for Monty's type checker
TYPE_STUBS = """
def journal_add(*, date: str, account_id: int, description: str,
                debit: float = 0, credit: float = 0,
                counterparty: str = "", reference: str = "",
                confidence: float = 0.0, status: str = "pending-review",
                evidence: str = "", tags: str = "", notes: str = "") -> dict: ...

def journal_add_double(*, date: str, description: str,
                       debit_account: int, credit_account: int,
                       amount: float, counterparty: str = "",
                       reference: str = "", confidence: float = 0.0,
                       status: str = "pending-review", evidence: str = "",
                       tags: str = "", notes: str = "") -> dict: ...

def journal_query(*, status: str = "", since: str = "",
                  month: str = "", account_id: int = 0,
                  limit: int = 0) -> list: ...

def journal_void(entry_id: str, reason: str) -> dict: ...
def journal_update_status(entry_id: str, status: str) -> dict: ...
def journal_balance(*, account_id: int = 0, month: str = "") -> dict: ...

def accounts_list() -> list: ...
def accounts_get(account_id: int) -> dict: ...
def accounts_exists(account_id: int) -> bool: ...
def accounts_by_type(account_type: str) -> list: ...

def importer_scan() -> list: ...
def importer_parse(file_name: str, *, format: str = "") -> list: ...
def importer_mark_processed(file_name: str) -> dict: ...
def importer_deduplicate(transactions: list) -> list: ...

def rules_match(*, description: str, amount: float = 0) -> dict: ...
def rules_add(*, pattern: str, vendor_name: str, account_id: int,
              confidence: float = 0.90, source: str = "agent") -> dict: ...
def rules_update(rule_id: str, **kwargs) -> dict: ...
def rules_list() -> list: ...

def git_commit(message: str) -> dict: ...
def git_log(*, n: int = 10) -> list: ...

def queue_add_review(*, entry_id: str, description: str,
                     suggested_account: int = 0,
                     confidence: float = 0.0) -> dict: ...
def queue_pending() -> list: ...

def config_get(key: str) -> any: ...

def ctx_log(message: str) -> None: ...
def ctx_emit(event_name: str, data: dict = None) -> None: ...
def ctx_dry_run() -> bool: ...
"""

# Valid account IDs for dry run validation
VALID_ACCOUNTS = {1010, 1020, 2010, 3010, 4010, 4020, 5010, 5020, 5030, 5040, 5050}
VALID_STATUSES = {"auto-confirmed", "pending-review", "user-confirmed", "user-corrected", "voided", "bootstrap-confirmed"}

# Synthetic data for dry runs
SYNTHETIC_TRANSACTIONS = [
    {"date": "2025-01-03", "description": "GITHUB *PRO", "amount": -4.00,
     "reference": "ref001", "bank_account": "chase_checking"},
    {"date": "2025-01-05", "description": "AWS *SERVICES", "amount": -127.50,
     "reference": "ref002", "bank_account": "chase_checking"},
    {"date": "2025-01-15", "description": "CLIENT PAYMENT", "amount": 3500.00,
     "reference": "ref003", "bank_account": "chase_checking"},
]

SYNTHETIC_STUBS = {
    "importer_scan": [{"name": "test.csv", "path": "import/test.csv", "size": 100}],
    "importer_parse": SYNTHETIC_TRANSACTIONS,
    "importer_deduplicate": SYNTHETIC_TRANSACTIONS,
    "importer_mark_processed": {"success": True},
    "journal_query": [],
    "journal_void": {"reversal_id": "2025-01-099", "success": True},
    "journal_update_status": {"success": True},
    "journal_balance": {"1010": 5000.00},
    "accounts_list": [{"account_id": a, "account_name": f"Account {a}", "account_type": "expense"} for a in VALID_ACCOUNTS],
    "accounts_get": {"account_id": 5020, "account_name": "Software", "account_type": "expense"},
    "accounts_exists": True,
    "accounts_by_type": [],
    "rules_match": {"pattern": "GITHUB*", "vendor_name": "GitHub", "account_id": 5020, "confidence": 0.98},
    "rules_add": {"rule_id": "r_new", "success": True},
    "rules_update": {"success": True},
    "rules_list": [],
    "git_commit": {"commit_hash": "abc123", "success": True},
    "git_log": [],
    "queue_add_review": {"item_id": "q001", "success": True},
    "queue_pending": [],
    "config_get": 0.95,
    "ctx_log": None,
    "ctx_emit": None,
    "ctx_dry_run": True,
}

CONFIG_VALUES = {
    "business.name": "Test Corp",
    "thresholds.auto_confirm": 0.95,
    "thresholds.review": 0.70,
}


# ============================================================
# Stage 1: Static Validation
# ============================================================

@dataclass
class ValidationIssue:
    severity: str  # "error" or "warning"
    message: str
    location: str = ""


def static_validate(script: str) -> list[ValidationIssue]:
    """Check script for syntax errors, type errors, and forbidden constructs."""
    issues = []

    # 1. Syntax check — can Monty parse it?
    try:
        m = Monty(code=script, external_functions=ALL_PRIMITIVES)
    except MontySyntaxError as e:
        issues.append(ValidationIssue("error", f"Syntax error: {e}"))
        return issues  # Can't continue if it won't parse

    # 2. Type check — do primitive calls have correct argument types?
    try:
        m.type_check(prefix_code=TYPE_STUBS)
    except MontyTypingError as e:
        # Parse individual errors from the message
        for line in str(e).split("error["):
            if line.strip():
                issues.append(ValidationIssue("error", f"Type error: {line.split(chr(10))[0].strip()}"))

    # 3. Sandbox check — try to run and catch forbidden constructs
    # (Monty blocks these at runtime, but we want to detect them pre-run)
    forbidden = ["open(", "eval(", "exec(", "__import__(", "compile("]
    for construct in forbidden:
        if construct in script:
            issues.append(ValidationIssue("error", f"Forbidden construct: {construct}"))

    return issues


# ============================================================
# Stage 2: Dry Run
# ============================================================

@dataclass
class DryRunAction:
    primitive: str
    args: list
    kwargs: dict
    result: any


@dataclass
class InvariantViolation:
    rule: str
    message: str
    details: dict


@dataclass
class DryRunResult:
    success: bool
    actions: list[DryRunAction]
    violations: list[InvariantViolation]
    output: any
    error: str = ""


def dry_run(script: str, stub_overrides: dict = None) -> DryRunResult:
    """Execute script with recording primitives that check invariants."""
    stubs = dict(SYNTHETIC_STUBS)
    if stub_overrides:
        stubs.update(stub_overrides)

    m = Monty(code=script, external_functions=ALL_PRIMITIVES)
    actions = []
    violations = []

    # Track double-entry balance per entry group
    entry_counter = 0
    pending_entries = []  # entries from journal_add not yet balanced

    progress = m.start()

    while isinstance(progress, MontySnapshot):
        fname = progress.function_name
        args = list(progress.args)
        kwargs = dict(progress.kwargs)

        # Determine response
        if fname == "config_get" and len(args) > 0:
            result = CONFIG_VALUES.get(args[0])
        elif fname == "rules_match":
            desc = kwargs.get("description", "")
            if "CLIENT" in desc.upper() or "UNKNOWN" in desc.upper():
                result = None
            else:
                result = stubs.get(fname)
        else:
            result = stubs.get(fname)

        # Record the action
        actions.append(DryRunAction(fname, args, kwargs, result))

        # --- Invariant checks ---

        if fname == "journal_add":
            debit = kwargs.get("debit", 0)
            credit = kwargs.get("credit", 0)

            # Invariant 2: exactly one of debit/credit
            if debit and credit:
                violations.append(InvariantViolation(
                    "debit_xor_credit", "Both debit and credit set on same leg",
                    {"debit": debit, "credit": credit}))
            if not debit and not credit:
                violations.append(InvariantViolation(
                    "debit_xor_credit", "Neither debit nor credit set",
                    {"kwargs": kwargs}))

            # Invariant 3: valid account
            acct = kwargs.get("account_id", 0)
            if acct and acct not in VALID_ACCOUNTS:
                violations.append(InvariantViolation(
                    "valid_account", f"Unknown account_id: {acct}",
                    {"account_id": acct}))

            # Check status value
            status = kwargs.get("status", "")
            if status and status not in VALID_STATUSES:
                violations.append(InvariantViolation(
                    "valid_status", f"Invalid status: {status}",
                    {"status": status}))

            pending_entries.append(kwargs)

        if fname == "journal_add_double":
            amount = kwargs.get("amount", 0)

            # Invariant 1: amount must be positive for double entry
            if amount <= 0:
                violations.append(InvariantViolation(
                    "positive_amount", f"journal_add_double amount must be positive: {amount}",
                    {"amount": amount}))

            # Invariant 3: valid accounts
            for key in ("debit_account", "credit_account"):
                acct = kwargs.get(key, 0)
                if acct and acct not in VALID_ACCOUNTS:
                    violations.append(InvariantViolation(
                        "valid_account", f"Unknown {key}: {acct}",
                        {key: acct}))

            # Check status value
            status = kwargs.get("status", "")
            if status and status not in VALID_STATUSES:
                violations.append(InvariantViolation(
                    "valid_status", f"Invalid status: {status}",
                    {"status": status}))

            # journal_add_double is always balanced by definition — OK
            entry_counter += 1
            result = {"entry_id": f"2025-01-{entry_counter:03d}", "success": True}

        progress = progress.resume(return_value=result)

    # Check unbalanced journal_add entries
    if pending_entries:
        total_debit = sum(e.get("debit", 0) for e in pending_entries)
        total_credit = sum(e.get("credit", 0) for e in pending_entries)
        if abs(total_debit - total_credit) > 0.001:
            violations.append(InvariantViolation(
                "balanced_entries", f"Unbalanced entries: debits={total_debit}, credits={total_credit}",
                {"total_debit": total_debit, "total_credit": total_credit}))

    return DryRunResult(
        success=len(violations) == 0,
        actions=actions,
        violations=violations,
        output=progress.output,
    )


# ============================================================
# Stage 3: Behavioral Diff
# ============================================================

@dataclass
class BehaviorChange:
    category: str  # "added", "removed", "changed"
    description: str


def behavioral_diff(old_script: str, new_script: str) -> list[BehaviorChange]:
    """Run old and new agent against same data, diff the behavior."""
    old_result = dry_run(old_script)
    new_result = dry_run(new_script)

    changes = []

    # Compare primitive call counts
    old_counts = {}
    new_counts = {}
    for a in old_result.actions:
        old_counts[a.primitive] = old_counts.get(a.primitive, 0) + 1
    for a in new_result.actions:
        new_counts[a.primitive] = new_counts.get(a.primitive, 0) + 1

    all_primitives = set(list(old_counts.keys()) + list(new_counts.keys()))
    for p in sorted(all_primitives):
        old_n = old_counts.get(p, 0)
        new_n = new_counts.get(p, 0)
        if old_n == 0 and new_n > 0:
            changes.append(BehaviorChange("added", f"New primitive call: {p} (called {new_n}x)"))
        elif old_n > 0 and new_n == 0:
            changes.append(BehaviorChange("removed", f"Removed primitive call: {p} (was {old_n}x)"))
        elif old_n != new_n:
            changes.append(BehaviorChange("changed", f"{p}: {old_n}x → {new_n}x"))

    # Compare status distributions
    old_statuses = {}
    new_statuses = {}
    for a in old_result.actions:
        if a.primitive in ("journal_add", "journal_add_double"):
            s = a.kwargs.get("status", "unknown")
            old_statuses[s] = old_statuses.get(s, 0) + 1
    for a in new_result.actions:
        if a.primitive in ("journal_add", "journal_add_double"):
            s = a.kwargs.get("status", "unknown")
            new_statuses[s] = new_statuses.get(s, 0) + 1

    for status in sorted(set(list(old_statuses.keys()) + list(new_statuses.keys()))):
        old_n = old_statuses.get(status, 0)
        new_n = new_statuses.get(status, 0)
        if old_n != new_n:
            if old_n == 0:
                changes.append(BehaviorChange("added", f"New status '{status}': {new_n} entries"))
            elif new_n == 0:
                changes.append(BehaviorChange("removed", f"No more '{status}' entries (was {old_n})"))
            else:
                pct = ((new_n - old_n) / old_n) * 100
                direction = "more" if pct > 0 else "fewer"
                changes.append(BehaviorChange(
                    "changed",
                    f"'{status}' entries: {old_n} → {new_n} ({abs(pct):.0f}% {direction})",
                ))

    # Check for new invariant violations
    if not old_result.violations and new_result.violations:
        for v in new_result.violations:
            changes.append(BehaviorChange("added", f"NEW INVARIANT VIOLATION: {v.rule} — {v.message}"))

    # Queue changes
    old_queue = sum(1 for a in old_result.actions if a.primitive == "queue_add_review")
    new_queue = sum(1 for a in new_result.actions if a.primitive == "queue_add_review")
    if old_queue != new_queue:
        if new_queue > old_queue * 2 and old_queue > 0:
            changes.append(BehaviorChange(
                "changed",
                f"REVIEW QUEUE SPIKE: {old_queue} → {new_queue} items ({new_queue/old_queue:.1f}x increase)",
            ))

    return changes


def format_diff(changes: list[BehaviorChange]) -> str:
    """Format behavioral diff as human-readable text."""
    if not changes:
        return "No behavioral changes detected."
    lines = ["Behavioral changes detected:", ""]
    for c in changes:
        icon = {"added": "+", "removed": "-", "changed": "~"}[c.category]
        lines.append(f"  [{icon}] {c.description}")
    return "\n".join(lines)


# ============================================================
# Full Pipeline
# ============================================================

@dataclass
class PipelineResult:
    static_issues: list[ValidationIssue]
    dry_run_result: DryRunResult = None
    diff_changes: list[BehaviorChange] = None
    passed: bool = False


def validate_agent(script: str, old_script: str = None) -> PipelineResult:
    """Run full validation pipeline on an agent script."""
    result = PipelineResult(static_issues=[])

    # Stage 1: Static validation
    result.static_issues = static_validate(script)
    static_errors = [i for i in result.static_issues if i.severity == "error"]
    if static_errors:
        return result

    # Stage 2: Dry run
    try:
        result.dry_run_result = dry_run(script)
    except Exception as e:
        result.dry_run_result = DryRunResult(
            success=False, actions=[], violations=[], output=None,
            error=str(e))
        return result

    # Stage 3: Behavioral diff (if old script provided)
    if old_script:
        try:
            result.diff_changes = behavioral_diff(old_script, script)
        except Exception as e:
            result.diff_changes = [BehaviorChange("added", f"Diff failed: {e}")]

    result.passed = (
        len(static_errors) == 0
        and result.dry_run_result.success
    )
    return result
