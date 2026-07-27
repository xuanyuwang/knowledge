# CONVI-7378: SCAN Consent to Call QA score investigation

**Ticket:** [CONVI-7378](https://linear.app/cresta/issue/CONVI-7378/auto-scored-scorecard-changes-not-updating)
**Customer:** SCAN Health (`scan-health` / `us-west-2`)
**Surfaces:** Closed Conversations scorecard, Performance Insights
**Template:** Telesales QM (`019d4173-43d0-77cd-95f2-e59ab3f53ff7`)
**Criterion:** Consent to Call (`019d4176-27ef-76e1-9836-9d2cabe674b5`)
**Pattern family:** [mixed-revision QA score semantics](mixed-revision-qa-score-semantics.md)
**Validated:** 2026-07-27

## Executive Summary

SCAN reported that manually changing an auto-scored criterion from **No** to **Yes** did not update QA scores. Engineering investigation found that **the override did persist correctly** in both PostgreSQL and ClickHouse. The confusion comes from a **template revision scoring bug** that was later fixed: under the scorecard's pinned revision, selecting **Yes** on Consent to Call contributes **0%**, not 100%.

From the evaluator's perspective, six Yes answers should produce a 100% section score and a passing Consent criterion in Performance Insights. Under the stored revision semantics, the math is internally consistent but contradicts intuitive pass/fail meaning.

This is an **analytics semantics and template-revision trust** problem, not a data-sync failure. It belongs to the same pattern family as [CONVI-7254](https://linear.app/cresta/issue/CONVI-7254) and [CONVI-7238](https://linear.app/cresta/issue/CONVI-7238).

## Customer-Visible Symptoms

1. **Closed Conversations:** All six criteria in Positive Contact / Communication Skills show **Yes**, but the section score remains **83.3%**.
2. **Performance Insights:** Agent shows **0%** on Consent to Call after the evaluator manually changed the auto-scored answer from No to Yes.
3. **User expectation:** Manual overrides should improve scores and PI should reflect the corrected evaluation.

## Non-Engineering Explanation

Think of each scorecard as carrying a snapshot of the scoring rules from when it was created. SCAN briefly had a template version where "Consent to Call — Yes" was configured to count as **zero points**. The scorecard still uses that older rulebook even though the template has since been corrected.

When the evaluator changes the answer to Yes, the system records the change faithfully. But under the old rulebook, Yes is still a failing score for that criterion. That is why Performance Insights shows 0% and why the section cannot reach 100% even when every question is marked Yes.

The system is not "failing to update." It is applying outdated scoring meaning that conflicts with how evaluators read Yes/No labels today.

## What We Verified

| Check | Result |
|---|---|
| Postgres scorecard row updated | Yes — `updated_at` 2026-07-27 15:30:06 UTC |
| Consent criterion override stored | Yes — `numeric_value=0` (Yes), `ai_value=1` (original auto No) |
| ClickHouse `score_d` matches Postgres | Yes |
| ClickHouse `scorecard_d` matches Postgres | Yes — score 95.8, revision `a88e3c94` |
| Section aggregate recomputed | Yes — 83.3% is correct for pinned revision |
| Data sync defect | **Ruled out** |

## Root Cause

### Template revision scoring inversion

| Revision | When used | Yes (value 0) | No (value 1) | autoFail on |
|---|---|---|---|---|
| `a88e3c94` | Scorecard pinned revision | **0 points (0%)** | **1 point (100%)** | No |
| `eb3e8862` | Latest template (UI URL) | **1 point (100%)** | **0 points (0%)** | Yes |

The scorecard references `a88e3c94`. The UI URL references the latest revision `eb3e8862`, which has corrected semantics.

### Why 83.3% is mathematically correct (but misleading)

Under revision `a88e3c94`, all six criteria set to Yes produce:

| Criterion | Yes percentage |
|---|---|
| Opening Greeting | 100% |
| Transitional / Acknowledgement | 100% |
| Properly Identified Caller Type | 100% |
| **Consent to Call** | **0%** |
| Demonstrated Call Control | 100% |
| Active Listening | 100% |

Section average: `(100 + 100 + 100 + 0 + 100 + 100) / 6 = 83.3%`

### Why Performance Insights shows 0% for Consent

PI reads the stored criterion percentage derived from the pinned revision's option-to-score mapping. Yes → 0% under `a88e3c94`. The override changed the stored answer; it did not change the revision's scoring rules.

## Relationship to Related Tickets

| Ticket | Customer | Shared pattern | Difference |
|---|---|---|---|
| [CONVI-7378](https://linear.app/cresta/issue/CONVI-7378) | SCAN | Pinned-revision option-score inversion | Single scorecard; manual override vs intuitive pass/fail |
| [CONVI-7254](https://linear.app/cresta/issue/CONVI-7254) | HCD | Cross-revision weight domination in monthly QA aggregation | Population-level; brief weight-1 revisions dominate |
| [CONVI-7238](https://linear.app/cresta/issue/CONVI-7238) | United | Revision-derived N/A exclusion in leaderboard counts | Inclusion filter drops N/A-submitted scorecards |

All three share one structural issue: **analytics and UI surfaces do not make template-revision semantics legible to users.**

## Initiative Options for Product and Engineering Leadership

### Option A — Umbrella: revision-aware QA analytics (recommended strategic frame)

Treat CONVI-7378, CONVI-7254, and CONVI-7238 as one product-quality initiative: **make QA analytics trustworthy when template revisions change scoring meaning**.

Potential scope:

- Define a cross-surface contract for how criterion percentages, weights, and N/A behave across revisions.
- Fix `RetrieveQAScoreStats` cross-revision aggregation (CONVI-7254).
- Revisit leaderboard inclusion semantics for N/A-submitted scorecards (CONVI-7238).
- Add UI/API signals when displayed scores reflect superseded revision semantics (CONVI-7378).

**Pros:** Addresses systemic trust risk, not just one customer.
**Cons:** Requires PM agreement on intended semantics before engineering work.

### Option B — Point fix: SCAN template rescoring

Rescore affected SCAN scorecards after confirming `a88e3c94` was a configuration error.

**Pros:** Fastest customer relief for SCAN.
**Cons:** Does not prevent recurrence; does not fix analytics aggregation semantics for other customers.

### Option C — Transparency: revision context in UI

Show evaluators and managers when a scorecard uses an older template revision whose scoring rules differ from the current template.

**Pros:** Low engineering risk; improves interpretability immediately.
**Cons:** Does not change computed percentages; may increase confusion before rescoring.

### Option D — Closed Conversations template resolution audit

Investigate whether the scorecard page mixes latest-revision labels (URL `eb3e8862`) with pinned-revision scoring (`a88e3c94`).

**Pros:** May reduce label/score mismatch on the evaluation surface itself.
**Cons:** Does not fix Performance Insights aggregation semantics.

## Open Product Decisions

1. Should a manual override on a criterion always be interpreted using **current template pass/fail meaning**, or always using the **scorecard's pinned revision**?
2. When a template scoring bug is corrected, should historical scorecards be **rescored automatically**, **on demand**, or **left unchanged**?
3. Should Performance Insights and Leaderboard **disclose revision context** when a criterion's stored percentage may disagree with current template intent?
4. Should CONVI-7378 be tracked as a standalone customer issue or folded into the umbrella mixed-revision QA semantics initiative?

## Recommended Customer Communication

> The manual score change was saved correctly. The displayed percentages reflect the scoring rules from the template version active when the scorecard was created. In that version, "Consent to Call — Yes" was configured to count as zero points. A later template correction changed this behavior for new evaluations, but existing scorecards retain the earlier rules unless rescored.

## Evidence Links

- Canonical work item: [`../work-items/CONVI-7378.md`](../work-items/CONVI-7378.md)
- Pattern index: [`mixed-revision-qa-score-semantics.md`](mixed-revision-qa-score-semantics.md)
- Session evidence: [`../sessions/2026-07-27/codex-convi-7378-scan-consent-qa-score.md`](../sessions/2026-07-27/codex-convi-7378-scan-consent-qa-score.md)
- Example conversation: https://scan-health.cresta.com/director/conversations/closed/customers%2Fscan-health%2Fprofiles%2Fus-west-2%2Fconversations%2F019f8ae3-0135-7e87-8248-0e71b0977865
