# CONVI-7600 backend two-step implementation plan

Date: 2026-09-02 (revised 2026-09-03 after CONVI-7601 module-stats implementation)
Source repo: `/Users/xuanyu.wang/repos/go-servers`
Target worktree: `/Users/xuanyu.wang/repos/go-servers-milestone-2`
Current branch: `codex/milestone-2-lesson-reporting`
Reference implementation: CONVI-7601 `action_retrieve_training_simulator_module_stats.go` on `codex/milestone-3-module-reporting` ([PR #31952](https://github.com/cresta/go-servers/pull/31952), commit `5bca084abf`)

## Current state

- The milestone-2 worktree is far behind `origin/main`, has one obsolete local commit, tracked changes that extend `ListTrainingLessons`, and untracked lesson-stat implementation/test files based on the superseded list contract. Do not replay any of that.
- `origin/main` now includes:
  - cresta-proto `v2.22.11` with the merged dedicated `RetrieveTrainingSimulatorLessonStats` and `RetrieveTrainingSimulatorModuleStats` contracts;
  - merged CONVI-7583 assignment-rooted session reporting (`f376619355`);
  - merged or in-review CONVI-7601 module reporting ([PR #31952](https://github.com/cresta/go-servers/pull/31952)).
- The published lesson response is **flat**: `training_lesson_name`, `session_count`, `average_applicable_score`, `applicable_score_count`, `passed_assignment_count`, `total_assignment_count`. Nested module stats and `has_missing_result_status` were removed; the lesson drawer lazily calls `RetrieveTrainingSimulatorModuleStats` with `training_lesson_names` scope.
- The module-stats implementation is the canonical template for query shape, score normalization, latest-attempt selection, time-window overlap, and the two-step review workflow. Evidence from its review: `sessions/2026-09-03/claude-convi-7601-module-stats-review.md`.

## Worktree preparation

Preserve the old dirty draft in a named stash, then switch the existing worktree to a fresh CONVI-7600 backend branch created from current `origin/main`. Do not replay the obsolete CONVI-7582 commit or the old `ListTrainingLessons` integration. Use the old draft and the shipped module-stats action only as reference.

## Lessons from CONVI-7601 (apply on lesson from day one)

These are correctness and contract items discovered during module-stats development. The lesson API should not repeat them.

| Topic | Module experience | Lesson plan |
|---|---|---|
| Score scale | `training_simulator_conversation_scores.score` and `quiz_scores.score` are stored **0–1** end to end (Director divides evaluator output by 100 before persist). Dividing conversation scores by 100 again produced 100×-too-small averages. | Use stored values verbatim. Seed tests with realistic `0.80`, not `80`. |
| Lesson revision state | Filtering `state = ACTIVE` **before** `MAX(created_at)` counts archived lessons as active when an older ACTIVE revision exists. | For current lesson definitions, compute `MAX(created_at)` over all revisions, join the latest row, **then** filter `state = ACTIVE` (same pattern as `action_list_training_lessons.go`). |
| Response order | Iterating a module-ID set produced nondeterministic `module_stats` order; contract requires request order with one-to-one length. | Keep `orderedLessonIDs` / `orderedLessonNames` from parse through aggregation. Build the response by walking the ordered slice, not a set/map. Add an E2E test that asserts index order, not a lookup map. |
| Empty request list | Module temporarily allowed empty `training_module_names` → all profile modules, which breaks the published one-to-one response contract. | **Conform to proto**: reject empty `training_lesson_names` (`InvalidArgument`). Director always sends the visible lesson page. |
| Duplicate names | Module silently deduped duplicates, breaking one-to-one alignment. | Reject duplicate lesson names in the request, or echo duplicates in the response — pick one and test it. Prefer reject (simpler for Director batching). |
| Quiz modules | Early eng design excluded quiz; module stats and session stats both include quiz with raw 0–1 score and `score >= passing_score/100` pass derivation. | Include quiz modules in lesson pass/score rollups using the same rules as module stats. Do not port the obsolete “quiz excluded” draft behavior. |
| Latest attempt | Assignment key needs `agent_id` from score rows; batch-load runs + scores, sort per `(task, lesson, module, agent)` in Go (same as CONVI-7583 / module stats). | Reuse the same latest-attempt sort key: `created_at` asc, tie-break `resource_id` asc, take last. |
| Time window | `created_at <= end AND (due_time IS NULL OR due_time >= start)` on DirectorTasks; inclusive bounds; attempt timestamps are not filters. | Copy the inline due-time JSON path and predicate from module stats; do not invent a second variant. |
| Query naming | `find*` helpers for each batched DB read; one action file; preparation vs aggregation split. | Mirror the naming convention (`findLessonStatsLessonMetadata`, `findLessonAssignmentKeys`, …). |
| Read bounds | Module removed run caps during development; session stats still probes at 100k runs. | Restore an overflow probe for lesson path before release, or document measured bounds. Do not ship unbounded reads by default. |

## Aggregation semantics

Assignment grain for lesson rollups: `(director_task_id, lesson_id, agent_id)`.

Per assignment, using **current** lesson module membership:

1. For each required module in the current lesson definition, take the official latest attempt for that `(task, lesson, module, agent)`.
2. **Complete** — every required module has a settled latest attempt with a resolvable score link.
3. **Passed** — complete and every required module passes (conversation: stored `passed`; quiz: `score >= passing_score/100` from current module config).
4. **Applicable lesson score** — when complete, the simple arithmetic mean of required module normalized scores (0–1). Incomplete or never-started assignments contribute to `total_assignment_count` but not to `passed_assignment_count` or score numerators.
5. **Stale modules** — attempts for module IDs no longer in the current lesson definition are ignored (current-content semantics, same as module/session reporting).
6. **`session_count`** — count of distinct `director_task_id` values that contribute at least one assignment fact for the lesson within the filtered scope (not “started” or “completed” sessions).

Overall zero/false timeout and all-criteria-N/A outcomes follow the accepted failure-equivalent reporting decision. Do not reintroduce `has_missing_result_status` or separate N/A aggregate states.

## Pipeline shape (lesson-first)

Module stats discovers **module → lessons → tasks**. Lesson stats inverts that:

```
parseLessonStatsRequest
  → findLessonStatsLessonMetadata
  → findLessonStatsDirectorTasks
  → findLessonAssignmentKeys
  → findLatestLessonModuleAttemptResults
  → aggregateLessonStats
```

`findLessonStatsLessonMetadata` loads current lesson rows (latest revision, ACTIVE only), module membership, and display metadata for requested lesson IDs.

`findLessonStatsDirectorTasks` selects ACTIVE and ARCHIVED Training Simulator DirectorTasks whose assigned lesson set intersects the requested lessons and whose assignment window overlaps `time_range`.

`findLessonAssignmentKeys` expands each matched task’s audience to `(director_task_id, lesson_id, agent_id)` for lessons that are both requested and assigned on the task.

`findLatestLessonModuleAttemptResults` batch-loads task runs and conversation/quiz score rows for those keys, then picks the latest attempt per `(task, lesson, module, agent)` in memory.

`aggregateLessonStats` rolls module-level latest results up to lesson assignments, then to per-lesson proto fields.

Do not route reporting through `ListTrainingSimulatorTaskRuns`. Do not port the obsolete draft’s user/group filters, nested module results, or missing-result warning classification.

## Two review steps

### Step 1: request, reads, and aggregation input

Add one production file, `action_retrieve_training_simulator_lesson_stats.go`, plus its test file and BUILD entries.

Implement:

1. Parse and validate the profile parent, requested lesson names (nonempty, unique, valid resource names under parent), and time range.
2. Preserve caller order in `orderedLessonIDs` / `orderedLessonNames` for the full pipeline.
3. Implement the four `find*` helpers above: current lesson definitions, overlapping DirectorTasks, lesson assignment keys, and latest module attempts (conversation + quiz).
4. Emit one `lessonStatsFact` per `(task, lesson, agent)` with per-module latest results attached for aggregation.

Tests cover:

- Invalid/cross-profile lesson names, empty input, duplicate-name rejection
- Request-order preservation through parse (before aggregation exists)
- Time-window boundaries (`created_at == end`, `due_time == start`, open-ended due)
- ACTIVE/ARCHIVED task inclusion; DRAFT/DELETED exclusion
- Never-started assignees (zero runs → counted in `total_assignment_count`, not passed/scored)
- Tenant isolation and unrelated runs/tasks
- Deterministic latest-attempt selection across retries
- Archived lesson revision (newer ARCHIVED revision → lesson not in current metadata)
- Quiz and conversation modules in the same lesson
- Bounded-read / overflow behavior (whichever cap is chosen)

Stop for code review. Commit only after approval.

### Step 2: aggregation, response, and RPC integration

In the same production file:

1. Roll module latest results up to lesson assignments; apply complete/pass/applicable-score rules above.
2. Compute `session_count` as distinct tasks per lesson.
3. Compute `average_applicable_score` only when `applicable_score_count > 0`.
4. Build **one response entry per requested lesson in caller order**, with zero/default metrics when no assignments match. Length must equal `len(training_lesson_names)`.
5. Wire the public `RetrieveTrainingSimulatorLessonStats` method to the step-1 loader and step-2 aggregator.

Tests cover:

- Zero assignments, never-started, incomplete (partial modules), full pass, full fail
- Retries where the newest attempt wins
- Mixed conversation + quiz modules; quiz threshold changes on current module config
- Stale module attempts excluded after lesson edit
- Multiple lessons in one request with correct per-lesson isolation
- **Response order and length** match request (index assertions, not map lookup)
- End-to-end RPC/database case

Stop for code review. Commit only after approval.

## File organization

Keep all new production types, query helpers, preparation logic, aggregation, and RPC code in one `action_retrieve_training_simulator_lesson_stats.go` file initially. Tests remain in the conventional sibling `_test.go` file.

The current contract has no assignee filters. Module criterion breakdown is out of scope (module stats owns that); lesson stats only expose the flat lesson-level fields.

## Out of scope for CONVI-7600

- `training_lesson_names` narrowing on the module API (CONVI-7601 B4 — separate fix on module branch).
- `active_lesson_count` and criterion ordering (module-only fields).
- CSV export, ClickHouse, assignee/group filters, revision-exact historical reporting.
- Shared `ConversationEvaluationFromDB` refactor (CONVI-7632).

## Related artifacts

- Module stats review: `sessions/2026-09-03/claude-convi-7601-module-stats-review.md`
- Lazy drawer contract: `sessions/2026-09-01/codex-lazy-lesson-module-stats.md`
- Work item: `work-items/lesson-module-statistics-reporting.md`
