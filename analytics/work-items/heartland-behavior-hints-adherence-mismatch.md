# Heartland: Behavior Hints adherence mismatch

**Status:** diagnosed
**Primary domain:** `analytics`
**Primary subdomain:** `performance-insights`
**Official issue:** [Slack thread](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785201046765479)
**Last updated:** 2026-07-28

## Objective and Impact

- **Objective:** Explain why Jamie Rimbach's adherence for “Offering Scheduling Flexibility and Availability” is 79% in Performance Insights but roughly doubled in Assistance Insights, and why aggregate Behavior Hints can exceed 100%.
- **Customer/system impact:** Heartland users see inconsistent and mathematically invalid adherence percentages across analytics surfaces.
- **Role:** diagnosed

## Scope

**In scope**

- Director request construction and display transformations for Performance Insights and Assistance Insights.
- API and ClickHouse query semantics required to reproduce the displayed data.
- Identification of the discrepancy's root cause and likely fix location.

**Non-goals**

- Production data mutation.
- Implementing a fix before the diagnosis is validated.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/director`; related analytics backend and ClickHouse schema repositories as needed.
- **Worktrees:** Director main checkout for read-only investigation.
- **Branches:** `director/main`.
- **PRs/commits:** none.

## Current Understanding

The source data is correct, but Assistance Insights' `RetrieveHintStats` aggregates incompatible units for behavioral hints: sent hints are distinct action annotation IDs while followed hints are distinct positive moment annotation IDs. A single sent hint can have several valid positive moments. Director then directly divides followed moments by sent actions, producing 160.4% for the reported week. Performance Insights separately uses `RetrieveQAScoreStats` and displays a QA criterion score, so the pages are not measuring identical populations even though an action-deduplicated hint rate is 79.1% and numerically matches the report.

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

## Blockers and Dependencies

- Fix ownership should include Insights backend because Director does not receive action IDs and cannot correctly deduplicate the aggregate response.

## Validation and Rollout

- Read-only Heartland production query reproduced both 160.4% and the action-deduplicated 79.1%.
- Verified at least eight additional recent Heartland user/policy rows where the current API ratio exceeded 100% while action-level ratios remained valid.

## Next Actions

1. Confirm that Behavioral Hint engagement is intended to mean “fraction of sent hints followed”; then change behavioral `hint_followed_count` to count distinct `adherence_action_annotation_id`.
2. Add query/RPC regression coverage for multiple positive moments tied to one hint action.
3. Optionally add frontend invariant telemetry or a non-misleading fallback; do not treat clamping as the fix.

## Timeline

- 2026-07-28 — Investigation opened from the linked Slack report.
- 2026-07-28 — Reproduced 160.4% vs 79.1% from Heartland ClickHouse and isolated the mixed-unit `RetrieveHintStats` aggregation. Evidence: `sessions/2026-07-28/codex-heartland-behavior-hints-mismatch.md`.
