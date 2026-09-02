# CONVI-7378: Auto-scored scorecard changes not updating

**Status:** follow-up diagnosed; customer response drafted
**Primary domain:** analytics
**Primary subdomain:** qa-score
**Official ticket:** [CONVI-7378](https://linear.app/cresta/issue/CONVI-7378/auto-scored-scorecard-changes-not-updating)
**Pattern:** [mixed-revision QA score semantics](../deliverables/mixed-revision-qa-score-semantics.md) — pinned-revision option-score inversion
**Report:** [SCAN Consent to Call investigation](../deliverables/convi-7378-scan-consent-qa-score-investigation.md)
**Last updated:** 2026-08-19

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

### Follow-up example

| Field | Value |
|---|---|
| Conversation | `019ffc63-5239-7c1b-9f9d-ebaef898ec5d` |
| Scorecard | `019ffc6d-4515-7c21-a9df-c383019e2ee2` |
| Created | 2026-08-13 18:40:47 UTC |
| Scorecard revision | `7290d017` (published 2026-08-04) |
| Score | 95.8 overall; 83.3 Positive Contact section |

## Current Understanding

The manual override **did update** stored data. Postgres and ClickHouse are aligned. The reported mismatch is explained by **mixed-revision QA score semantics**:

- Revision `a88e3c94` maps Consent **Yes → 0 points (0%)**, **No → 1 point (100%)**.
- Revision `eb3e8862` corrects this to normal pass/fail mapping (Yes = pass).
- The scorecard is pinned to `a88e3c94`. All six section criteria are Yes (`numeric_value = 0`), so the section aggregate `(5×100% + 1×0%) / 6 = 83.3%` is correct for the pinned revision.
- Consent was auto-scored No (`ai_value = 1`) and manually overridden to Yes (`numeric_value = 0`). PI showing 0% for the criterion is consistent with the pinned revision's inverted mapping.

This is the same pattern family as [CONVI-7254](CONVI-7254.md) and [CONVI-7238](CONVI-7238.md). It is not a `scorecard-data-sync` defect.

The 2026-08-17 follow-up exposed a revision regression that the original explanation did not cover:

- The newer scorecard is pinned to `7290d017`, not the July `a88e3c94` revision.
- `7290d017` reintroduced the inverted mapping: Consent Yes/value 0 -> 0 points and No/value 1 -> 1 point.
- The latest observed revision, `b3facf54` from 2026-08-17, still has the inverted mapping.
- The override persisted (`numeric_value=0`, `ai_value=1`, `manually_scored=true`) and PostgreSQL/ClickHouse agree.
- Of 24 leaf criteria, Consent is the only one with `percentage_value=0`; `23 / 24 = 95.8%`. Its six-criterion section is `5 / 6 = 83.3%`.

Therefore the follow-up conversation was created after the July correction, but on a later revision that regressed the scoring configuration. The issue remains template semantics rather than data propagation.

## Findings and Decisions

- Reclassified from `scorecard-data-sync` after PG/CH alignment was confirmed. Sync-domain duplicate artifacts were removed; routing note retained in `scorecard-data-sync/log/2026-07-27.md`.
- Linear ticket created and triaged to Done with label `scorecard-template-revision`.
- Follow-up investigation corrected the earlier customer explanation: later revisions reintroduced the inverted Consent mapping.

## Blockers and Dependencies

- Product decision on whether analytics should surface revision context or rescoring is needed before a customer-facing fix.
- Optional follow-up: trace whether Closed Conversations UI resolves latest template revision in URL while scoring uses pinned revision.

## Validation and Rollout

- Production Postgres read-only queries on 2026-07-27.
- Production ClickHouse queries on `scan_health_us_west_2` on 2026-07-27.
- No code change shipped; diagnosis complete.

## Next Actions

1. Send the drafted follow-up response after review.
2. Ask the SCAN template owner to correct the active Consent mapping to Yes = 1 and No = 0.
3. Decide how to remediate manually edited scorecards pinned to affected revisions; the existing AutoQM-only backfill does not cover them.
4. Keep this case in the umbrella mixed-revision QA semantics initiative alongside CONVI-7238 and CONVI-7254.

## Timeline

- 2026-07-26 — Jimmy Skelton reports on Slack.
- 2026-07-27 — PG + CH investigation; sync ruled out; pattern named and reclassified to analytics.
- 2026-07-27 — [CONVI-7378](https://linear.app/cresta/issue/CONVI-7378/auto-scored-scorecard-changes-not-updating) created and triaged to Done. Evidence: `sessions/2026-07-27/codex-convi-7378-scan-consent-qa-score.md`, `deliverables/convi-7378-scan-consent-qa-score-investigation.md`.
- 2026-08-17 — Jimmy supplied newer conversation `019ffc63-5239-7c1b-9f9d-ebaef898ec5d`, challenging the old-revision-only explanation.
- 2026-08-19 — Follow-up production validation found revision `7290d017` had reintroduced the inverted mapping; draft response recorded in `sessions/2026-08-19/codex-convi-7378-follow-up.md`.
