# Scorecard total and stored rows: transaction visibility

- Date: 2026-11-03 (America/Toronto).
- Primary domain: scorecard-workflows; subdomain: evaluation-and-scoring.
- Source repo/worktree: /Users/xuanyu.wang/repos/go-servers, main at 35c364ff698e86e4d8eef77f4504eb85dc76c5cc.
- Request: explain whether reading/merging/recalculating inside a transaction prevents the supplied overlapping A/B update scenario. Explanation only; no product code changes or database access.

## Source evidence

- `apiserver/internal/coaching/action_update_scorecard.go`: `updateScorecardAndScoresAtomic` calls `prepareUpdatedScorecardAndScores` before `transaction.RunTransaction`. Preparation calls `ComputeScores` on request scores and separately fetches existing rows.
- `shared/scoring/scorecard_scores_dao.go`: `UpdateScorecardAndScoresInDB` saves the precomputed parent before merging/writing criterion rows. `mergeScoresData` can preserve an existing valid criterion omitted from the new scores.
- `shared/scoring/scorecard_calculator.go`: `ComputeScores` builds overall and chapter summaries from supplied scores, skipping absent criteria.

## Explanation and conclusion

Under PostgreSQL READ COMMITTED, each ordinary SELECT sees committed data as of that statement's start, plus its transaction's own prior writes. BEGIN does not freeze the database, and an ordinary SELECT does not block concurrent writers. Previously returned application objects do not refresh when another transaction commits.

Reading/merging/recalculating inside a transaction fixes the supplied case if B commits before A reads the scores. It does not guarantee consistency if A reads Q1-Q3, B commits Q4 and its total, and A later saves a total calculated from its old read while preserving Q4. A write lock acquired at the final parent UPDATE is too late to refresh the earlier calculation.

A concrete stronger proposal is BEGIN, lock the existing parent scorecard with SELECT FOR UPDATE, read current criterion rows, merge the request using the intended omission/deletion rules, compute total and chapter summaries from the final effective criteria, persist everything, COMMIT. Under READ COMMITTED a waiting updater reads scores in a subsequent statement after acquiring the parent lock, seeing the prior updater's committed changes. All paths changing scores/totals must follow the same parent-lock protocol. Locking existing child rows alone cannot protect a missing Q4 row.

REPEATABLE READ preserves a transaction snapshot but is not a replacement for this protocol; conflicts may abort. SERIALIZABLE with retries of the full read/merge/calculate/write operation is another possible design. Actual deployed isolation was not queried. No concurrency reproduction or implementation was performed.

This keeps totals consistent with persisted rows; rejecting older requests or preventing stale overlapping answers from overwriting newer answers is a separate product decision.

## Validation and follow-up

- Evidence: current local source inspection and explicit A/B statement timelines; no live database queries or credentials used.
- If implemented later, test both B-commits-before-A-reads and A-reads-before-B-commits schedules, total/chapter consistency, and omitted versus explicitly cleared criteria.
