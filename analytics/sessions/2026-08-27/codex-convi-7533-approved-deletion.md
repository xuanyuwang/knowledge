# CONVI-7533 approved deletion handoff

## Context

- Main execution ticket: [CONVI-7533](https://linear.app/cresta/issue/CONVI-7533/request-to-delete-outlier-scorecards-to-fix-monthly-view)
- Approval source: [CONVI-7254 comment 39bb4b21](https://linear.app/cresta/issue/CONVI-7254/home-care-delivered-performance-insights-monthly-view-shows-0percent#comment-39bb4b21)
- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree context: `/Users/xuanyu.wang/repos/go-servers-convi-7254`
- Branch context: `convi-7254-monthly-qa-score`

## Approval

Vamsi Bhavana confirmed that HCD explicitly approved deleting all four scorecards, including Latrese Proctor’s February draft, and accepted the four previously documented collateral February criterion moves.

Approved scorecard IDs:

- `019e7502-3fd5-7528-94c6-e45c0b7e7448`
- `019e7503-2411-7e04-883f-6bea8930d41c`
- `019e7504-dc03-78a7-b70b-d0e751b4e725`
- `019c2e30-ba72-7ed4-a202-b3ea651eaeec`

## Tracking Update

Added CONVI-7533 comment `98f0e109-01b6-4253-acc1-c1888ca1e668`, making it the main execution tracker and recording the exact approved IDs and remaining validation steps.

No production deletion was performed in this session.

## Pre-deletion backup

- Updated the Linear document [CONVI-7533 pre-deletion PostgreSQL backup](https://linear.app/cresta/document/convi-7533-pre-deletion-postgresql-backup-83fe3a3e16bf).
- Captured source: PostgreSQL database `hcd`, 2026-08-27 before deletion.
- Preserved 4 `director.scorecards` rows and 190 `director.scores` rows; no coaching-plan join rows.
- Stored the complete 98,845-byte export as a Base64-encoded ZIP split across three indexed document comments, with source SHA-256 `d8f11bdbd997715ff0eef120bbf3d658090ea359a62219c9ff3adf059a46affd`.
- A fourth comment contains directly readable JSONL for the February scorecard and its 21 scores.
- Reread the document and all four comments after writing. Linear file uploads were disabled, so the comment-chunk fallback was used.
- No production data was deleted or mutated.

## Next Steps

1. Delete exactly the four approved scorecards through the approved deletion workflow.
2. Confirm `scorecard_d` and `score_d` latest-version projections no longer return them.
3. Re-run the exact monthly aggregation.
4. Spot-check the affected May and February Performance Insights cells.
5. Post final observed values on CONVI-7533 before closing.

## Pre-deletion backup and execution constraint

- Completed and verified the Linear document [CONVI-7533 pre-deletion PostgreSQL backup](https://linear.app/cresta/document/convi-7533-pre-deletion-postgresql-backup-83fe3a3e16bf).
- Captured 4 complete `director.scorecards` rows and 190 complete cascading `director.scores` rows as row-level JSON. There are no `director.coaching_plan_scorecards_join` rows and no scorecards referencing the targets through `calibrated_scorecard_id` or `reference_scorecard_id`. The 98,845-byte export is stored as a checksummed Base64 ZIP across indexed document comments.
- Added directly readable RFC 4180 CSV exports to five indexed document comments: one containing all 4 scorecards and four containing the 190 scores grouped 21/56/57/56 by scorecard. Reread verification confirmed all 33 scorecard columns, all 16 score columns, exact row counts, CRLF formatting, and per-payload SHA-256 checksums.
- Pre-delete ClickHouse inventory is 4 `scorecard_d` rows and 187 `score_d` rows across the four IDs.
- Code and operational-history review confirmed there is no public scorecard-instance deletion RPC or dedicated deletion CLI. `ResetScorecard` is unsuitable because all four targets have `ai_scored_at`; it would retain/nullify and re-project them rather than hard-delete them.
- The approved operational path remains the CONVI-6786 precedent: scorecard-workflows/ops deletes the approved IDs from `director.scorecards`, then removes or verifies the derived ClickHouse `score` and `scorecard` rows.
- The available app-DB skill is hard-coded and required to use `cresta-cli connstring --read-only`; its security requirements explicitly prohibit removing `--read-only` or bypassing the safety check. Production PostgreSQL mutation therefore requires an approved write-capable operational mechanism or a human/ops executor.
- Posted the guarded deletion transaction and execution handoff on CONVI-7533 in comment `2d043ade`.

## Deletion and validation result

- The user executed the guarded PostgreSQL transaction. Read-only verification returned zero matching rows in `director.scorecards`, `director.scores`, and `director.coaching_plan_scorecards_join`.
- Deleted the four approved IDs from ClickHouse raw `score` and `scorecard` on cluster `conversations`.
- Verified zero matching rows in `scorecard_d`, `score_d`, and raw `scorecard` / `score` across all replicas.
- Re-ran the exact production monthly aggregation:
  - May phone number: `0.27448915104276395` (27.45%, 1303 passing / 3444 failing).
  - February referral source: `0.21779859484777508` (21.78%, 93 passing / 334 failing).
- Both remaining populations have one distinct weight (`1e-13`).
- Posted database results to CONVI-7533 in comment `fef12e42`.
- Live Performance Insights spot-check completed in Chrome:
  - February 1–28, monthly, “How did you hear about HCD?”: 427 scorecards, rendered 22%, chart point 21.73%.
  - May 1–31, monthly, “Asking for phone number”: 4,747 scorecards, rendered 27%, chart point 27.45%.
- Posted final UI evidence to CONVI-7533 comment `a3d58374` and CONVI-7254 comment `8a0a96e4`.
- Moved CONVI-7533 to Done. No remaining execution action.
