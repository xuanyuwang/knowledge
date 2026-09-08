# CONVI-7584: Focused Director session-reporting updates

**Status:** draft PR open; branch review identified two follow-ups; Linear status remains authoritative
**Primary domain:** `training-simulator`
**Primary subdomain:** `reporting`
**Official ticket:** [CONVI-7584](https://linear.app/cresta/issue/CONVI-7584/complete-the-focused-director-updates-for-training-simulator-session)
**Last updated:** 2026-09-02

## Objective and Impact

- **Objective:** Add the two remaining visible Director data points after the corrected session backend lands, without rebuilding the existing Figma experience.
- **Customer/system impact:** Managers can see incomplete work across the filtered reporting set and authoritative attempt counts per agent while preserving the already-shipped result and score presentation.
- **Role:** investigated and ticketed

## Verified Current UI

- The session dashboard, table, review drawer, Passed/Failed/Incomplete filters, agent identity/date, View routing, and per-agent status/score tag already exist.
- Per-agent status comes from backend `AgentPerformanceEntry.status` and `passed`; the displayed percentage is the agent's score. It is not a missing per-agent pass-rate field.
- Empty session average-score and pass-rate values already render as unavailable (`—`).
- Director already joins the independently loaded assignment audience with optional stats, so zero-run assignments and assignees appear as incomplete even though the current stats RPC can omit them.

## Scope

- Display the incomplete count across the current filtered reporting set.
  - Use the Figma “Assigned agents” grain: unique agents with at least one incomplete visible assigned session, equivalently assigned unique agents minus agents complete in every visible assigned session.
  - This is derivable from Director's existing enriched rows; no protobuf field is required.
- Display authoritative `AgentPerformanceEntry.attempt_count` in each session-drawer agent row.
- Preserve existing backend-derived Passed/Failed/Incomplete plus score presentation; do not add a second “per-agent pass rate” unless Product separately defines a historical-attempt metric and denominator.
- Preserve unavailable rendering, View routing, and the defensive assignment join during backend rollout.
- Rename the internal `completedAt` field to `latestAttemptAt` because incomplete retries can also supply the latest-attempt timestamp.
- Add focused state, accessibility, loading, and error tests for the two visible additions.

## Non-goals

- Rebuilding the existing dashboard/table/drawer.
- Lesson/module reporting, new navigation, or CSV export.
- A new backend aggregate for the incomplete dashboard count.
- Historical attempt pass rate, first-pass rate, or improvement-between-attempts; these require separate metric definitions.
- Client-side official-attempt or score recomputation.

## Dependencies

- [CONVI-7583](https://linear.app/cresta/issue/CONVI-7583/complete-the-training-simulator-session-reporting-backend) and the `attempt_count` contract are merged. Linear still retains the stale blocker relation and Backlog status.

## Implementation

- Director worktree: `/Users/xuanyu.wang/repos/director-convi-7584`
- Branch: `convi-7584-session-reporting-fe`
- Commit after squash: `a84fb080df` (`[CONVI-7584] Complete session reporting frontend`)
- Draft PR: [director#22413](https://github.com/cresta/director/pull/22413)
- The dashboard now reports unique incomplete and overdue agents, matching Figma's two-pill Assigned agents card.
- The session drawer maps protobuf `int64` `attempt_count` from its generated string representation into a numeric view-model field and renders localized attempt text, including zero.
- `AgentResult.completedAt` was renamed to `latestAttemptAt`.
- `@cresta/web-client` was upgraded from `2.22.5` to `2.22.13`.
- Validation: 40 focused tests pass; Director app TypeScript check passes; pre-commit i18n, lint, i18next lint, and formatting hooks pass.
- Branch review: CodeRabbit reported zero findings. Manual review found no blocker in the changed logic, but loading badges can flash zero values and stats-query errors currently fall through to zero-attempt incomplete rows; decide whether to address these inherited states before PR.

## Release Gates

- Verify the unique-agent incomplete count against Figma and test passed, failed/failure-equivalent, incomplete, never-started, retry, and quiz-containing session rows.
- Verify attempt count includes all assigned-agent conversation and quiz task runs within the session, including runs for stale modules and retries.
- Add focused state, accessibility, loading, and error coverage without rebuilding the shipped dashboard/table/drawer.
- Confirm the UI treatment of any nonzero partial-timeout state after the backend contract is decided.

## Timeline

- 2026-08-25 — Created in the Training Simulator project / Convo Intelligence Backlog, assigned to `xuanyu.wang`, blocked by CONVI-7583.
- 2026-08-31 — Rebased the recorded frontend gap on the accepted 2026-08-27 result-state decision: distinct overall all-N/A presentation and legacy missing-status warnings are no longer required solely to distinguish the collapsed zero/false cases. No local CONVI-7584 implementation worktree or branch is present; Linear remains authoritative for official status.
- 2026-08-31 — Reverified current Director and Figma. The remaining visible data additions are the filtered-set incomplete count and drawer attempt count. Per-agent Passed/Failed/Incomplete plus score and unavailable aggregate rendering already exist. Only attempt count requires new backend response data; the incomplete count is derivable in Director.
- 2026-08-31 — Replaced the Linear description with the verified two-delta scope, explicit incomplete-count grain decision, response derivability, non-goals, and acceptance criteria. Linear status remains Backlog and blocked by CONVI-7583.
- 2026-09-02 — Reverified GitHub Director main after the backend merge. `SessionsDashboard` still shows assigned/completed/overdue rather than incomplete, while `AgentResult` and the review drawer do not carry or render backend `attempt_count`. These remain the two visible Milestone 1 implementation gaps.
- 2026-09-02 — Created `/Users/xuanyu.wang/repos/director-convi-7584` from current `origin/main`, implemented both visible gaps, renamed latest-attempt semantics, upgraded the generated client, added focused aggregation/mapping/drawer coverage, and committed locally.
- 2026-09-02 — Reviewed the local branch with CodeRabbit and an interactive canvas. Automated review found no issues; manual review recorded the loading-badge flash and stats-error fallback as two non-core follow-ups.
- 2026-09-02 — Renamed `latestActivityAt` to the more precise `latestAttemptAt`, revalidated 40 focused tests/typecheck/lint, and rebased both local commits onto the latest `origin/main`.
- 2026-09-02 — Squashed the two unpushed implementation commits into `a84fb080df`; the branch is one commit ahead of `origin/main`.
- 2026-09-02 — Pushed the branch and opened draft [director#22413](https://github.com/cresta/director/pull/22413).
