# Debugging a Dual-Database Sync Bug: Three Fixes, Two Race Conditions, and the Trap of Async Side Effects

When your system writes to two databases — a relational DB as source of truth and a columnar analytics store — "eventual consistency" can hide subtle, compounding bugs. This post walks through a real investigation where customer-reported data inconsistencies led to discovering not one but two race conditions, a failed first fix, and ultimately a principled approach to verification.

From a staff engineering perspective, the interesting parts aren't the fixes themselves (which are small) — they're the investigation methodology, the layered nature of the bugs, and the discipline of validating assumptions against production data.

## The architecture

The system has a simple dual-write pattern:

1. **API request** arrives (e.g., update a record, submit a record)
2. **Synchronous work**: write to PostgreSQL inside a transaction
3. **Asynchronous work**: fire-and-forget goroutine updates ClickHouse (columnar analytics DB)

ClickHouse uses [ReplacingMergeTree](https://clickhouse.com/docs/en/engines/table-engines/mergetree-family/replacingmergetree), which deduplicates rows by keeping the one with the highest `version` column. In our case, `version` = `update_time`, set to `time.Now()` at the moment of the ClickHouse write.

PostgreSQL is the source of truth. ClickHouse powers dashboards and analytics. The expectation: they should agree.

## The symptom

Customers reported that analytics dashboards showed incorrect data — records appeared unsubmitted or had stale scores, even though the primary application showed the correct state.

## Bug #1: Async work runs out of order

The system has two related APIs that users call in sequence:

- **UpdateRecord**: saves scores/data to the record
- **SubmitRecord**: marks the record as finalized (sets `submitted_at`, `submitter_id`)

Both APIs follow the same sync-then-async pattern. The problem: async goroutines don't finish in the order they were started.

```
Timeline (failure scenario):
────────────────────────────────────────────────────────
Time    UpdateRecord                SubmitRecord
────────────────────────────────────────────────────────
T1      API called
T2      PG transaction commits
T3      Async goroutine starts
T4      Reads PG (submitted_at=NULL)
T5                                  API called
T6                                  PG transaction commits
T7                                  Async goroutine starts
T8                                  Reads PG (submitted_at=correct)
T9                                  Writes to CH (update_time=T9)
T10     Writes to CH (update_time=T10, stale data!)
        ^ T10 > T9, so ReplacingMergeTree keeps stale row
────────────────────────────────────────────────────────
```

The Update goroutine reads from PG *before* Submit's transaction commits (so it sees no submission), but writes to ClickHouse *after* Submit's goroutine finishes (so its `time.Now()` is later). ReplacingMergeTree keeps the row with the higher `update_time` — the stale one.

### Fix attempt 1: Use PG `updated_at` as the ClickHouse version (failed)

The intuition: if we use PG's `updated_at` as ClickHouse's `update_time` instead of `time.Now()`, the ordering would be correct because Submit's `updated_at` is always later than Update's.

This failed because **the async work captured the record object in a closure**. When the goroutine ran, it used the stale snapshot from before the transaction — including the stale `updated_at`. Changing the timestamp source didn't help because the data itself was stale.

**Lesson: The root cause was reading stale data, not the timestamp source.** The failed fix addressed a symptom while the real problem persisted.

### Fix 2: Atomic transactions + re-read from DB (merged)

The real fix had two parts:

1. **Move analytics-schema writes inside the transaction**, so they're committed atomically with the primary data
2. **Make async work re-read from the database** instead of using closure-captured data

```go
func UpdateRecordAtomic(...) {
    var asyncWork func()

    db.Transaction(func(tx) {
        // Primary write
        UpdateRecordInDB(tx, ...)
        // Analytics schema write (now inside transaction)
        WriteAnalyticsData(tx, ...)

        // Prepare async work — will execute AFTER commit
        asyncWork = func() {
            // Re-read from DB (write replica), not closure
            record := ReadFromDB(recordID)
            WriteToClickHouse(record)
        }
    })

    asyncWorkQueue.Execute(asyncWork)
}
```

This ensures the ClickHouse write always reflects the latest committed state. Even if goroutines run out of order, both re-read from PG and see the same committed data.

## Bug #2: PostgreSQL lost update (discovered during testing)

While load-testing Fix 2, a second race condition emerged — this time in PostgreSQL itself.

The ORM used `Save()` which persists the **entire struct**, not just modified fields. When Update and Submit run concurrently:

```
T1  UpdateRecord: reads record (submitted_at=NULL)
T2  SubmitRecord: reads record (submitted_at=NULL)
T3  SubmitRecord: sets submitted_at=NOW(), saves entire struct
T4  UpdateRecord: modifies scores, saves entire struct
    ^ Overwrites submitted_at back to NULL!
```

**Fix 3: Partial updates.** Use the ORM's field-exclusion mechanism (`Omit` in GORM) so UpdateRecord never writes to `submitted_at` or `submitter_id`:

```go
tx.Model(record).Omit("submitted_at", "submitter_id").Save(record)
```

This is a classic lost-update problem, but it was masked by the async ClickHouse bug — both bugs produced the same symptom (missing submission data), making it easy to assume a single root cause.

## Validation: load testing and production verification

### Load testing

Built a custom load test tool that:
1. Creates a record
2. Calls Update and Submit with configurable delay between them
3. Waits for ClickHouse convergence
4. Compares PG vs CH data

Results with both fixes applied:

| API delay | Success rate |
|-----------|-------------|
| 10ms      | ~80%        |
| 50ms      | 94%         |
| 100ms+    | ~100%       |

The ~100ms threshold represents the time for the Update goroutine's async work to complete (DB re-read + ClickHouse write). Beyond that, Submit's goroutine always writes last and wins.

### Production verification

After gradual rollout behind a feature flag, verified on production across ~3,000 submitted records over 39 days:

| Metric | Result |
|--------|--------|
| Score mismatches (PG vs CH) | **0** |
| Submitter mismatches | **0** |
| CH missing submission state | 26 (0.87%) |

The 0.87% residual: users who clicked Save then Submit in quick succession in the UI, triggering the async race. All 26 had correct data in PostgreSQL — only the ClickHouse analytics copy was stale. Acceptable for the use case.

### Closing the loop

An initial assumption was that the 0.87% residual came from automated batch processing (which would call Update + Submit programmatically with minimal delay). Code review disproved this: the batch code path writes to PG and ClickHouse synchronously in a single function — no async goroutines, no race possible. The actual trigger was human users interacting quickly in the UI.

This correction matters: if the cause were automated, we'd need to add delays to the batch pipeline. Since it's human UI interaction, the ~1% rate is inherently bounded and acceptable.

## Staff-level reflections

### 1. Investigation beats intuition

The first fix failed because it was based on an intuitive model ("the timestamp is wrong") rather than a precise understanding of data flow ("the closure captures stale state"). Tracing the exact sequence of reads and writes — which goroutine reads what, when — was what led to the correct fix.

### 2. Multiple bugs can produce identical symptoms

The async ordering bug and the PostgreSQL lost-update bug both caused the same observable symptom: missing submission data. If we'd stopped after fixing one, the other would have continued causing intermittent failures. Load testing with high concurrency was essential for separating the two.

### 3. `time.Now()` as a version column is a design smell

Using wall-clock time as the ReplacingMergeTree version means **write order determines truth, not data order**. A goroutine that reads stale data but writes late will "win." A more robust design would use a monotonic version derived from the source-of-truth database (e.g., a sequence number or the PG transaction's `updated_at`). We considered this but it requires ensuring the async work always reads fresh data — which is exactly what Fix 2 addresses. The two approaches are complementary.

### 4. Feature flags and staged rollout pay for themselves

The fix touched four API code paths. Rolling out behind a feature flag let us:
- Enable on staging first, run load tests
- Enable on one production customer, verify with real traffic
- Roll out globally only after production data confirmed correctness
- Eventually remove the flag and 740 lines of legacy code with confidence

### 5. Verify your assumptions against the actual code path

"92% of affected records came from automated scoring" sounded like a strong signal — until we read the code and discovered that (a) the enum value we assumed meant "auto-scored" actually meant "submitted from a specific UI page," and (b) the automated scoring code path doesn't use the affected APIs at all. Production data analysis is powerful, but you must ground it in the actual code to avoid misleading conclusions.

## The general pattern

If your system does sync-write-to-DB then async-side-effect-to-another-store:

1. **Never capture mutable state in async closures.** Re-read from the source of truth.
2. **Use partial updates.** Full-struct saves (`Save()`) are lost-update bugs waiting to happen.
3. **Your version column strategy matters.** `time.Now()` at write time means write order = version order, which may not reflect data order.
4. **Build verification tooling early.** A tool that compares the two stores for a single record is invaluable for both debugging and production validation.
5. **Document the acceptable residual.** After fixing what you can, quantify and document what remains. "0.87% of records have stale analytics data under specific UI interaction patterns" is a much better state than "sometimes the data is wrong."
