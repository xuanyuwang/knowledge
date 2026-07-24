# CONVI-7254: HCD monthly QA criterion aggregation

**Status:** active
**Primary domain:** `analytics`
**Primary subdomain:** `qa-score`
**Official ticket:** [CONVI-7254](https://linear.app/cresta/issue/CONVI-7254/home-care-delivered-performance-insights-monthly-view-shows-0percent)
**Last updated:** 2026-07-24

## Objective and Impact

- **Objective:** Determine why monthly `RetrieveQAScoreStats` aggregation reports effectively 0% for HCD's "Asking for phone number" criterion while most daily values are non-zero.
- **Customer/system impact:** HCD cannot trust monthly Performance Insights criterion scores.
- **Role:** diagnosed

## Scope

**In scope**

- `RetrieveQAScoreStats`, its ClickHouse aggregation, and source rows for criterion `019e7502-0e12-756b-a7ca-c81276df1781`.

**Non-goals**

- Unrelated Performance Insights APIs or frontend presentation changes unless runtime evidence points outside `RetrieveQAScoreStats`.

## Source Context

- **Repos:** `go-servers`
- **Worktrees:** `/Users/xuanyu.wang/repos/go-servers-convi-7254`
- **Branches:** `convi-7254-monthly-qa-score`
- **PRs/commits:** prior zero-weight fix `15822775f3` / PR #27623
- **Report:** [`deliverables/hcd-mixed-revision-qa-score-investigation.md`](../deliverables/hcd-mixed-revision-qa-score-investigation.md)

## Current Understanding

Production ClickHouse rows for May contain 4,747 scores from template revision `2ab0f092` with semantic weight zero, physically stored as `float_weight=1e-13` to keep aggregation arithmetic defined, plus three zero-valued scores from revision `33e46102` with weight 1. Production PostgreSQL revision history shows that `33e46102` introduced the criterion at weight 1 on May 29 at 14:32:09 Toronto time, and `2ab0f092` changed it to weight 0 only 3 minutes 44 seconds later. The weight remained zero through June and was restored to 1 by revision `d0aa3ef7` on July 1 at 07:37:43. The exact SQL shape captured by the unit-test golden queries reproduces the API's monthly result, confirming that the three rows created during the brief May weight-1 interval dominate the criterion aggregate. A fix has not yet been selected.

## Findings and Decisions

- The supplied API response is consistent with `SUM(percentage_value * float_weight) / SUM(float_weight)`: target weight sum is approximately `3`, and the weighted numerator comes almost entirely from the `1e-13` rows.
- Direct production query shows 1,303 passing and 3,444 failing semantic weight-zero rows stored as `1e-13`, plus three failing unit-weight rows on May 29.
- Daily aggregation is normal on most dates but is also effectively zero on May 29, where the three unit-weight rows occur.
- Reconstructing the exact `RetrieveQAScoreStats` SQL from the unit-test golden files produced `score=4.343333332646083e-11`, `weight_sum=3.0000000004746927`, and 4,750 scorecards, matching the API response.
- The exact query's unweighted criterion average is `0.27431578947368424`; on May 29 the weighted score is approximately zero while the unweighted score is `0.2146118721461187`.
- Root cause is cross-template-revision weighting within criterion-grouped aggregation, not monthly time truncation, voicemail exclusion, response conversion, or cache behavior.
- Template history is `1 → 0 → 1`, not only `0 → 1`: criterion introduced at weight 1 in `33e46102` (2026-05-29 14:32:09 Toronto), changed to 0 in `2ab0f092` 3m44s later, remained 0 across four June revisions, and returned to 1 in `d0aa3ef7` (2026-07-01 07:37:43 Toronto).
- Restoring the latest template to weight 1 does not rewrite historical score rows; each row retains values derived from its referenced revision.

## Blockers and Dependencies

- Need agreement on the intended criterion-grouped weighting semantics before implementing a code fix.

## Validation and Rollout

- Production ClickHouse source distribution and daily aggregates queried on 2026-07-24.
- Exact monthly and daily generated-query shapes executed against production and matched the reported API output.
- Temporary runtime instrumentation was removed after the exact SQL reproduction; the source worktree is clean.

## Next Actions

1. Confirm whether criterion-grouped results should average applicable criterion scores independently of template weight.
2. Implement and test the agreed aggregation semantics.
3. Re-run the exact monthly and daily production queries for validation.

## Timeline

- 2026-07-24 — Reproduced the near-zero monthly math from production source rows and the exact generated SQL shape, isolated cross-revision criterion weights as the cause, and recovered the production weight-revision timeline. Evidence: `sessions/2026-07-24/codex-convi-7254-monthly-qa-score.md`.
