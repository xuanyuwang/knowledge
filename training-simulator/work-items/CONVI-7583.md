# CONVI-7583: Complete the session-reporting backend

**Technical status:** In Progress in Linear; backend PR open with two minor review findings
**Primary domain:** `training-simulator`
**Primary subdomain:** `reporting`
**Official ticket:** [CONVI-7583](https://linear.app/cresta/issue/CONVI-7583/complete-the-training-simulator-session-reporting-backend)
**Last updated:** 2026-09-01

## Objective and Impact

- **Objective:** Make session reporting assignment-rooted, complete for zero-run users, uncapped by the task-run list RPC, explicit about result states, authorized, and operationally measurable.
- **Customer/system impact:** Every qualified session and assignee is represented with trustworthy attempts, current result state, score/pass denominators, and explicit safety behavior.
- **Role:** investigated and ticketed

## Scope

- Read qualified active assignments and audiences independently of task runs.
- Return tasks and assigned agents with zero runs.
- Reuse task-run List parsing/enrichment through an internal method with configurable page size and an explicit reporting safety bound.
- Count all assigned-agent conversation/quiz task runs within the session before latest-run selection.
- Preserve latest-attempt-first selection with a resource-ID tie break.
- Classify conversation and quiz results using the accepted collapsed overall semantics: the listed zero/false timeout, all-criteria-N/A, and failure cases are failure-equivalent for reporting.
- Correct aggregate denominators and optional/unavailable values.
- Enforce explicit reporting personas and row-level authorization.
- Add query-stage and incomplete/malformed-result telemetry plus boundary validation.

## Contract and Implementation Plan

- Add only `AgentPerformanceEntry.attempt_count = 8` as new public response data.
- Do not add an incomplete-count field: Director can derive the chosen incomplete grain from assignment audiences plus each agent-session status.
- Do not add a per-agent pass-rate field for Milestone 1: the shipped drawer already displays backend-derived pass/fail plus score. A rate over historical attempts is a separate, undefined metric.
- Keep `AGENT` on the stats RPC because the agent-facing Assigned Training Sessions flow consumes it; enforce self-only agent results and manageable-user manager results with backend user/ACL filtering.
- Reuse existing status, passed, score, and not-applicable fields; no result-state field, warning field, persistence column, or schema migration is needed after the accepted zero/false collapse.
- Preserve existing settled-result detection: conversation score presence and quiz submission presence. Missing pass semantics remain false rather than making the run incomplete.
- Plan: [CONVI-7583 backend implementation plan](../deliverables/convi-7583-backend-implementation-plan.md).
- Contract PR: [cresta-proto#9716](https://github.com/cresta/cresta-proto/pull/9716), commit `7d425251e5`.
- Contract PR status at the latest verification: open, mergeable, and green.
- Follow-up: correct the published field comment, which currently narrows attempts to required modules.
- Backend PR: [go-servers#31780](https://github.com/cresta/go-servers/pull/31780), latest commit `286c64633e`, from `/Users/xuanyu.wang/repos/go-servers-convi-7583` on `xw/convi-7583-session-reporting-backend`. It uses main's cresta-proto v2.21.28. Request user/group fields select matching tasks through `ListDirectorTasks`; every matched task is aggregated over its full stored audience. Current Director callers do not require `direct_team_only`; row-level ACL and agent self-scope remain separate work. Go and Bazel package tests, Gazelle, vet, formatting, and diff checks pass. Repository lint is blocked by the local golangci-lint binary's Go 1.24 build version versus the repository's Go 1.25 target.
- Interactive review artifact: `/Users/xuanyu.wang/repos/pr-31780-training-simulator-review.html`. The review confirmed one functional edge case: a zero-run task filtered to a non-first configured lesson can report metadata for the first lesson, plus one misleading-comment issue in the equal-time tie-break test. At review time the PR was one commit behind `main`.

## Current Behavior to Preserve

- Latest task run is selected before its result is tested.
- Conversation and quiz runs both participate in required-module session completion.
- Existing active-assignment lifecycle scope remains unchanged.

## Dependencies

- No longer technically blocked by CONVI-7582 for the purpose of distinguishing overall zero/false results; [the 2026-08-27 decision](../decisions/2026-08-27-collapse-overall-zero-false-results.md) supersedes that prerequisite.
- Blocks [CONVI-7584](https://linear.app/cresta/issue/CONVI-7584/complete-the-focused-director-updates-for-training-simulator-session).

## Open Decisions and Release Gates

- Confirm how a timed-out partial snapshot with applicable criteria or a nonzero score is classified; the accepted zero/false collapse does not define this case.
- Enforce self-only scope for agent-only callers and manageable-user scope for managers rather than trusting request filters; administrators retain their authorized customer/profile scope.
- Reconcile representative staging cases, including zero-run assignees, retries, quiz-containing sessions, and the old 1,000-run boundary.
- Validate explicit safety bounds, authorization failures, query-stage counts/latency telemetry, and deterministic latest-attempt tie breaking.
- Fix the zero-run requested-lesson fallback and its regression coverage; correct the multiple-attempt test comments before merge.

## Evidence

- 68 active staging tasks; 12 have no runs.
- 97 active-task assigned-agent facts; 39 have no run.
- 317 task runs, including 40 quiz runs; 48 retry groups and a maximum of 53 attempts.
- Current handler inherits a 1,000-row maximum and returns no task stats when its run set is empty.

## Timeline

- 2026-08-25 — Initially created for latest-attempt-first classification, then rewritten as the complete Milestone 1 session-backend correctness ticket after confirming latest-attempt-first is already the core current behavior. Remains assigned to `xuanyu.wang` and blocked by CONVI-7582.
- 2026-08-27 — Removed the technical dependency on CONVI-7582 for overall result-state distinction after Product accepted failure-equivalent reporting for the listed zero/false timeout, all-criteria-N/A, and failed cases. External Linear dependency state has not been changed here.
- 2026-08-31 — Revalidated the knowledge record and local workspace: no CONVI-7583 implementation worktree or branch is present, so the recorded backend scope and release gates remain gaps. Linear remains authoritative for official status.
- 2026-08-31 — Traced the current proto, handler, schema, tests, and initial Director reporting consumer. Determined that attempt count is the only new response datum and all other work is corrected loading/classification/aggregation over existing storage. Produced the implementation plan before creating a product-code worktree.
- 2026-08-31 — Corrected the authorization conclusion after tracing the second Director consumer: agent-facing Assigned Training Sessions calls the same stats RPC for progress/status/score, so `AGENT` must remain and the backend must enforce self-only rows. Created `/Users/xuanyu.wang/repos/cresta-proto-convi-7583` on `xw/convi-7583-session-stats-contract`, added only `attempt_count = 8`, and passed Buf lint/build plus `bazel build //cresta/v1/trainingsimulator:all`.
- 2026-08-31 — Committed `7d425251e5`, pushed the branch, and opened [cresta-proto#9716](https://github.com/cresta/cresta-proto/pull/9716). The PR is open and non-draft; initial CI is queued/in progress.
- 2026-08-31 — Created `/Users/xuanyu.wang/repos/go-servers-convi-7583` from `origin/main` commit `95068db42b` on `xw/convi-7583-session-reporting-backend`. Drafted the assignment-rooted backend shape: shared user/ACL filtering with agent self-only narrowing, task-audience intersection, direct tenant-scoped task-run reads with 500-ID chunks and a 100,000-row ceiling, batched conversation/quiz enrichment, pre-reduction attempt counts, deterministic latest selection, required-module-only aggregation, and zero-valued unavailable scores. Per request, no formatting or validation was run.
- 2026-08-31 — Simplified the backend proof-of-shape by removing user/group expansion, `direct_team_only`, row-level ACL, agent self-only narrowing, task-audience intersection, and all downstream dependence on an authorized-user set. The draft now lists tasks without audience filters and aggregates each qualified task's complete stored audience. These remain deliberately unsupported rather than implemented behavior.
- 2026-08-31 — Reverified the current Director response usage and Figma drawer. Only authoritative all-attempt count needs new public response data. The incomplete dashboard count is frontend-derivable, while per-agent result/score already exists; no additional pass-rate protobuf field is justified without a new historical-attempt metric definition.
- 2026-08-31 — Updated the Linear description to this verified boundary and removed the stale CONVI-7582 blocker. Linear status remains In Progress and CONVI-7583 continues to block CONVI-7584.
- 2026-08-31 — Rebuilt the go-servers draft after revalidation: retained existing explicit user/group filtering and audience intersection, made assignments the aggregation root, added bounded direct reads and batched outcome enrichment, counted attempts before deterministic latest selection, and added zero-run plus newer-incomplete-retry tests. `gofmt` and `git diff --check` pass; compilation succeeds when the pending `AttemptCount` references are temporarily excluded. The final branch now fails only on the unpublished generated field, while DB suite execution remains blocked in local PostgreSQL setup.
- 2026-08-31 — Upgraded go-servers to cresta-proto v2.21.27 and polished the implementation for review. Added filter-forwarding/intersection, deterministic tie-break, stale-module, mixed attempt-count, and 1,001-attempt coverage. `mage cleanBuild`, full Go package tests, Bazel package tests, Gazelle, vet, formatting, and diff checks pass. No commit was created.
- 2026-09-01 — Generated an interactive review page for [go-servers#31780](https://github.com/cresta/go-servers/pull/31780). Verified the assignment-rooted/full-audience design and identified two minor follow-ups: select the requested configured lesson for filtered zero-run tasks, and correct copied comments in the equal-time tie-break test. The artifact's 30 diff-hunk anchors and 15 cross-links passed static validation.
