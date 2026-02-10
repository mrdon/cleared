"""
4 agent scripts generated from the system prompt + stubs only.
Each is what an LLM would produce given the prompt.
"""

# --- Prompt 1: "Write an agent that imports CSV files and classifies transactions" ---
INGEST_AGENT = '''
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
        txns = importer_deduplicate(txns)
        ctx_log(f"Processing {f['name']}: {len(txns)} new transactions")

        for txn in txns:
            match = rules_match(description=txn["description"], amount=txn["amount"])

            if match and match["confidence"] >= threshold:
                if txn["amount"] < 0:
                    journal_add_double(
                        date=txn["date"],
                        description=txn["description"],
                        debit_account=match["account_id"],
                        credit_account=1010,
                        amount=abs(txn["amount"]),
                        counterparty=match["vendor_name"],
                        reference=txn["reference"],
                        confidence=match["confidence"],
                        status="auto-confirmed",
                        evidence="rule match: " + match["pattern"],
                    )
                else:
                    journal_add_double(
                        date=txn["date"],
                        description=txn["description"],
                        debit_account=1010,
                        credit_account=4010,
                        amount=txn["amount"],
                        counterparty=match["vendor_name"],
                        reference=txn["reference"],
                        confidence=match["confidence"],
                        status="auto-confirmed",
                        evidence="rule match: " + match["pattern"],
                    )
                total_confirmed = total_confirmed + 1
            else:
                if txn["amount"] < 0:
                    journal_add_double(
                        date=txn["date"],
                        description=txn["description"],
                        debit_account=5030,
                        credit_account=1010,
                        amount=abs(txn["amount"]),
                        reference=txn["reference"],
                        confidence=0.0,
                        status="pending-review",
                        evidence="no confident match",
                    )
                else:
                    journal_add_double(
                        date=txn["date"],
                        description=txn["description"],
                        debit_account=1010,
                        credit_account=4010,
                        amount=txn["amount"],
                        reference=txn["reference"],
                        confidence=0.0,
                        status="pending-review",
                        evidence="no confident match",
                    )
                queue_add_review(
                    entry_id="pending",
                    description=txn["description"],
                    suggested_account=match["account_id"] if match else 0,
                    confidence=match["confidence"] if match else 0.0,
                )
                total_review = total_review + 1

            total_imported = total_imported + 1

        importer_mark_processed(f["name"])

    git_commit("import: " + str(len(files)) + " files, " + str(total_imported) + " transactions")
    ctx_log(f"Done: {total_confirmed} auto-confirmed, {total_review} for review")
    {"imported": total_imported, "confirmed": total_confirmed, "review": total_review}
'''

# --- Prompt 2: "Write an agent that analyzes corrections to learn new rules" ---
LEARNING_AGENT = '''
corrections = journal_query(status="user-corrected")
if not corrections:
    ctx_log("No corrections to learn from")
    {"new_rules": 0}
else:
    existing_rules = rules_list()
    existing_patterns = [r["pattern"] for r in existing_rules]

    # Group corrections by description to find patterns
    pattern_groups = {}
    for entry in corrections:
        desc = entry["description"].upper()
        # Use first word as a simple pattern key
        words = desc.split()
        if words:
            key = words[0]
            if key not in pattern_groups:
                pattern_groups[key] = []
            pattern_groups[key] = pattern_groups[key] + [entry]

    new_rules = 0
    for key, entries in pattern_groups.items():
        if len(entries) < 2:
            ctx_log(f"Skipping '{key}': only {len(entries)} occurrence(s)")
        else:
            # Check if all corrections point to the same account
            accounts = [e["account_id"] for e in entries]
            first_account = accounts[0]
            all_same = True
            for a in accounts:
                if a != first_account:
                    all_same = False

            if all_same:
                pattern = key + "*"
                if pattern in existing_patterns:
                    ctx_log(f"Rule already exists for '{pattern}'")
                else:
                    vendor = entries[0].get("counterparty", key)
                    rules_add(
                        pattern=pattern,
                        vendor_name=vendor,
                        account_id=first_account,
                        confidence=0.90,
                        source="learned_from_corrections",
                    )
                    ctx_log(f"New rule: {pattern} -> account {first_account} (from {len(entries)} corrections)")
                    new_rules = new_rules + 1
            else:
                ctx_log(f"Skipping '{key}': corrections point to different accounts")

    if new_rules > 0:
        git_commit("learn: " + str(new_rules) + " new rules from user corrections")

    {"new_rules": new_rules, "patterns_analyzed": len(pattern_groups)}
'''

# --- Prompt 3: "Modify ingest agent to emit event and track counts" ---
MODIFIED_INGEST = '''
files = importer_scan()
if not files:
    ctx_log("No new files")
    {"imported": 0, "confirmed": 0, "review": 0}
else:
    total_confirmed = 0
    total_review = 0

    for f in files:
        txns = importer_parse(f["name"])
        txns = importer_deduplicate(txns)
        for txn in txns:
            match = rules_match(description=txn["description"], amount=txn["amount"])
            if match and match["confidence"] >= config_get("thresholds.auto_confirm"):
                journal_add_double(date=txn["date"], description=txn["description"],
                    debit_account=match["account_id"], credit_account=1010,
                    amount=abs(txn["amount"]), counterparty=match["vendor_name"],
                    reference=txn["reference"], confidence=match["confidence"],
                    status="auto-confirmed", evidence="rule match: " + match["pattern"])
                total_confirmed = total_confirmed + 1
            else:
                journal_add_double(date=txn["date"], description=txn["description"],
                    debit_account=5030, credit_account=1010,
                    amount=abs(txn["amount"]), reference=txn["reference"],
                    confidence=0.0, status="pending-review", evidence="no match")
                queue_add_review(entry_id="pending", description=txn["description"],
                    confidence=0.0)
                total_review = total_review + 1
        importer_mark_processed(f["name"])

    git_commit("import: processed " + str(len(files)) + " files")

    total = total_confirmed + total_review
    ctx_log(f"Import complete: {total} transactions ({total_confirmed} auto-confirmed, {total_review} for review)")

    ctx_emit("import_complete", {
        "files": len(files),
        "total": total,
        "confirmed": total_confirmed,
        "review": total_review,
    })

    {"imported": total, "confirmed": total_confirmed, "review": total_review}
'''

# --- Prompt 4: "Fix the comma-splitting bug" ---
FIXED_INGEST = '''
files = importer_scan()
total = 0
confirmed = 0
review = 0

for f in files:
    txns = importer_parse(f["name"])
    for txn in txns:
        description = txn["description"]
        match = rules_match(description=description, amount=txn["amount"])

        if match and match["confidence"] >= config_get("thresholds.auto_confirm"):
            if txn["amount"] < 0:
                journal_add_double(
                    date=txn["date"],
                    description=description,
                    debit_account=match["account_id"],
                    credit_account=1010,
                    amount=abs(txn["amount"]),
                    counterparty=match["vendor_name"],
                    reference=txn["reference"],
                    confidence=match["confidence"],
                    status="auto-confirmed",
                    evidence="rule match: " + match["pattern"],
                )
                confirmed = confirmed + 1
            else:
                journal_add_double(
                    date=txn["date"],
                    description=description,
                    debit_account=1010,
                    credit_account=4010,
                    amount=txn["amount"],
                    counterparty=match["vendor_name"],
                    reference=txn["reference"],
                    confidence=match["confidence"],
                    status="auto-confirmed",
                    evidence="rule match: " + match["pattern"],
                )
                confirmed = confirmed + 1
        else:
            if txn["amount"] < 0:
                journal_add_double(
                    date=txn["date"],
                    description=description,
                    debit_account=5030,
                    credit_account=1010,
                    amount=abs(txn["amount"]),
                    reference=txn["reference"],
                    status="pending-review",
                    evidence="no confident match",
                )
            else:
                journal_add_double(
                    date=txn["date"],
                    description=description,
                    debit_account=1010,
                    credit_account=4010,
                    amount=txn["amount"],
                    reference=txn["reference"],
                    status="pending-review",
                    evidence="no confident match",
                )
            queue_add_review(
                entry_id="pending",
                description=description,
                suggested_account=match["account_id"] if match else 0,
                confidence=match["confidence"] if match else 0.0,
            )
            review = review + 1

        total = total + 1

    importer_mark_processed(f["name"])

if total > 0:
    git_commit("import: " + str(total) + " transactions")

ctx_log(f"Processed {total} transactions: {confirmed} confirmed, {review} for review")
{"total": total, "confirmed": confirmed, "review": review}
'''

ALL_AGENTS = [
    ("Ingest Agent", INGEST_AGENT),
    ("Learning Agent", LEARNING_AGENT),
    ("Modified Ingest (emit events)", MODIFIED_INGEST),
    ("Fixed Ingest (no comma split)", FIXED_INGEST),
]
