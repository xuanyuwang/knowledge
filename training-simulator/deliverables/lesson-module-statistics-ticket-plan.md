# Training Simulator statistics — implementation ticket plan

> **Superseded requirement (2026-08-27):** TS-04 and statements making persisted overall evaluation status/N/A a ship blocker are no longer current. Product accepted failure-equivalent reporting for the listed zero/false timeout, all-criteria-N/A, and failed cases. Criterion-level N/A remains excluded from scoring. See [the decision record](../decisions/2026-08-27-collapse-overall-zero-false-results.md).

Date: 2026-08-24

Status: Superseded by the simpler three-milestone plan in `superhuman-api-design-update-draft.md`. Do not create the 18 tickets below; retain this document only as detailed implementation reference material.

Source design: [Training Simulator Lesson and Module Statistics — Engineering Design](superhuman-api-design-update-draft.md)

## Planning assumptions

- Use **Option 2** as the working API shape: add optional statistics to `ListTrainingLessons` and `ListTrainingModules`.
- Option 2 is not final until ticket `TS-00` closes the API review. If another option is selected, the shared persistence, loading, calculation, session, and most frontend tickets remain valid; only the public transport and frontend request wiring change.
- Reporting reads assignments, current content, task runs, and conversation-score rows directly. It does not aggregate a paginated `ListTrainingSimulatorTaskRuns` response.
- Lesson and module fields follow Figma exactly. Quiz reporting and revision-exact historical reporting are out of scope.
- Initial reporting access is `QA_ADMIN`, `ADMIN`, and `SUPER_ADMIN`.
- `S`, `M`, and `L` below are relative sizes, not calendar estimates.

## Dependency map

```text
TS-00 Contract gate
  ├─ TS-01 Proto contract ───────────────┬─ TS-04 Result persistence
  │                                      ├─ TS-09 FE adapters and mocks
  │                                      └─ TS-08 Lesson/module list integration
  ├─ TS-02 Calculation fixtures ── TS-05 Aggregator ─┬─ TS-07 Session stats
  └─ TS-03 Query spike ─────────── TS-06 DB loaders ─┴─ TS-08 Lesson/module list integration

TS-09 ─ TS-10 Filter/list wiring ─┬─ TS-11 Lesson table ─ TS-13 Lesson drawer ─┐
                                 └─ TS-12 Module table ─ TS-14 Module drawer ─┤
TS-07 + TS-08 ─ TS-15 BE hardening ───────────────────────────────────────────┤
TS-13 + TS-14 + TS-15 ─ TS-16 FE polish/E2E ─ TS-17 staging and rollout ──────┘
```

The number is the recommended planning order, not a requirement to serialize every ticket. Work in the same dependency layer can run in parallel.

## Tickets

### TS-00 — Freeze reporting contracts and product semantics

- **Area:** Product + design + BE + FE + security
- **Size:** S
- **Hard dependencies:** None
- **Deliverable:** Record the selected API option and close the remaining lifecycle, date, N/A, authorization, empty-value, warning, ordering, and latency decisions.
- **Acceptance criteria:**
  - Select Option 2 or explicitly replace the working assumption.
  - Confirm `ACTIVE` and `ARCHIVED` task inclusion, `DRAFT` and `DELETED` exclusion, and task active-window overlap for the date filter.
  - Confirm latest-attempt selection, all-N/A display, score scale, absent optional values, and missing-result-status warning behavior.
  - Confirm initial roles and that selected users/groups narrow results but never grant access.
  - Confirm Option 2's same-length/same-order response rule and whether a requested statistics failure fails the list request. The working choice is to fail the request when `stats_filter` is present.
  - Agree on measurable cold and cached page latency targets.

### TS-01 — Add the additive protobuf contract

- **Area/repo:** API contract / `cresta-proto`
- **Size:** S
- **Hard dependencies:** TS-00
- **Deliverable:** Add the shared statistics filter/result messages, Option 2 request/response fields, and the session-side-panel fields.
- **Acceptance criteria:**
  - Add `TrainingSimulatorStatsFilter` with separate user, virtual-group, and team-group names.
  - Add the exact Figma-scoped lesson, module, module-chip, and criterion statistics messages.
  - Add optional `stats_filter` and parallel statistics arrays to the lesson/module list APIs without changing behavior when the filter is absent.
  - Add `attempt_count` and `has_missing_result_status` to `AgentPerformanceEntry`.
  - Preserve protobuf field compatibility; generated clients compile in Go and Director.

### TS-02 — Create calculation fixtures and invariant tests

- **Area/repo:** Backend tests / `go-servers`
- **Size:** S
- **Hard dependencies:** TS-00
- **Deliverable:** A table-driven, storage-independent fixture suite that defines the expected reporting answers before loader work begins.
- **Acceptance criteria:**
  - Cover zero assignments, assigned users with zero runs, and multiple tasks/agents/modules.
  - Cover latest-attempt ordering and deterministic timestamp ties.
  - Cover passed, failed, incomplete, all-N/A, mixed N/A/applicable, pending/in-progress evaluation, and ambiguous legacy rows.
  - Verify lesson weighting, module/criterion denominators, session attempt counts, and absent values for empty denominators.
  - Assert `passed_count + failed_count` denominator and exclusion of N/A/incomplete rows.

### TS-03 — Validate the direct database query plan and safety bounds

- **Area/repo:** Backend/data / `go-servers`
- **Size:** S–M
- **Hard dependencies:** TS-00
- **Deliverable:** A documented query spike using representative task, assignment, run, and conversation-score volumes.
- **Acceptance criteria:**
  - Demonstrate direct, task-scoped reads with `EXPLAIN (ANALYZE, BUFFERS)`.
  - Test the planned 200-content, 2,000-assignment, and 10,000-run envelope.
  - Define chunk sizes and explicit `RESOURCE_EXHAUSTED` bounds; no silent truncation is allowed.
  - Decide from measurements whether an additional index is necessary. Do not add speculative lesson/module indexes.
  - Record query-stage latency and the chosen representative fixtures.

### TS-04 — Persist evaluation status and overall N/A

- **Area/repo:** API + backend storage / `cresta-proto`, `go-servers`
- **Size:** M
- **Hard dependencies:** TS-01
- **Deliverable:** Persist `evaluation_status` and overall `not_applicable` on the existing `director.training_simulator_conversation_scores` row through the evaluation update path.
- **Acceptance criteria:**
  - Add backward-compatible nullable storage fields and migration handling.
  - Carry both values from `EvaluateTrainingConversationResponse` through `UpdateTrainingSimulatorTaskRun`.
  - Write status, N/A, score, passed, and criterion results in the same transaction.
  - Never treat pending, in-progress, or a partial timeout snapshot as a settled result.
  - Do not modify `app.chats` or add another result table.
  - Add write-path and migration/fallback tests.

### TS-05 — Implement the pure statistics calculation library

- **Area/repo:** Backend / `go-servers`
- **Size:** M
- **Hard dependencies:** TS-02
- **Deliverable:** Storage-independent functions that reduce assignment facts and normalized conversation results into session, lesson, module, and criterion statistics.
- **Acceptance criteria:**
  - Select the latest run per `(task, lesson, module, agent)` using resource ID as the tie-breaker.
  - Count all matching runs before latest-attempt reduction for session `attempt_count`.
  - Implement only the statistics defined in the design and Figma.
  - Return absent score/rate values for empty denominators and mark ambiguous legacy data.
  - Pass all TS-02 fixtures without database setup.

### TS-06 — Build the shared assignment and attempt loaders

- **Area/repo:** Backend / `go-servers`
- **Size:** L
- **Hard dependencies:** TS-03
- **Integration dependency:** TS-04 before the reporting feature can ship
- **Deliverable:** Bounded loaders shared by the session and content reporting handlers.
- **Acceptance criteria:**
  - Resolve authorized/requested users, including separate virtual-group and team-group inputs; a selected empty group returns no assignments.
  - Load qualified Training Simulator DirectorTasks and expand every audience member into lesson/module/session assignment facts, including users with no runs.
  - Load current lesson/module definitions, qualified conversation task runs, and conversation-score rows in bounded batches.
  - Exclude quiz runs and runs that cannot be attributed to a matching assignment fact.
  - Keep loading, normalization, and pure calculation separate.
  - Emit explicit errors at safety bounds rather than returning partial data.

### TS-07 — Complete session-level reporting in `RetrieveTrainingSimulatorTaskStats`

- **Area/repo:** Backend / `go-servers`
- **Size:** M
- **Hard dependencies:** TS-04, TS-05, TS-06
- **Deliverable:** Fill the missing session-level behavior without changing the session API boundary.
- **Acceptance criteria:**
  - Return a `TaskStats` row for a qualified session with no runs and an agent row for every assigned agent.
  - Populate agent passed/failed/incomplete/N/A state correctly.
  - Populate all-run `attempt_count`, most-recent run time through existing task runs, agent identity, score percentage inputs, and warning state.
  - Read qualified runs directly and do not inherit the task-run list's 1,000-row limit.
  - Preserve existing fields and Conversation Review link inputs.
  - Add handler/database integration tests, including uncapped attempt-count coverage.

### TS-08 — Add lesson/module statistics to the list APIs

- **Area/repo:** Backend / `go-servers`
- **Size:** L
- **Hard dependencies:** TS-01, TS-04, TS-05, TS-06
- **Deliverable:** Implement Option 2 in `ListTrainingLessons` and `ListTrainingModules`.
- **Acceptance criteria:**
  - Preserve existing list behavior and query cost when `stats_filter` is absent.
  - With `stats_filter`, calculate one bounded batch for the returned page—never one query per row.
  - Return statistics arrays with the same length and order as the content arrays.
  - Return zero-count/absent-rate rows for valid content with no matching assignments.
  - Enforce reporting authorization separately from content-authoring access.
  - Apply the agreed failure behavior when statistics are requested.
  - Add request validation, ordering, pagination, authorization, and integration tests.

### TS-09 — Add Director adapters, view models, and mocks

- **Area/repo:** Frontend / `director`
- **Size:** S
- **Hard dependencies:** TS-01
- **Deliverable:** Transport-facing types plus pure display models that allow UI work to proceed before the backend is live.
- **Acceptance criteria:**
  - Map normalized API values to percentages once at the display boundary.
  - Preserve absent values as `null`/`--`, not zero.
  - Index or align statistics safely by full resource name and verify the server ordering contract.
  - Model missing-result-status warnings and complete all-N/A separately from failure.
  - Provide realistic zero, loading, error, legacy-warning, lesson, module, and criterion fixtures.
  - Add pure adapter tests.

### TS-10 — Thread filters into lesson/module list requests

- **Area/repo:** Frontend / `director`
- **Size:** M
- **Hard dependencies:** TS-09
- **Deliverable:** Reuse the existing Assignee and Date Range state when requesting lesson/module list statistics.
- **Acceptance criteria:**
  - Thread `filterState` through `TrainingSimulatorTabs` → `LessonConfiguration` → both sub-tabs.
  - Map users, virtual groups, and teams to their separate API fields without expanding membership in Director.
  - Include the statistics filter in stable React Query keys and refetch on relevant changes.
  - A selected group with no members cannot become an all-user query.
  - Do not add a new filter UI or per-row requests.
  - Add hook/request tests using mocks.

### TS-11 — Replace lesson-table placeholders

- **Area/repo:** Frontend / `director`
- **Size:** S
- **Hard dependencies:** TS-10
- **Live-data dependency:** TS-08
- **Deliverable:** Display Figma's lesson-level values in the existing table while preserving authoring behavior.
- **Acceptance criteria:**
  - Show Sessions assigned, average score, and pass rate only.
  - Preserve existing edit links, pagination, sorting behavior, and content metadata.
  - Render loading/error/empty/warning states without blocking authoring controls.
  - Add focused component tests.

### TS-12 — Replace module-table placeholders

- **Area/repo:** Frontend / `director`
- **Size:** S
- **Hard dependencies:** TS-10
- **Live-data dependency:** TS-08
- **Deliverable:** Display Figma's module-level values in the existing table while preserving authoring behavior.
- **Acceptance criteria:**
  - Show Active Lessons, average score, and pass fraction/rate only.
  - Preserve existing edit links, pagination, sorting behavior, and content metadata.
  - Render loading/error/empty/warning states without blocking authoring controls.
  - Add focused component tests.

### TS-13 — Build the lesson reporting drawer and module drill-through

- **Area/repo:** Frontend / `director`
- **Size:** M
- **Hard dependencies:** TS-11
- **Live-data dependency:** TS-08
- **Deliverable:** Figma-aligned lesson drawer using the lesson statistics already returned with the list page.
- **Acceptance criteria:**
  - Show lesson metadata, Sessions assigned, average score, pass rate, and one pass-rate chip per required module.
  - Keep only the selected lesson resource name in drawer state so filter refetches update an open drawer.
  - Support the specified module drill-through without issuing per-row requests.
  - Cover loading, empty, error, and missing-result-status states.
  - Add component tests for open, refresh, drill-through, and close/focus return.

### TS-14 — Build the module reporting drawer and criterion breakdown

- **Area/repo:** Frontend / `director`
- **Size:** M
- **Hard dependencies:** TS-12
- **Live-data dependency:** TS-08
- **Deliverable:** Figma-aligned module drawer using the module and criterion statistics already returned with the list page.
- **Acceptance criteria:**
  - Show Active Lessons, average score, passed/failed fraction and rate, current threshold, and criterion pass fractions.
  - Exclude N/A criterion results from displayed denominators.
  - Keep only the selected module resource name in drawer state so filter refetches update an open drawer.
  - Cover loading, empty, error, and missing-result-status states.
  - Add component tests for criterion ordering and all display states.

### TS-15 — Harden backend authorization, observability, and performance

- **Area/repo:** Backend/security/data / `go-servers`
- **Size:** M
- **Hard dependencies:** TS-07, TS-08
- **Deliverable:** Release-quality controls and evidence around both the updated session RPC and Option 2 list APIs.
- **Acceptance criteria:**
  - Verify initial-role access, customer/profile isolation, and requested-user intersection in integration tests.
  - Record RPC/query-stage latency, counts of tasks/assignment facts/runs/scores, missing status, orphan rows, and invariant violations.
  - Meet the agreed representative-data latency target or document and fix the measured bottleneck.
  - Verify no silent truncation and explicit safety-bound errors.
  - Put new reporting behavior behind `enableTrainingSimulatorV2` or the selected dedicated flag.

### TS-16 — Finish frontend warnings, accessibility, localization, and end-to-end tests

- **Area/repo:** Frontend + QA / `director`
- **Size:** M
- **Hard dependencies:** TS-07, TS-13, TS-14, TS-15
- **Deliverable:** A production-ready session/lesson/module reporting experience.
- **Acceptance criteria:**
  - Show incomplete assigned agents and the per-agent session drawer values, including attempt count and N/A-aware status.
  - Render ambiguous legacy score/pass values as `--` with a warning, never as 0%, failed, or N/A.
  - Use localized text, real buttons/links, non-color-only status meaning, accessible drawer labels, and correct focus return.
  - Add end-to-end coverage for filters, zero-run agents/content, lesson and module drawers, session agent rows, and Conversation Review links.
  - Confirm there is no per-row request fan-out.

### TS-17 — Reconcile staging data and roll out gradually

- **Area:** BE + FE + QA + product + operations
- **Size:** M
- **Hard dependencies:** TS-15, TS-16
- **Deliverable:** Staging correctness evidence and a monitored customer rollout.
- **Acceptance criteria:**
  - Manually reconcile selected staging users across session, lesson, module, and criterion levels, including no-run and missing-status cases.
  - Measure cold first-tab, cold tab-switch, and cached tab-switch behavior.
  - Confirm metric invariants, warning counts, orphan counts, latency, and error dashboards.
  - Enable the feature for an internal/QA group first, then a small customer group, with explicit rollback criteria.
  - Record the known current-content historical limitation and quiz exclusion in release notes/support guidance.

## Recommended execution waves

### Wave 1 — Close small gates and define the answers

Run TS-00 first. Then run TS-01, TS-02, and TS-03 in parallel. These are the smallest useful tickets and remove ambiguity for every larger implementation ticket.

### Wave 2 — Build independent foundations in parallel

Run TS-04, TS-05, and TS-09 in parallel. Start TS-06 after TS-03. Frontend work uses TS-09 mocks and does not wait for backend handlers.

### Wave 3 — Connect the product paths

Run TS-07 and TS-08 once persistence, loaders, and calculations are ready. In parallel, continue TS-10 and then TS-11/TS-12 using mocks. Connect those tables to live data as TS-08 becomes available.

### Wave 4 — Complete the deeper UI and hardening

Run TS-13 and TS-14 in parallel. Run TS-15 after both backend handler tickets. This keeps performance and security verification against the real query paths rather than prototypes.

### Wave 5 — Validate and release

Run TS-16 against the integrated backend, then TS-17. Do not enable customers before persistence correctness, authorization, no-truncation behavior, and cross-level reconciliation pass.

## Critical path

The likely critical path is:

```text
TS-00 → TS-03 → TS-06 → TS-08 → TS-15 → TS-16 → TS-17
```

TS-04 is also a ship blocker even if it is not the longest engineering path. UI development can proceed using mocks, but production statistics must not ship until new evaluation results persist explicit completion and overall N/A state.
