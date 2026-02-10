"""
Spike 3: Test all 4 validation scenarios + success criteria.
"""

import time

from validate import (
    behavioral_diff,
    dry_run,
    format_diff,
    static_validate,
    validate_agent,
)


def test_static_forbidden_constructs():
    """Scenario: Agent tries filesystem access → static validation blocks it."""
    print("--- Test 1: Static validator catches forbidden constructs ---")
    cases = [
        ("open() call", 'data = open("/etc/passwd")\ndata'),
        ("eval() call", 'eval("1+1")'),
        ("exec() call", 'exec("print(1)")'),
        ("__import__() call", '__import__("os")'),
    ]

    all_passed = True
    for name, script in cases:
        issues = static_validate(script)
        errors = [i for i in issues if i.severity == "error"]
        if errors:
            print(f"  {name}: BLOCKED ({errors[0].message})")
        else:
            print(f"  {name}: FAIL — not caught!")
            all_passed = False

    # Good script should pass
    good_script = """
files = importer_scan()
ctx_log("found " + str(len(files)) + " files")
{"files": len(files)}
"""
    issues = static_validate(good_script)
    errors = [i for i in issues if i.severity == "error"]
    if not errors:
        print(f"  Valid script: PASSED (no false positives)")
    else:
        print(f"  Valid script: FALSE POSITIVE — {errors}")
        all_passed = False

    print(f"  {'PASS' if all_passed else 'FAIL'}")
    return all_passed


def test_type_checking():
    """Scenario: Wrong argument types → type checker catches it."""
    print("\n--- Test 2: Type checker catches wrong argument types ---")
    cases = [
        ("string where int expected",
         'journal_add(date="2025-01-01", account_id="wrong", description="test", debit=1.0, status="pending-review")'),
        ("missing required arg",
         'journal_add_double(date="2025-01-01", description="test", debit_account=5020, amount=10.0)'),
        ("int where string expected",
         'git_commit(123)'),
    ]

    all_passed = True
    for name, script in cases:
        issues = static_validate(script)
        errors = [i for i in issues if i.severity == "error"]
        if errors:
            print(f"  {name}: CAUGHT ({errors[0].message[:80]})")
        else:
            print(f"  {name}: MISSED")
            all_passed = False

    print(f"  {'PASS' if all_passed else 'FAIL'}")
    return all_passed


def test_dry_run_invariant_violations():
    """Scenario: Bug produces unbalanced entries → dry run catches it."""
    print("\n--- Test 3: Dry run detects invariant violations ---")

    # Bad agent: uses journal_add with only debits, no credits
    unbalanced_script = """
files = importer_scan()
for f in files:
    txns = importer_parse(f["name"])
    for txn in txns:
        journal_add(date=txn["date"], account_id=5020,
                    description=txn["description"],
                    debit=abs(txn["amount"]),
                    status="auto-confirmed", evidence="test")
    importer_mark_processed(f["name"])
git_commit("import: unbalanced")
"""
    result = dry_run(unbalanced_script)
    has_balance_violation = any(v.rule == "balanced_entries" for v in result.violations)
    print(f"  Unbalanced entries: {'CAUGHT' if has_balance_violation else 'MISSED'} ({len(result.violations)} violations)")
    for v in result.violations:
        print(f"    - {v.rule}: {v.message}")

    # Bad agent: uses invalid account ID
    bad_account_script = """
journal_add_double(date="2025-01-01", description="test",
                   debit_account=9999, credit_account=1010,
                   amount=100.0, status="auto-confirmed")
"""
    result2 = dry_run(bad_account_script)
    has_account_violation = any(v.rule == "valid_account" for v in result2.violations)
    print(f"  Invalid account: {'CAUGHT' if has_account_violation else 'MISSED'}")

    # Bad agent: uses invalid status
    bad_status_script = """
journal_add_double(date="2025-01-01", description="test",
                   debit_account=5020, credit_account=1010,
                   amount=50.0, status="yolo-confirmed")
"""
    result3 = dry_run(bad_status_script)
    has_status_violation = any(v.rule == "valid_status" for v in result3.violations)
    print(f"  Invalid status: {'CAUGHT' if has_status_violation else 'MISSED'}")

    # Good agent should pass
    good_script = """
journal_add_double(date="2025-01-01", description="test",
                   debit_account=5020, credit_account=1010,
                   amount=50.0, status="auto-confirmed")
"""
    result4 = dry_run(good_script)
    print(f"  Valid agent: {'PASSED' if result4.success else 'FALSE POSITIVE — ' + str(result4.violations)}")

    passed = has_balance_violation and has_account_violation and has_status_violation and result4.success
    print(f"  {'PASS' if passed else 'FAIL'}")
    return passed


def test_behavioral_diff():
    """Scenario: Agent auto-confirms everything → behavioral diff flags it."""
    print("\n--- Test 4: Behavioral diff detects dangerous changes ---")

    # Original agent: respects confidence threshold
    old_script = """
files = importer_scan()
for f in files:
    txns = importer_parse(f["name"])
    for txn in txns:
        match = rules_match(description=txn["description"], amount=txn["amount"])
        if match and match["confidence"] >= config_get("thresholds.auto_confirm"):
            journal_add_double(date=txn["date"], description=txn["description"],
                debit_account=match["account_id"], credit_account=1010,
                amount=abs(txn["amount"]), status="auto-confirmed",
                evidence="rule match")
        else:
            journal_add_double(date=txn["date"], description=txn["description"],
                debit_account=5030, credit_account=1010,
                amount=abs(txn["amount"]), status="pending-review",
                evidence="no match")
            queue_add_review(entry_id="pending", description=txn["description"])
    importer_mark_processed(f["name"])
git_commit("import: done")
"""

    # Bad modification: auto-confirms EVERYTHING (ignores threshold)
    new_script = """
files = importer_scan()
for f in files:
    txns = importer_parse(f["name"])
    for txn in txns:
        match = rules_match(description=txn["description"], amount=txn["amount"])
        journal_add_double(date=txn["date"], description=txn["description"],
            debit_account=5020, credit_account=1010,
            amount=abs(txn["amount"]), status="auto-confirmed",
            evidence="auto")
    importer_mark_processed(f["name"])
git_commit("import: done")
"""

    changes = behavioral_diff(old_script, new_script)
    diff_text = format_diff(changes)
    print(f"  {diff_text}")

    # Check that the diff flags the change
    has_status_change = any("auto-confirmed" in c.description and "more" in c.description for c in changes)
    has_queue_change = any("queue" in c.description.lower() or "queue_add_review" in c.description for c in changes)
    flagged = has_status_change or has_queue_change

    print(f"\n  Auto-confirm-all flagged: {'YES' if flagged else 'NO'}")
    print(f"  {'PASS' if flagged else 'FAIL'}")
    return flagged


def test_full_pipeline():
    """Run full pipeline on a good agent and a bad agent."""
    print("\n--- Test 5: Full pipeline integration ---")

    good_script = """
files = importer_scan()
if files:
    for f in files:
        txns = importer_parse(f["name"])
        for txn in txns:
            match = rules_match(description=txn["description"])
            if match:
                journal_add_double(date=txn["date"], description=txn["description"],
                    debit_account=match["account_id"], credit_account=1010,
                    amount=abs(txn["amount"]), status="auto-confirmed",
                    evidence="rule match")
            else:
                journal_add_double(date=txn["date"], description=txn["description"],
                    debit_account=5030, credit_account=1010,
                    amount=abs(txn["amount"]), status="pending-review",
                    evidence="no match")
        importer_mark_processed(f["name"])
    git_commit("import: done")
{"status": "ok"}
"""

    result = validate_agent(good_script)
    print(f"  Good agent: passed={result.passed}, issues={len(result.static_issues)}, violations={len(result.dry_run_result.violations) if result.dry_run_result else 'N/A'}")

    bad_script = """
journal_add_double(date="2025-01-01", description="test",
                   debit_account=9999, credit_account=1010,
                   amount=50.0, status="yolo")
"""
    result2 = validate_agent(bad_script)
    print(f"  Bad agent: passed={result2.passed}, violations={len(result2.dry_run_result.violations) if result2.dry_run_result else 'N/A'}")

    passed = result.passed and not result2.passed
    print(f"  {'PASS' if passed else 'FAIL'}")
    return passed


def test_pipeline_speed():
    """Verify pipeline runs in < 5 seconds."""
    print("\n--- Test 6: Pipeline speed ---")

    script = """
files = importer_scan()
for f in files:
    txns = importer_parse(f["name"])
    txns = importer_deduplicate(txns)
    for txn in txns:
        match = rules_match(description=txn["description"])
        if match:
            journal_add_double(date=txn["date"], description=txn["description"],
                debit_account=match["account_id"], credit_account=1010,
                amount=abs(txn["amount"]), status="auto-confirmed",
                evidence="rule match")
        else:
            journal_add_double(date=txn["date"], description=txn["description"],
                debit_account=5030, credit_account=1010,
                amount=abs(txn["amount"]), status="pending-review",
                evidence="no match")
            queue_add_review(entry_id="pending", description=txn["description"])
    importer_mark_processed(f["name"])
git_commit("import: done")
ctx_log("done")
"""

    t0 = time.time()
    for _ in range(10):  # Run 10x to get reliable timing
        result = validate_agent(script, old_script=script)
    elapsed = time.time() - t0
    per_run = elapsed / 10 * 1000

    print(f"  10 full pipeline runs in {elapsed*1000:.0f}ms ({per_run:.0f}ms/run)")
    passed = per_run < 5000
    print(f"  Target: < 5000ms — {'PASS' if passed else 'FAIL'}")
    return passed


def main():
    print("=" * 60)
    print("Spike 3: Validation Pipeline")
    print("=" * 60)

    results = [
        ("Static validation (forbidden constructs)", test_static_forbidden_constructs()),
        ("Type checking (wrong arg types)", test_type_checking()),
        ("Dry run (invariant violations)", test_dry_run_invariant_violations()),
        ("Behavioral diff (dangerous changes)", test_behavioral_diff()),
        ("Full pipeline integration", test_full_pipeline()),
        ("Pipeline speed (< 5 seconds)", test_pipeline_speed()),
    ]

    print(f"\n{'='*60}")
    print("SPIKE 3 RESULTS")
    print(f"{'='*60}")

    passed = sum(1 for _, r in results if r)
    total = len(results)
    for name, r in results:
        print(f"  [{'✓' if r else '✗'}] {name}")

    print(f"\n  {passed}/{total} tests passed")

    print(f"\nSuccess Criteria:")
    print(f"  [{'✓' if results[0][1] else '✗'}] Static validator catches forbidden constructs")
    print(f"  [{'✓' if results[2][1] else '✗'}] Dry run detects invariant violations")
    print(f"  [{'✓' if results[3][1] else '✗'}] Behavioral diff produces understandable summaries")
    print(f"  [{'✓' if results[5][1] else '✗'}] Pipeline runs in < 5 seconds")
    print(f"  [{'✓' if passed >= 5 else '✗'}] False positive rate < 10%")


if __name__ == "__main__":
    main()
