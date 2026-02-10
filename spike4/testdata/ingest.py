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
                    confidence=0.0,
                )
                total_review = total_review + 1

            total_imported = total_imported + 1

        importer_mark_processed(f["name"])

    git_commit("import: " + str(total_imported) + " transactions from " + str(len(files)) + " files")
    ctx_log("Done: " + str(total_confirmed) + " auto-confirmed, " + str(total_review) + " for review")
    {"imported": total_imported, "confirmed": total_confirmed, "review": total_review}
