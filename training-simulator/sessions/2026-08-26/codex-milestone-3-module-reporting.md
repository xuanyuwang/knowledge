# Milestone 3 module reporting implementation

**Date:** 2026-08-26
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** contract, backend, and frontend implementation are isolated in `/Users/xuanyu.wang/repos/cresta-proto-milestone-3-module-reporting`, `/Users/xuanyu.wang/repos/go-servers-milestone-3-module-reporting`, and `/Users/xuanyu.wang/repos/director-milestone-3-module-reporting`, each on `codex/milestone-3-module-reporting`. The backend worktree is based on committed CONVI-7582 result persistence. The original dirty proto checkout remains preserved as user work.
**Scope:** Milestone 3 module-level Training Simulator reporting only.

> **Superseded transport (2026-08-28):** The assignment/result aggregation findings remain applicable, but the `ListTrainingModules` extension is no longer the selected public contract. Replace it with `RetrieveTrainingSimulatorModuleStats` per [the dedicated API decision](../../decisions/2026-08-28-dedicated-lesson-module-stats-apis.md).

## Requested outcomes

- Extend `ListTrainingModules` with module statistics without changing its non-statistics behavior or query cost.
- Calculate Active Lessons, average score, module passed/failed counts and pass rate, and criterion passed/failed counts and pass rate.
- Reuse latest-attempt-first selection and result classification, use current content definitions, exclude criterion N/A from denominators, and exclude quiz aggregates.
- Add only the module table values, module drawer, and criterion breakdown in Director using the existing Assignee and Date Range filters.
- Verify reporting authorization separately from content-list access, preserve authoring usability on reporting failure, reconcile representative results, and measure query/page behavior.

## Evidence labels

Findings in this session use these labels:

- **Observed behavior:** directly traced in code, schema, tests, or measured output.
- **Confirmed decision:** explicitly required by the Milestone 3 request or accepted design artifacts.
- **Proposed behavior:** implementation choice not yet established by authoritative behavior.
- **Unresolved assumption:** material question that cannot yet be established from the available evidence.

## Investigation log

- **Confirmed decision:** Option 2 is selected for Milestone 3; `ListTrainingModules` returns `module_stats` aligned one-to-one and in order with `training_modules` when requested.
- **Confirmed decision:** revision-exact historical reporting, quiz module aggregates, lesson-level counts/aggregation, and new tables/indexes without evidence are out of scope.
- **Observed behavior:** the local `cresta-proto` checkout is dirty with user-owned `module_stats.proto`, `lesson_stats.proto`, service request/response fields, and Bazel changes. Backend and Director main checkouts started clean.
- **Observed behavior:** `go-servers/main` does not contain persisted conversation `evaluation_status` or overall N/A. The clean CONVI-7582 worktree has committed schema/model/converter support and is therefore the dependency base for the isolated Milestone 3 backend worktree.
- **Observed behavior:** ordinary `ListTrainingModules` access includes `AGENT`, `MANAGER`, and `MANAGER_2ND`; the existing Lesson Configuration UI is restricted to global admins and `QA_ADMIN`. Statistics therefore need an explicit backend role check instead of inheriting list access.

## Module reporting mental model

- **Observed behavior:** a Training Simulator assignment is one `director.tasks` row. Its content config names a lesson and its Training Simulator audience config stores explicit user IDs. There is no persisted per-user module-assignment row.
- **Confirmed decision:** module assignment facts are derived from each matching assignment's audience and the current latest lesson definition. A fact is `(task_id, current_lesson_id, requested_module_id, agent_user_id)`.
- **Confirmed decision:** current latest lesson definitions determine whether an assigned lesson contains a requested module. This intentionally is not revision-exact historical reporting.
- **Observed behavior:** a task run stores task, lesson, module, conversation-score ID, quiz-score ID, and creation time. Conversation result identity and outcome live on `training_simulator_conversation_scores`, including agent ID, normalized score, pass, criterion results, and—on the CONVI-7582 base—evaluation status and overall N/A.
- **Confirmed decision:** select the latest conversation task run for each derived fact using `(created_at, resource_id)` before classifying its joined result. A newer unfinished retry hides an older completed result. Quiz runs are excluded from module reporting.
- **Confirmed decision:** only `EVALUATION_STATUS_COMPLETE` is settled. Settled overall N/A and incomplete/ambiguous results do not enter module score or pass/fail denominators. Criterion rows may be aggregated only from settled results, and each criterion's own N/A flag excludes it from that criterion denominator.
- **Confirmed decision:** module pass rate is `passed / (passed + failed)` and criterion pass rate uses the same denominator. Optional rates and average score are absent at a zero denominator.
- **Observed behavior:** persisted criterion rows carry criterion ID, stable behavior resource name, display name, pass/N/A, weight, and auto-fail result. Module statistics use those persisted values; current module criteria are only a display-name fallback.
- **Confirmed decision:** criterion grouping uses behavior resource name when present. For historical criteria without a behavior name, the deterministic internal fallback key is `(module_id, criterion_id)`; the response keeps `behavior_name` empty and exposes the persisted criterion ID. This avoids accidentally merging nameless criteria across modules while preserving an explainable row within one module.
- **Confirmed decision:** Active Lessons counts distinct current latest `STATE_ACTIVE` lesson definitions containing the module and is independent of assignee/date filters.

## Lesson-to-module transition decision

- **Observed behavior:** Milestone 2's lesson reporting drawer is not implemented in the current Director checkout, and the selected Option 2 module request has no lesson-scope reporting field.
- **Proposed behavior:** the module reporting page remains the canonical all-current-lessons aggregate for the selected assignee/date filters. A future lesson-drawer module chip should navigate to the module row/drawer while preserving those global filters, but it must not imply numerical equality with the lesson-scoped chip. The target drawer should disclose that its statistics cover all lessons.
- **Confirmed decision for this implementation:** do not wire a lesson-to-module transition in Milestone 3 because the source lesson drawer does not exist and adding a lesson-scope API field would exceed the selected contract. When Milestone 2 lands, the transition must either use the documented all-lessons behavior or introduce an explicitly reviewed lesson-scope filter; it must never silently compare the two grains as equal.

## Implementation outcome

- **Observed behavior:** the handwritten proto extends `ListTrainingModulesRequest` with `include_stats`, `stats_time_range`, and direct user fields. Group/team filters were removed on 2026-08-28. `ListTrainingModulesResponse.module_stats` is an ordered parallel list. No `TrainingSimulatorStatsFilter` or generated source was added.
- **Observed behavior:** `include_stats=false` returns through the existing content path before any reporting loader call. `include_stats=true` performs a separate reporting authorization check and then executes one page-level loader; there is no reporting query inside the module aggregation loop.
- **Observed behavior:** reporting loading, latest-attempt selection, persisted-result normalization, module aggregation, and criterion aggregation are separate functions. Loader limits are 2,000 current lessons, 2,000 assignments, and 10,000 task runs; exceeding a bound returns `RESOURCE_EXHAUSTED` instead of silently truncating statistics.
- **Observed behavior:** the loader emits observed row counts and elapsed milliseconds for lesson, task, and run/result stages. It uses four bounded SQL statements after the ordinary module list: current lessons, matching assignments, matching conversation runs, and joined conversation-score rows. User/group expansion is an additional external user-filter operation only when assignee filters are present.
- **Observed behavior:** Director issues one combined list request for the module table, passes the existing Assignee and Date Range state, validates parallel response alignment, and never issues a request per table row. On reporting failure it retries the existing content-only list and renders reporting fields as `--`, preserving module authoring links and controls.
- **Confirmed decision:** module reporting table values, drawer, criterion breakdown, and performance action are gated with the existing `enableTrainingSimulatorV2` module surface. Quiz modules do not expose the performance action.
- **Confirmed decision:** no lesson-drawer transition is wired. The module drawer explicitly states that it covers all current lessons for the selected global filters.

## Worked denominator example and reconciliation

The aggregation fixture represents one module in three active lessons and four underlying assignment facts:

| Agent-module latest result | Module contribution | Empathy criterion | Resolution criterion |
|---|---:|---:|---:|
| Agent A: score 0.90, pass | pass | pass | N/A |
| Agent B: score 0.60, fail | fail | fail | pass |
| Agent C: overall N/A | excluded | N/A | N/A |
| Agent D: incomplete | excluded | absent | absent |

- **Observed behavior:** the reconciled module denominator is `1 passed + 1 failed = 2`; average score is `(0.90 + 0.60) / 2 = 0.75`, and pass rate is `1 / 2 = 50%`.
- **Observed behavior:** Empathy excludes Agent C's N/A result, so its denominator is `1 passed + 1 failed = 2` and its pass rate is `50%`.
- **Observed behavior:** Resolution excludes Agents A and C as N/A, so its denominator is `1 passed + 0 failed = 1` and its pass rate is `100%`.
- **Observed behavior:** Active Lessons is `3` and is independent of the agent and date filters.
- **Observed behavior:** the source fixture asserts the ordered module response and these underlying agent-module outcomes in `module_reporting_test.go`. Test execution is currently blocked by intentionally absent generated dependencies, described below.

## Validation and measured behavior

- **Observed behavior:** `bazel build //cresta/v1/trainingsimulator:trainingsimulator_proto` passed in the proto worktree. The build produced only Bazel output; no generated source appeared in `git status`.
- **Observed behavior:** `git diff --check` passed in the proto, backend, and frontend worktrees. Go sources were formatted with `gofmt`; Director sources were formatted with the repository Biome binary.
- **Observed behavior:** the focused Go test command reached compilation and stopped because generated CONVI-7582 fields (`EvaluationStatus` and `EvaluationNotApplicable`) are absent from the checked-in generated DB/proto models. Generated artifacts were intentionally not produced.
- **Observed behavior:** the focused Director Vitest command could not start from the isolated worktree because dependencies are not installed there; using the main checkout's runner still cannot resolve the worktree's Vite dependencies. The new web-client request/response types also require the normal proto generation/dependency update.
- **Unresolved assumption:** representative database timings and a browser page-load trace cannot be measured until normal generated artifacts are available and the backend/frontend can run. The implementation records stage durations and result counts for that measurement, but this session makes no latency target or page-load performance claim.
- **Unresolved assumption:** generated field names and TypeScript shapes must be reconciled after the normal generation pipeline runs. The handwritten names follow the local proto and CONVI-7582 source definitions but have not been compiler-verified end to end.

## Remaining blockers

1. Run the repository's normal proto/GORM/web-client generation outside this no-generation task, then compile and execute the focused Go and Director tests.
2. Run the module page against representative data, capture logged query counts/stage timings and a browser page-load trace, and compare the returned parallel module statistics to the underlying agent-module rows.
3. Resolve any generated-name mismatch without expanding into Milestone 1, Milestone 2, lesson aggregation, or quiz aggregation.

## Proto pull request

- **Observed behavior:** rebased the three handwritten proto files onto current `origin/main` as `1a7cafc6b3`; [cresta-proto PR #9676](https://github.com/cresta/cresta-proto/pull/9676) targets `main` and contains only the module-reporting contract.
- **Observed behavior:** created [CONVI-7601](https://linear.app/cresta/issue/CONVI-7601/add-training-simulator-module-reporting-api-contract) for the module-reporting proto contract and updated PR #9676 to include the ticket identifier.
- **Confirmed decision:** evaluation-result persistence [PR #9656](https://github.com/cresta/cresta-proto/pull/9656), lesson reporting [PR #9677](https://github.com/cresta/cresta-proto/pull/9677), and module reporting PR #9676 all target `main` independently and cross-link as one review series.

## 2026-08-27 API-lint follow-up

- **Observed behavior:** PR #9676 failed API lint because the new reporting filter fields lacked explicit `OPTIONAL` field behavior and AIP-132 does not ordinarily permit an aligned statistics projection in a list response.
- **Confirmed decision:** Annotate the module list request fields explicitly and retain `module_stats` as a one-to-one reporting projection, with a narrowly documented `core::0132::response-unknown-fields` suppression.
- Commit `05e4c9b756` is on the PR branch. Repository CI run `33111102325`, job `98653832549`, passed `Lint API`; local Buf, Bazel, and whitespace validation also passed.

## 2026-08-28 count-semantics review

- **Confirmed decision:** Group and team selection is not required yet; remove `stats_virtual_group_names` and `stats_team_group_names` from the lesson and module reporting contracts while retaining direct user selection.
- **Observed behavior:** PR #9676 review comments `r3875681846` and `r3875693048` recommend replacing failed counts with a module-level total because incomplete facts do not fit a strict passed/failed partition. The comments do not explicitly define N/A as failure.
- **Confirmed decision:** Expose `passed_count` and `total_count`, not `failed_count` or `pass_rate`. The residual `total_count - passed_count` deliberately combines failures with other non-pass states instead of falsely claiming they are all failures. Clients may derive a presentation percentage when `total_count > 0`.
- **Proposed behavior:** Define module `total_count` as all matching assignment-rooted agent-module facts, including no-run, latest-attempt-incomplete, and completed N/A facts. `passed_count` counts only completed passes. Criterion statistics carry only `passed_count` and use the parent module's `total_count` as their shared denominator.
- **Unresolved assumption:** Using one module-level denominator for every current criterion treats incomplete facts, criterion-level N/A, and historical results without a current criterion as non-passes. This needs explicit confirmation because revision-exact criterion reporting remains out of scope.
- **Observed behavior:** Implemented the count contract in commit `6b11a998c6` on PR #9676 and removed group/team filters from lesson PR #9677 in commit `cd21847e66`. Replied to review comments `r3875681846` and `r3875693048` and resolved both threads. Local Buf/Bazel/whitespace checks passed; authoritative #9676 API lint passed in run `33174279406`, job `98858756278`.
- **Observed behavior:** the PR contains 103 additions across `BUILD.bazel`, `module_stats.proto`, and `training_simulator_service.proto`; no generated files are included. Required CI checks were pending immediately after creation.
