# CONVI-7254: HCD monthly QA criterion aggregation

**Status:** active
**Primary domain:** `analytics`
**Primary subdomain:** `qa-score`
**Official ticket:** [CONVI-7254](https://linear.app/cresta/issue/CONVI-7254/home-care-delivered-performance-insights-monthly-view-shows-0percent)
**Last updated:** 2026-07-27

## Objective and Impact

- **Objective:** Determine why monthly `RetrieveQAScoreStats` aggregation reports effectively 0% for HCD's "Asking for phone number" criterion and 100% for "How did you hear about HCD" despite contradictory drill-down data.
- **Customer/system impact:** HCD cannot trust monthly Performance Insights criterion scores.
- **Role:** diagnosed

## Scope

**In scope**

- `RetrieveQAScoreStats`, its ClickHouse aggregation, frontend request/rendering paths, and source rows for criteria `019e7502-0e12-756b-a7ca-c81276df1781` and `0199c056-ce00-751a-ab4b-ecc11e4d7412`.

**Non-goals**

- Unrelated Performance Insights APIs or frontend presentation changes unless runtime evidence points outside `RetrieveQAScoreStats`.

## Source Context

- **Repos:** `go-servers`
- **Worktrees:** `/Users/xuanyu.wang/repos/go-servers-convi-7254`
- **Branches:** `convi-7254-monthly-qa-score`
- **PRs/commits:** prior zero-weight fix `15822775f3` / PR #27623
- **Report:** [`deliverables/hcd-mixed-revision-qa-score-investigation.md`](../deliverables/hcd-mixed-revision-qa-score-investigation.md)

## Current Understanding

Both reported symptoms are caused by cross-template-revision criterion weighting. In May, three failing weight-1 rows dominate 4,747 semantic-weight-zero rows and force "Asking for phone number" toward 0%. In February, one passing weight-1 row dominates 427 semantic-weight-zero rows and forces "How did you hear about HCD" toward 100%, even though only 94 of 428 scorecards passed. Exact generated-query reproductions match both API results. Frontend requests and rendering pass through the backend results without recomputing the headline from agent percentages. A fix has not yet been selected.

## Findings and Decisions

- The supplied API response is consistent with `SUM(percentage_value * float_weight) / SUM(float_weight)`: target weight sum is approximately `3`, and the weighted numerator comes almost entirely from the `1e-13` rows.
- Direct production query shows 1,303 passing and 3,444 failing semantic weight-zero rows stored as `1e-13`, plus three failing unit-weight rows on May 29.
- Daily aggregation is normal on most dates but is also effectively zero on May 29, where the three unit-weight rows occur.
- Reconstructing the exact `RetrieveQAScoreStats` SQL from the unit-test golden files produced `score=4.343333332646083e-11`, `weight_sum=3.0000000004746927`, and 4,750 scorecards, matching the API response.
- The exact query's unweighted criterion average is `0.27431578947368424`; on May 29 the weighted score is approximately zero while the unweighted score is `0.2146118721461187`.
- Root cause is cross-template-revision weighting within criterion-grouped aggregation, not monthly time truncation, voicemail exclusion, response conversion, or cache behavior.
- Template history is `1 → 0 → 1`, not only `0 → 1`: criterion introduced at weight 1 in `33e46102` (2026-05-29 14:32:09 Toronto), changed to 0 in `2ab0f092` 3m44s later, remained 0 across four June revisions, and returned to 1 in `d0aa3ef7` (2026-07-01 07:37:43 Toronto).
- Restoring the latest template to weight 1 does not rewrite historical score rows; each row retains values derived from its referenced revision.
- For "How did you hear about HCD" in February, source rows are one passing weight-1 row, 93 passing `1e-13` rows, and 334 failing `1e-13` rows. The weighted result is `0.9999999999666005` (displayed as 100%); the equal-observation result is `94 / 428 = 21.96%`.
- Latrese Proctor owns the unit-weight pass. Her weighted result is approximately 100% across eight scorecards, while an equal-observation result is `1 / 8 = 12.5%`. Stacy Brown's three equal-weight rows produce the visible 33.33%.
- `averageQaScore` is reconstructed from summed ClickHouse weighted numerators and denominators, not averaged from the displayed per-agent percentages. The frontend displays this field in the popover headline and displays each returned agent score without further aggregation.
- The second symptom confirms the same defect in the opposite direction: a unit-weight failure collapses a bucket toward 0%, while a unit-weight pass inflates it toward 100%.

## Blockers and Dependencies

- Need agreement on the intended criterion-grouped weighting semantics before implementing a code fix.

## Validation and Rollout

- Production ClickHouse source distribution and daily aggregates queried on 2026-07-24.
- Exact monthly and daily generated-query shapes executed against production and matched the reported API output.
- Exact February per-agent query executed against production and matched the supplied 428-scorecard response and `averageQaScore=1`.
- Temporary runtime instrumentation was removed after the exact SQL reproduction; the source worktree is clean.

## Next Actions

1. Confirm whether criterion-grouped results should average applicable criterion scores independently of template weight.
2. Implement and test the agreed aggregation semantics.
3. Re-run the exact monthly and daily production queries for validation.

## Timeline

- 2026-07-24 — Reproduced the near-zero monthly math from production source rows and the exact generated SQL shape, isolated cross-revision criterion weights as the cause, and recovered the production weight-revision timeline. Evidence: `sessions/2026-07-24/codex-convi-7254-monthly-qa-score.md`.
- 2026-07-27 — Traced the frontend cell/popover request and rendering paths and reproduced the February 100% result from production. Confirmed that one passing unit-weight row dominates 427 semantic-weight-zero rows. Evidence: `sessions/2026-07-27/codex-convi-7254-february-qa-score.md`.
