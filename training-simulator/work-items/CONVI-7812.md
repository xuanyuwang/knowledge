# CONVI-7812: Update reporting pass rates for changing rubrics

**Status:** Original change merged; quiz question percentage follow-up open in draft director#24097
**Primary domain:** `training-simulator`
**Primary subdomain:** `reporting`
**Official ticket:** [CONVI-7812](https://linear.app/cresta/issue/CONVI-7812/update-reporting-pass-rates-for-changing-rubrics)
**Last updated:** 2026-10-22

## Objective and Impact

- **Objective:** Show lesson and module pass rates, including criterion rows, as percentages without displayed passed/applicable counts when a module's rubric changes.
- **Customer/system impact:** Historical criteria remain visible, and each criterion percentage uses only its applicable evaluated results. The module pass rate continues to use all assigned participants.
- **Role:** implemented

## Scope

**In scope**

- Conversation-module criterion rows in the lesson and module reporting drawers.
- A mixed-rubric view-model test covering shared, removed, and added criteria.

**Non-goals**

- Recomputing backend aggregates or changing the reporting API.
- Original PR did not change quiz question rows; the October 22 follow-up now changes them to percentages.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/director`; backend behavior reviewed in `/Users/xuanyu.wang/repos/go-servers`.
- **Worktree:** `/Users/xuanyu.wang/repos/director-convi-7812`.
- **Branch:** Follow-up `xw/convi-7812-quiz-question-pass-rates` from `origin/main` at `6c25ce0789`; original branch `xw/convi-7812-reporting-pass-rates` is merged.
- **PRs/commits:** [director#23579](https://github.com/cresta/director/pull/23579), commits `a08f82fafe` and `9e39225baa`.

## Current Understanding

The module-stats backend already seeds current criteria and appends historical result-only criteria by ID. Each criterion exposes `passed_result_count` and `applicable_result_count`; N/A results are excluded from the latter. The Director drawer previously showed a fraction with its overall module percentage and only fractions on criterion rows. The final UI shows percentages without counts for pass rates, while retaining criterion-specific denominators in the calculation.

## Findings and Decisions

- The Slack example changes C1+C2 to C1+C3 and asks to retain all three criteria. The backend aggregation supports this for scored latest attempts of assigned agents.
- The overall denominator remains `total_assignment_count`; a criterion denominator is its `applicable_result_count`, which may differ due to rubric changes or N/A results. Neither count is displayed next to a pass rate.
- Zero applicable results continue to display the existing placeholder rather than a misleading 0%.

## Validation and Rollout

- 2026-10-13 design follow-up: module stats now use three cards for pass rate, average score with threshold, and active lessons; criterion values have a "Pass rate" heading. Figma frame `13112:26624` confirms a 24px panel gap and `Background/Elevation` (`#F8F9FA`), already mapped to the drawer token.
- Focused module view-model tests: 13 passed. Director app TypeScript, changed-file ESLint, Biome, i18n full extraction, and commit hooks passed. Local Director at `localhost:3101` connected to Walter dev rendered all three cards and a 50% criterion row without wrapping at a wide viewport.
- Commit `116c5c6bf9` was pushed to [director#23579](https://github.com/cresta/director/pull/23579). The PR description was updated without changing its two existing attachment links. CI on this new head was pending at handoff. The user will replace the After video; the existing attachment shows the earlier layout.
- Focused Director view-model tests: 24 passed after the percentage-only follow-up, using Node's temporary local-storage file option. The initial run failed during setup before tests due to Node v26 local-storage configuration.
- Biome check: passed for both changed files.
- Director app TypeScript check: passed.
- Director pre-commit i18n extraction, lint, i18next lint, and format checks: passed.
- [Director PR #23579](https://github.com/cresta/director/pull/23579) is ready for review. The PR body includes inline full-screen Before and After videos of the Walter dev module criterion row: `(1/2)` before and `50%` in the local build connected to Walter dev.
- The repository's `smoke-UI` QA request is selected, `enableTrainingSimulatorReporting` and the Walter dev environment are documented, and all required PR checks passed after the percentage-only commit.
- Walter dev source data and the local build were used for UI verification; the branch was also built to the staging/production CDN by CI, but no production rollout was performed.

## Next Actions

- Review draft [director#24097](https://github.com/cresta/director/pull/24097) for the quiz question display gap.
- Browser verification and before/after videos remain pending; CI has not been confirmed.

## Timeline

- 2026-10-07 — Read CONVI-7812 and its Slack thread; traced backend criterion aggregation and Director presentation; created a dedicated Director worktree and implemented percentage-plus-fraction criterion rows. [Session](../sessions/2026-10-07/codex-convi-7812-reporting-pass-rates.md).
- 2026-10-07 — Committed `a08f82fafe`, pushed `xw/convi-7812-reporting-pass-rates`, and opened draft [director#23579](https://github.com/cresta/director/pull/23579). All local checks passed; CI was queued at creation.
- 2026-10-07 — Recorded and frame-checked full-screen Walter dev and local-build videos, added them to the PR's Before and After fields, selected `smoke-UI`, documented the feature flag and environment, and marked the PR ready for review. Replaced the initial clips after frame inspection showed overlays and loading states.
- 2026-10-07 — Interpreted the ticket as percentage-only pass-rate display, committed/pushed `9e39225baa`, and refreshed the After video and PR copy. The Before video remains unchanged; GitHub renders both bare attachment URLs inline. Focused tests (24), Biome, TypeScript, and `qa-metadata-check` passed.
- 2026-10-13 — Matched the updated Figma side panel in the module stats drawer, verified the local view against Walter dev data, committed/pushed `116c5c6bf9`, and updated the PR summary. The user took ownership of the replacement video.

- 2026-10-22 — Verified page coverage and live merged state (merged 2026-10-14T01:59:45Z). [Session](../sessions/2026-10-22/codex-convi-7812-page-coverage.md).

- 2026-10-22 — Implemented quiz question percentages and shared Pass rate heading in draft [director#24097](https://github.com/cresta/director/pull/24097), commit f740a2386f. 27 focused tests, TypeScript, ESLint, Biome and hooks passed. [Session](../sessions/2026-10-22/codex-convi-7812-quiz-question-percentages.md).
