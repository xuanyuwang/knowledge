# CONVI-7533: HCD outlier-scorecard deletion validation

**Status:** complete
**Primary domain:** `analytics`
**Primary subdomain:** `qa-score`
**Official ticket:** [CONVI-7533](https://linear.app/cresta/issue/CONVI-7533/request-to-delete-outlier-scorecards-to-fix-monthly-view)
**Last updated:** 2026-08-27

## Objective and Impact

- **Objective:** Identify the four historical weight-1 scorecards implicated by CONVI-7254 and verify, using the production aggregation formula, whether excluding them corrects the May and February monthly criterion values.
- **Customer/system impact:** Determines whether a narrowly scoped deletion is a safe remediation for HCD Performance Insights.
- **Role:** validated

## Scope

**In scope**

- Read-only identification of the three May and one February scorecards.
- Exact production-formula simulation with those IDs excluded.
- Evidence-backed update to the Linear ticket.

**Non-goals**

- Deleting scorecards or changing production data.
- Implementing a general mixed-revision aggregation fix.

## Source Context

- **Repos:** `go-servers`, `knowledge`
- **Worktrees:** `/Users/xuanyu.wang/repos/go-servers-convi-7254`
- **Branches:** `convi-7254-monthly-qa-score`
- **PRs/commits:** none
- **Linear comment:** [a982024d](https://linear.app/cresta/issue/CONVI-7533/request-to-delete-outlier-scorecards-to-fix-monthly-view#comment-a982024d)

## Current Understanding

The four approved scorecards have been deleted from PostgreSQL and ClickHouse. Exact-ID checks return zero source and projection rows. The production weighted formula now returns `1303/4747 = 27.45%` (May) and `93/427 = 21.78%` (February), with uniform remaining `float_weight=1e-13`. Performance Insights renders the corrected monthly cells as 27% for May and 22% for February, with chart points of 27.45% and 21.73% respectively. The complete pre-deletion PostgreSQL export remains backed up in the [Linear document](https://linear.app/cresta/document/convi-7533-pre-deletion-postgresql-backup-83fe3a3e16bf): 4 scorecards, 190 scores, and no coaching-plan join rows.

## Findings and Decisions

- Exactly four weight-1 rows exist; no duplicates and no additional weight-1 revisions on the two target criteria.
- May outliers are three unsubmitted auto scorecards on revision `33e46102`, owned by Melissa Crossin, Rabiah McCaskey, and Sheena Paris.
- February outlier is one unsubmitted manually scored draft on revision `8dd94019`, owned by Latrese Proctor and created/updated by Kathy Barretta.
- Production baseline matches prior API reproduction: May `4.343333332646079e-11` (0%), February `0.9999999999665996` (100%).
- Exclusion results: May `0.274489151042764` (27.45%), February `0.2177985948477751` (21.78%).
- HCD approved deleting all four IDs, including Latrese Proctor’s February draft, and explicitly accepted the four collateral February moves.

## Blockers and Dependencies

- None.

## Validation and Rollout

- Read-only ClickHouse (`home_care_delivered_us_east_1`) and PG (`hcd.director.scorecards`, `auth.users`) verification completed 2026-08-17.
- Pre-deletion backup manifest and full row capture are stored and checksummed in the [Linear deleted-data document](https://linear.app/cresta/document/convi-7533-pre-deletion-postgresql-backup-83fe3a3e16bf): 4 `director.scorecards` rows, 190 `director.scores` rows, no coaching-plan joins, and no blocking scorecard references.
- Pre-delete ClickHouse inventory: 4 `scorecard_d` rows and 187 `score_d` rows.
- PostgreSQL post-delete checks: zero matching `director.scorecards`, `director.scores`, or coaching-plan joins.
- ClickHouse post-delete checks: zero matching `scorecard_d`, `score_d`, raw `scorecard`, or raw `score` rows across all replicas.
- Post-delete aggregation: May `0.27448915104276395`; February `0.21779859484777508`.
- Performance Insights UI spot-check: May 1–31 monthly “Asking for phone number” renders 27% for 4,747 scorecards with chart point 27.45%; February 1–28 monthly “How did you hear about HCD?” renders 22% for 427 scorecards with chart point 21.73%.
- Final database results posted in CONVI-7533 comment `fef12e42`.
- Final UI results posted in CONVI-7533 comment `a3d58374` and relayed on CONVI-7254 in comment `8a0a96e4`; CONVI-7533 moved to Done.

## Next Actions

- No remaining execution actions. Treat future occurrences as part of the broader mixed-revision aggregation problem rather than repeating this deletion without separate approval.

## Timeline

- 2026-08-17 — Started targeted identification and exclusion simulation. Evidence: `sessions/2026-08-17/codex-convi-7533-deletion-validation.md`.
- 2026-08-17 — Completed read-only identification and production-formula exclusion. Replaced Linear comment a982024d with results. Evidence: `sessions/2026-08-17/codex-convi-7533-deletion-validation.md`.
- 2026-08-27 — Read customer approval for all four deletions, including accepted February collateral, and moved execution tracking to CONVI-7533 via comment `98f0e109`. Evidence: `sessions/2026-08-27/codex-convi-7533-approved-deletion.md`.
- 2026-08-27 — Preserved and reread the complete 98,845-byte pre-deletion PostgreSQL export in the linked Linear document using indexed comment chunks; source SHA-256 `d8f11bdbd997715ff0eef120bbf3d658090ea359a62219c9ff3adf059a46affd`. No production data was changed.
- 2026-08-27 — Completed PostgreSQL and ClickHouse deletion, verified zero matching rows across source/projections/all replicas, and reproduced 27.45% May / 21.78% February post-delete values. UI spot-check remains.
- 2026-08-27 — Verified the live PI monthly cells in Chrome: May renders 27% (chart 27.45%) and February renders 22% (chart 21.73%). Posted results to CONVI-7533 and CONVI-7254, then closed CONVI-7533 as Done.
