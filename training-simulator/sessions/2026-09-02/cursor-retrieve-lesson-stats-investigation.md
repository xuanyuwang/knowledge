# Investigation: RetrieveTrainingSimulatorLessonStats in go-servers

Date: 2026-09-02
Tool: Cursor (read-only)
Checkout: `/Users/xuanyu.wang/repos/go-servers` (same inode as `go-servers`)
Branch: `main`, **behind origin/main by 128 commits**

## Scope

Trace Training Simulator service structure and `RetrieveTrainingSimulatorTaskStats` as the template for implementing `RetrieveTrainingSimulatorLessonStats`. Locate cresta-proto PR #9677 release without fetch/mutate.

## Checkout / dependency state

| Surface | Value |
|---|---|
| Local cresta-proto pin | `github.com/cresta/cresta-proto/v2 v2.21.12` (`go.mod` + `deps.bzl`) |
| origin/main cresta-proto pin (gh raw, no fetch) | `v2.22.11` |
| LessonStats impl on local or origin/main package listing | **Absent** — only TaskStats action file |

## cresta-proto PR #9677

- Title: `[CONVI-7600] Add lesson reporting stats contract`
- State: **MERGED** 2026-09-02T00:07:45Z
- Merge commit: `2364c69ff5d33030945ed03c275e5e034166e614`
- URL: https://github.com/cresta/cresta-proto/pull/9677
- PR body text is **stale** (still describes list `include_stats`); merged sources match the dedicated-RPC decision in `decisions/2026-08-28-dedicated-lesson-module-stats-apis.md`.

### First release containing the merge

GitHub compare (merge OID not present in local cresta-proto objects):

- `v2.22.6` vs merge → **behind**
- `v2.22.7` vs merge → **ahead** by 2 commits

**Earliest visible release/tag with PR #9677: `v2.22.7`**. Latest observed release: `v2.22.11`.

### Merged contract (authoritative)

RPC: `RetrieveTrainingSimulatorLessonStats`

- Request: required `parent`, required ordered `training_lesson_names`, optional `time_range`
- Response: `lesson_stats` repeated `TrainingSimulatorLessonStats` (same order; no assignments → zeros)

`TrainingSimulatorLessonStats`: `training_lesson_name`, `session_count`, `average_applicable_score`, `applicable_score_count`, `passed_assignment_count`, `total_assignment_count`

Local cresta-proto checkout is dirty with uncommitted List `include_stats` WIP — **ignore**; use released `v2.22.7+`.

## go-servers Training Simulator structure

Package: `apiserver/internal/trainingsimulator`

- DI: `module.go`
- Impl: `trainingsimulatorserviceimpl.go` embeds `UnimplementedTrainingSimulatorServiceServer`
- Envelope registration under `service-envelope/internal/services/`
- Pattern: one `action_*.go` per RPC (+ `_test.go`); Gazelle-managed `BUILD.bazel` with explicit `srcs`

### RetrieveTrainingSimulatorTaskStats (template)

File: `action_retrieve_training_simulator_stats.go`

Verified coaching symbols: `ListDirectorTasks`, `DirectorTask`, ACTIVE + `DIRECTOR_TASK_TYPE_TRAINING_SIMULATOR`.

Flow:

1. Parse profile parent
2. Expand user/group audience filters
3. List active Training Simulator director tasks (time + audience)
4. Filter by lesson names from task content config
5. Fetch lessons (with modules) + users
6. Fetch task runs via internal ListTrainingSimulatorTaskRuns
7. Aggregate per task: latest attempt per (task, lesson, module, agent); completion = all required modules scored; exclude non-required modules from score/pass

### DB models / DAOs

`apiserver/sql-schema/gen/model|dao`:

- `TrainingSimulatorTaskRuns`, `TrainingSimulatorConversationScores`
- `TrainingLessons`, `TrainingModules`, `TrainingScenarios`
- Quiz score tables

No lesson-stats table; computed reporting.

### Tests / BUILD

- Suite embeds shared DB test suite; seeds modules/lessons/task runs; mocks `ListDirectorTasks`
- Cases cover basic, failed module, multi-attempt, filters, never-started, stale/extra modules
- After new files or proto bump: `bazel run //:gazelle` or `mage cleanBuild` (also refreshes `deps.bzl`)

## Recommended commit split

### Step 1 — Proto bump / plumbing

Sync toward origin/main (already `v2.22.11`) or bump to `>= v2.22.7` (prefer `v2.22.11`), run cleanBuild/gazelle, prove compile with Unimplemented LessonStats. Optional ordered-zero stub only if desired for a callable RPC without business logic.

### Step 2 — Implementation + focused tests

New `action_retrieve_training_simulator_lesson_stats.go` (+ test). Validate parent/names; preserve response order; apply time_range with assignment-window overlap semantics; fill the six lesson_stats fields using TaskStats loading primitives where possible — do not clone TaskStats agent-performance shape. Tests: order/zeros, time filter, multi-lesson conversation+quiz, incomplete vs passed, zero applicable scores. Include BUILD srcs update.

## Risks

- Evaluation-persistence proto PR #9656 closed; define applicable-score using existing conversation/quiz score data (same as TaskStats).
- Knowledge also documents a go-servers/Director product line; **this codepath is go-servers + cresta-proto TrainingSimulatorService.**
