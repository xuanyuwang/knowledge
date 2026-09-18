# CONVI-7682: Fix Group Calibration consistency percentage

**Status:** complete
**Primary domain:** `scorecard-workflows`
**Primary subdomain:** `group-calibration`
**Official ticket:** [CONVI-7682](https://linear.app/cresta/issue/CONVI-7682/consistency-score-percent-showing-as-9000percent-instead-of-90percent)
**Last updated:** 2026-09-11

## Objective and Impact

- **Objective:** Display an already percentage-scaled Group Calibration consistency score as `90%`, not `9000%`, on QM Task Home.
- **Customer/system impact:** Woolies sees materially incorrect consistency values in active Group Calibration task rows.
- **Role:** diagnosed and implemented

## Scope

**In scope**

- Correct the consistency-score formatter in the Group Calibration progress table.
- Add focused behavior coverage for percentage formatting.

**Non-goals**

- Change backend score calculations or API contracts.
- Address the separately tracked calibration due-time issue.

## Source Context

- **Repos:** `director`
- **Worktrees:** `/Users/xuanyu.wang/repos/director-convi-7682`
- **Branches:** `xw/convi-7682-consistency-score-percentage`
- **PRs/commits:** [director PR #22763](https://github.com/cresta/director/pull/22763); commit `4b301086af`

## Current Understanding

The `consistencyScore` supplied to the QM Task Home Group Calibration progress table is a dynamically computed API field on a 0-100 percentage scale. Woolworths production does not persist this field: `director.scorecards.calibration_consistency_score` is null for every row. `ListScorecards` computes it as `100 * matching criteria / scorable criteria`; backend tests assert outputs such as `75`, `66.67`, and `100`. An earlier localization change replaced `formatPercentage` with `fmtPercentage`, whose contract expects a 0-1 fraction, without normalizing the input; it therefore scaled `90` to `9000%`. The renderer now divides defined API values by 100 before locale-aware percentage formatting.

## Findings and Decisions

- Replace the fraction-scale formatter only at the incorrect task-table rendering boundary.
- Preserve backend values, sorting, score visibility rules, and other calibration surfaces.
- Production storage is not the source of the malformed display; consistency is computed dynamically in the API response on a 0-100 scale.
- Confirmed regression: Director PR #14756 (`d3d29a030ba`, merged 2025-11-07) changed this cell from `formatPercentage(consistencyScore)` to `fmtPercentage(consistencyScore)` during an i18n migration. The new formatter expects 0-1 input, but the PR preserved the 0-100 API value and added no test for the displayed score.

## Blockers and Dependencies

- None.

## Validation and Rollout

- `yarn workspace @cresta/director-app lint:precommit` — passed.
- `yarn tsc` — passed.
- `yarn vitest packages/director-utils/src/utils.test.ts --run` — 18 tests passed.
- `git diff --check` — passed.
- Draft PR #22763 created from the repository template; functional smoke proof and preview environment remain pending.

## Next Actions

1. Capture before/after full-screen proof in a preview environment with Group Calibration data.
2. Move PR #22763 out of draft after proof/review readiness, then deploy through the normal Director release path.

## Timeline

- 2026-09-11 — Retrieved the ticket through Linear MCP, traced the faulty renderer, updated from current `origin/main`, and created the dedicated Director worktree. Evidence: `sessions/2026-09-11/codex-convi-7682-consistency-percentage.md`.
- 2026-09-11 — Normalized the 0-100 score before locale-aware formatting and completed lint, type, formatter-suite, and diff validation.
- 2026-09-11 — Verified Woolworths production storage through a read-only app DB query and traced `ListScorecards` plus `CalculateConsistencyScore`; the DB field is universally null and the API dynamically returns 0-100 values.
- 2026-09-11 — Committed and pushed the fix, then opened draft Director PR #22763 using the repository PR template with functional smoke steps and pending proof called out explicitly.
- 2026-09-11 — Confirmed the defect is a regression introduced by Director PR #14756 on 2025-11-07: an i18n-only formatter substitution changed the numeric scale without normalizing the existing 0-100 input.
- 2026-09-11 — Patched PR #22763's live description with regression provenance after re-reading the remote body. Preserved the remotely added smoke-UI selection, before/after videos, staging environment link, and `50%` verification example.
