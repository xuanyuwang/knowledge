# Heartland: Behavior Hints adherence mismatch

**Status:** in review
**Primary domain:** `analytics`
**Primary subdomain:** `assistance-insights`
**Official issue:** [CONVI-7387](https://linear.app/cresta/issue/CONVI-7387/discrepancy-in-user-adherence-data-between-insights-tools); [Slack thread](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785201046765479)
**Last updated:** 2026-08-04

## Objective and Impact

- **Objective:** Explain why Jamie Rimbach's adherence for “Offering Scheduling Flexibility and Availability” is 79% in Performance Insights but roughly doubled in Assistance Insights, and why aggregate Behavior Hints can exceed 100%.
- **Customer/system impact:** Heartland users see inconsistent and mathematically invalid adherence percentages across analytics surfaces.
- **Role:** implementation in review

## Scope

**In scope**

- Director request construction and display transformations for Performance Insights and Assistance Insights.
- API and ClickHouse query semantics required to reproduce the displayed data.
- Identification of the discrepancy's root cause and likely fix location.

**Non-goals**

- Production data mutation.
- Implementing a fix before the diagnosis is validated.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/go-servers`; related Director and ClickHouse schema repositories.
- **Worktrees:** `/Users/xuanyu.wang/repos/go-servers-convi-7387`.
- **Branches:** `convi-7387-discrepancy-in-user-adherence-data-between-insights-tools`.
- **PRs/commits:** [go-servers #30782](https://github.com/cresta/go-servers/pull/30782); `190ef15783`.

## Current Understanding

The source data is correct, but Assistance Insights' deployed `RetrieveHintStats` aggregates incompatible units for behavioral hints: sent hints are distinct action annotation IDs while followed hints are distinct positive moment annotation IDs. A single sent hint can have several valid positive moments. Director then directly divides followed moments by sent actions, producing 160.4% for the reported week. Performance Insights separately uses `RetrieveQAScoreStats` and displays a QA criterion score, so the pages are not measuring identical populations even though an action-deduplicated hint rate is 79.1% and numerically matches the report. PR #30782 changes the behavioral followed count to distinct action annotation IDs.

## Findings and Decisions

- Performance Insights reports 79%; Assistance Insights reports roughly twice that value.
- Performance Insights uses `RetrieveQAScoreStats`; Assistance Behavioral Hints uses `RetrieveHintStats`. QA criterion score and post-hint follow rate are distinct metrics and are not inherently expected to match.
- Director calls `RetrieveHintStats` and directly computes `totalHintFollowedCount / totalHintSentCount` on the Behavioral Hints card, breakdown table, and agent/policy leaderboard.
- The ClickHouse API query counts sent as `COUNT(DISTINCT adherence_action_annotation_id)` but followed as `COUNT(DISTINCT moment_annotation_id)`.
- For Jamie and the reported policy over `[2026-07-20, 2026-07-27)`, the production data contains 134 sent actions, 215 positive moments, and 106 distinct followed actions:
  - Current Assistance result: `215 / 134 = 160.4%`.
  - Action-deduplicated hint result: `106 / 134 = 79.1%`, numerically matching the Performance report but not proving metric equivalence.
- Multiple positive moments per action are valid source rows, not storage duplicates. The API turns them into duplicate followed-hint counts.
- This is primarily a backend query/API semantic defect. The frontend exposes the invalid response and lacks a defensive check, but clamping to 100% would conceal the issue and remain incorrect.
- Action and moment annotations are independent records. Adherence outcome moments carry an application-level `adherence_action_annotation_id` reference to the same-conversation hint action; ClickHouse does not enforce it with a foreign key.
- The default moment-based sent path makes DDX followed IDs a subset of DDX/DNX sent IDs. The alternate action-annotation sent path additionally assumes linked action rows exist and their denormalized dimensions agree.
- A separate pre-existing risk exists for mixed-category requests: sent branches are combined with `MAX`, while Behavioral and KB/GW followed branches are combined with `SUM`. The explicit Behavioral Hint filter makes the non-Behavioral sent branch empty for CONVI-7387, so this does not explain the Heartland result.

## Blockers and Dependencies

- PR #30782 requires review, merge, and deployment before production verification.

## Validation and Rollout

- Read-only Heartland production query reproduced both 160.4% and the action-deduplicated 79.1%.
- Verified at least eight additional recent Heartland user/policy rows where the current API ratio exceeded 100% while action-level ratios remained valid.
- `bazel test //insights-server/internal/analyticsimpl:retrieve_hint_stats_test` passes with regression coverage for both behavioral sent-query implementations.
- `git diff --check` passes.

## Next Actions

1. Review and merge [go-servers #30782](https://github.com/cresta/go-servers/pull/30782).
2. After deployment, replay the Heartland query and verify Assistance Insights remains at or below 100%.
3. Add a data-level regression fixture and consider integrity monitoring for orphaned or dimension-mismatched action links.
4. Investigate the separate `MAX`-sent versus `SUM`-followed behavior for unfiltered mixed-category requests.
5. Optionally add frontend invariant telemetry or a non-misleading fallback; do not treat clamping as the fix.

## Timeline

- 2026-07-28 — Investigation opened from the linked Slack report.
- 2026-07-28 — Reproduced 160.4% vs 79.1% from Heartland ClickHouse and isolated the mixed-unit `RetrieveHintStats` aggregation. Evidence: `sessions/2026-07-28/codex-heartland-behavior-hints-mismatch.md`.
- 2026-08-04 — Created CONVI-7387 fix in [go-servers #30782](https://github.com/cresta/go-servers/pull/30782), aligning behavioral followed and sent counts at the action-annotation grain.
- 2026-08-05 — Documented the annotation data model, physical schema, query stages, and referential-integrity caveat. Evidence: `sessions/2026-08-05/codex-hint-annotation-data-model.md`.
