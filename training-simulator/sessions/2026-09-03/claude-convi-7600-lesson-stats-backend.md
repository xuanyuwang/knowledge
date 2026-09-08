# CONVI-7600 lesson stats backend — plan execution

Date: 2026-09-03
Tool: Claude Code
Source repo: `/Users/xuanyu.wang/repos/go-servers-milestone-2` (worktree)
Branch: `claude/convi-7600-lesson-stats` (fresh from `origin/main` @ `fe7caf35cc`)
Plan executed: `sessions/2026-09-02/cursor-convi-7600-two-step-plan.md`
Reference: CONVI-7601 module stats @ `5bca084abf` (`go-servers-milestone-3-module-reporting` worktree, PR #31952, not yet merged to main)

## Worktree preparation (done)

- Old dirty draft preserved as stash `obsolete-lesson-reporting-list-extension-draft` on `codex/milestone-2-lesson-reporting` (included untracked `lesson_stats.go`/test via `stash -u`). Obsolete CONVI-7582 local commit `7cd3f9f14f` left on the old branch, not replayed.
- New branch `claude/convi-7600-lesson-stats` created from current `origin/main` in the same worktree.
- Verified published flat lesson contract in cresta-proto module cache v2.22.11 (`lesson_stats.proto`: `training_lesson_name`, `session_count`, `average_applicable_score`, `applicable_score_count`, `passed_assignment_count`, `total_assignment_count`; request requires nonempty `training_lesson_names` in caller order). go.mod pins v2.22.26.
- Note: local `/Users/xuanyu.wang/repos/cresta-proto` checkout has uncommitted stale draft (`lesson_stats.proto` with nested `modules` + `has_missing_result_status`) — superseded design, do not use as contract source.

## Step 1 implemented (request, reads, aggregation input)

Files (uncommitted, pending review per plan):

- `apiserver/internal/trainingsimulator/action_retrieve_training_simulator_lesson_stats.go`
- `apiserver/internal/trainingsimulator/action_retrieve_training_simulator_lesson_stats_test.go`
- BUILD.bazel updates via Gazelle (in progress)

Pipeline: `parseLessonStatsRequest` → `findLessonStatsLessonMetadata` → `findLessonStatsDirectorTasks` → `expandLessonAssignmentKeys` → `findLatestLessonModuleAttemptResults` → `prepareLessonStatsInputs` emits one `lessonStatsFact` per `(task, lesson, agent)` with per-module latest results.

CONVI-7601 lessons applied from day one:

- Scores used verbatim (stored 0–1); tests seed 0.80/0.90, not 80/90.
- Lesson metadata: `MAX(created_at)` over ALL revisions, join latest, then `state = ACTIVE` (archived lessons excluded; regression-tested).
- Caller order preserved via `orderedLessonIDs`/`orderedLessonNames` slices from parse through aggregation inputs.
- Empty `training_lesson_names` rejected (`InvalidArgument`); duplicates rejected (chose reject over echo per plan).
- Quiz modules included; pass derived `score >= passingScore/100` from current module config.
- Latest attempt: `created_at` asc, `resource_id` asc tie-break, take last.
- Time window: `created_at <= end AND (dueTime IS NULL OR dueTime >= start)`, inclusive; tasks ACTIVE + ARCHIVED, DRAFT/DELETED excluded.
- Read bound: `trainingSimulatorLessonStatsMaxTaskRuns = 100000` with `Limit(max+1)` overflow probe → `ResourceExhausted` (session-stats pattern applied to direct DB reads; `maxTaskRuns` is a parameter so tests exercise overflow without seeding 100k rows).
- All helpers/types use `lessonStats*`-prefixed names to avoid colliding with the module-stats file when both land on main.

## Validation

- `go build`, `go vet`, `gofmt -w -s` clean.
- `TEST_DATABASE_URL=postgres://cresta:$LOCAL_TEST_DB_PW@127.0.0.1:5432 go test -run TestRetrieveTrainingSimulatorLessonStats -count=1` → PASS (all 7 test funcs incl. 9 parse subtests).
- Full package `go test ./apiserver/internal/trainingsimulator/` → ok 9.8s.
- Environment note: local PostgreSQL 15 existed but had no `cresta` role/db; created `cresta` superuser + `cresta` database per go-servers CLAUDE.md local-test convention.

Test coverage vs plan: parse validation (invalid/cross-profile/cross-customer/empty/duplicate/inverted range), request-order preservation, status + inclusive time-window boundaries (created_at == end, due_time == start, open-ended due), never-started assignees, tenant isolation (same IDs under customer2/profile2), latest-attempt determinism (retries + resource_id tie-break), archived latest revision exclusion, quiz + conversation modules in one lesson, overflow probe.

## Code review rounds (two `/code-review` passes before commit)

Round 1 — fixed: (1) `IN ?` ID lists exceed Postgres 65535 bind-param limit → `= ANY(?)` + `pq.StringArray` everywhere (precedent `action_list_director_task.go:272`); (2) runs probe budget consumed by archived-lesson runs → scope runs to surviving ACTIVE lesson IDs; (3) run classification now quiz-first to match canonical `action_list_training_simulator_task_runs.go`; (4) DB errors → `fmt.Errorf(%w)` without double-logging per repo CLAUDE.md; (5) quiz `passingScore > 0` guard documented. Refuted/adjudicated: 0–1 conversation scale (verified: only `UpdateTrainingSimulatorTaskRun` writes the column, proto documents `evaluation_score` 0.0–1.0, Director divides by 100 pre-persist; `EvaluateTrainingConversation`'s 0–100 is response-only), ARCHIVED task inclusion (plan spec), created_at revision-join tie (shipped convention), test seeder duplication (follow-up when module stats merges), DISTINCT ON redesign (contradicts approved pipeline).

Round 2 — fixed: missing `@com_github_lib_pq` Bazel dep (Gazelle re-run, `bazel build` strict-deps verified); runs query additionally scoped to current module membership (index `ix_training_sim_task_runs_task_lesson_module`); TOASTed `criterion_results` no longer dragged (narrow `Select` on both score loaders); fail-open comment on task lesson-name parse; invariant comment for unresolvable score links (agent identity lives on score rows). Refuted again: the 0–100 conversation-scale claim (traced both writers). Accepted as documented: zero-module ACTIVE lessons report zero-default entries rather than sibling's `FailedPrecondition` (batch endpoint — one misconfigured lesson must not fail the whole page); tasks query unbounded (profile task counts are small; probe covers runs).

Step 1 committed: `18e7427531` `[CONVI-7600] Lesson stats: request parsing, reads, and aggregation input`.

## Step 2 implemented (aggregation, response, RPC)

- `RetrieveTrainingSimulatorLessonStats` RPC wired: parse → prepare → aggregate.
- `aggregateLessonStats`: per-assignment complete = every current module has settled (scored) latest attempt; passed = complete ∧ all modules pass; applicable score = mean of per-assignment module-score means over complete assignments; `session_count` = distinct tasks with ≥1 fact; response walks `orderedLessonIDs` with zero/default entries for unknown/archived/unassigned lessons.
- Tests: E2E (two lessons, reversed request order with index assertions, retry-newest-wins, full pass/fail, incomplete, never-started, ARCHIVED task inclusion, session_count across 2 tasks), zero-default entries (unknown/archived/idle), stale module attempts excluded after lesson edit, quiz threshold from current module revision.
- Validation: suite PASS, full package ok 9.8s, Gazelle no-op.

## Step 2 review and completion (2026-09-04)

- The step-2 `/code-review` run stalled twice (stream watchdog killed the finder-agent tree both times — infrastructure flake, no verdict). Given the small, additive diff (RPC + `aggregateLessonStats` + tests over the already twice-reviewed step-1 machinery), proceeded with a manual review: response one-to-one in caller order, zero/default entries, `session_count` = distinct tasks with ≥1 fact, complete = all current modules settled+scored, average guarded on `applicable_score_count > 0`, no new imports (Gazelle no-op).
- Step 2 committed: `111e860b37` `[CONVI-7600] Lesson stats: aggregation, response, and RPC`.
- Branch `claude/convi-7600-lesson-stats` = `origin/main` + 2 commits. NOT pushed — awaiting human review/PR decision.

## Final state

- `work-items/lesson-module-statistics-reporting.md` updated (branch, commits, status, timeline).
- Suggested follow-ups: push branch and open PR; optionally re-run `/code-review` on the full branch diff when the agent infra is healthy; unify the three stats suites' seed helpers when CONVI-7601 merges; CONVI-7601 B4 (`training_lesson_names` narrowing) remains a separate module-branch fix.
