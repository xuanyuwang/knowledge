# CONVI-7378: Auto-scored scorecard changes not updating

**Status:** complete (diagnosed)
**Primary domain:** analytics
**Primary subdomain:** qa-score
**Official ticket:** [CONVI-7378](https://linear.app/cresta/issue/CONVI-7378/auto-scored-scorecard-changes-not-updating)
**Pattern:** [mixed-revision QA score semantics](../deliverables/mixed-revision-qa-score-semantics.md) — pinned-revision option-score inversion
**Report:** [SCAN Consent to Call investigation](../deliverables/convi-7378-scan-consent-qa-score-investigation.md)
**Last updated:** 2026-07-27

## Objective and Impact

- **Objective:** Determine whether SCAN scorecard manual overrides of auto-scored criteria are failing to propagate to Closed Conversations and Performance Insights.
- **Customer/system impact:** SCAN evaluators believe manual score changes are ignored. QA coaching and agent performance views lose trust when Yes/No labels disagree with displayed percentages.
- **Role:** diagnosed

## Scope

**In scope**

- Template revision diff for Consent to Call option-to-score mapping.
- Postgres scorecard/criterion rows and ClickHouse `score_d` / `scorecard_d` alignment.
- Performance Insights criterion percentage semantics for overridden auto-scored criteria.
- Closed Conversations section aggregate semantics under pinned template revision.

**Non-goals**

- PG/CH reindex or projection repair (ruled out).
- Template rescoring of historical scorecards unless product requests it.

## Source Context

- **Repos:** `go-servers`, `director`, `clickhouse-schema`
- **Worktrees:** main checkouts
- **Branches:** main
- **Labels:** `ScanHealth`, `scorecard-template-revision`
- **Slack:** https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785114593796649

## Entities

| Field | Value |
|---|---|
| Customer / profile | scan-health / us-west-2 |
| Conversation | `019f8ae3-0135-7e87-8248-0e71b0977865` |
| Scorecard | `019f8c65-1276-71f6-bc84-8304492da8d9` |
| Template | `019d4173-43d0-77cd-95f2-e59ab3f53ff7` (Telesales QM) |
| Scorecard revision | `a88e3c94` |
| Latest revision (UI URL) | `eb3e8862` |
| Criterion | `019d4176-27ef-76e1-9836-9d2cabe674b5` (Consent to Call) |
| Section | `019d4174-0ed4-7040-8c43-c6a57bdc439d` (Positive Contact / Communication Skills) |

## Current Understanding

The manual override **did update** stored data. Postgres and ClickHouse are aligned. The reported mismatch is explained by **mixed-revision QA score semantics**:

- Revision `a88e3c94` maps Consent **Yes → 0 points (0%)**, **No → 1 point (100%)**.
- Revision `eb3e8862` corrects this to normal pass/fail mapping (Yes = pass).
- The scorecard is pinned to `a88e3c94`. All six section criteria are Yes (`numeric_value = 0`), so the section aggregate `(5×100% + 1×0%) / 6 = 83.3%` is correct for the pinned revision.
- Consent was auto-scored No (`ai_value = 1`) and manually overridden to Yes (`numeric_value = 0`). PI showing 0% for the criterion is consistent with the pinned revision's inverted mapping.

This is the same pattern family as [CONVI-7254](CONVI-7254.md) and [CONVI-7238](CONVI-7238.md). It is not a `scorecard-data-sync` defect.

## Findings and Decisions

- Reclassified from `scorecard-data-sync` after PG/CH alignment was confirmed. Sync-domain duplicate artifacts were removed; routing note retained in `scorecard-data-sync/log/2026-07-27.md`.
- Linear ticket created and triaged to Done with label `scorecard-template-revision`.

## Blockers and Dependencies

- Product decision on whether analytics should surface revision context or rescoring is needed before a customer-facing fix.
- Optional follow-up: trace whether Closed Conversations UI resolves latest template revision in URL while scoring uses pinned revision.

## Validation and Rollout

- Production Postgres read-only queries on 2026-07-27.
- Production ClickHouse queries on `scan_health_us_west_2` on 2026-07-27.
- No code change shipped; diagnosis complete.

## Next Actions

1. Share investigation deliverable with PM/managers for initiative planning.
2. Confirm with template owner whether `a88e3c94` inversion was a configuration bug fixed in `eb3e8862`.
3. Decide whether to fold into umbrella mixed-revision QA semantics initiative alongside CONVI-7238 and CONVI-7254.

## Timeline

- 2026-07-26 — Jimmy Skelton reports on Slack.
- 2026-07-27 — PG + CH investigation; sync ruled out; pattern named and reclassified to analytics.
- 2026-07-27 — [CONVI-7378](https://linear.app/cresta/issue/CONVI-7378/auto-scored-scorecard-changes-not-updating) created and triaged to Done. Evidence: `sessions/2026-07-27/codex-convi-7378-scan-consent-qa-score.md`, `deliverables/convi-7378-scan-consent-qa-score-investigation.md`.
