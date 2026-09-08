# CONVI-7601 module stats backend review

Date: 2026-09-03
Reviewer: Claude Code
Source repo: `/Users/xuanyu.wang/repos/go-servers-milestone-3-module-reporting`
Branch: `codex/milestone-3-module-reporting` @ `1d1e5143d4` (`[CONVI-7601] Implement RetrieveTrainingSimulatorModuleStats`, squashed, unpushed; diverged from remote)
Base: `origin/main` with cresta-proto `v2.22.11` (module contract #9676 merged @ v2.22.6; lesson contract #9677 merged as `2364c69ff5`)
Scope: correctness, SQL/performance, edge cases, consistency with CONVI-7583 session reporting, API contract vs published proto, test gaps

Verdict: **Request changes** — two correctness bugs and three violations of the published proto contract. Structure, query shape, identity joins, and time-window semantics are otherwise sound.

## Validation performed

- Read the full implementation (`action_retrieve_training_simulator_module_stats.go`, 710 lines), tests (567 lines), BUILD.bazel diff.
- Compared against `action_retrieve_training_simulator_stats.go` (shipped CONVI-7583) and `action_list_director_task.go` time-range predicates.
- Verified the published contract in cresta-proto v2.22.11 (`training_simulator_service.proto`, `module_stats.proto`) including generated PGV (`training_simulator_service.pb.validate.go`: only `parent` + `time_range` validated; no rules on `training_module_names`).
- Traced identity/scale end to end: internal DB proto (`apiserver/sql-schema/protos/qa/task.proto` audience `user_ids` = bare IDs), converter (`APIToDBTrainingSimulatorAudienceConfig`), run creation (`action_create_training_simulator_task_run.go`), score writes (`action_update_training_simulator_task_run.go`), Director persistence (`useModuleEvaluationPipeline.ts`, `SCORE_PERSIST_SCALE = 100`, introduced with the pipeline in CONVI-7104 `0a32269c60` — no legacy 0–100 era), Director display (`agentScore.ts`: "BE ships per-agent scores as a 0–1 float despite the 0–100 proto comment; the UI works in 0–100").
- Confirmed lesson updates create new revision rows (`action_update_training_lesson.go:89-90`), so archiving leaves older ACTIVE revision rows behind.
- Ran the suite: `go test -run TestRetrieveTrainingSimulatorModuleStats -count=1 ./apiserver/internal/trainingsimulator/` → ok, 9s. `gofmt -l` and `go vet` clean.

## Blockers

### B1 — Conversation score divided by 100, but the DB stores 0–1

`action_retrieve_training_simulator_module_stats.go:584`:

```go
result.normalizedScore = convScore.Score.Float64 / 100.0
```

`training_simulator_conversation_scores.score` is stored **0–1**:

- Director divides the evaluator's 0–100 score by `SCORE_PERSIST_SCALE = 100` before calling `UpdateTrainingSimulatorTaskRun` (`useModuleEvaluationPipeline.ts:24,133`); that RPC writes `EvaluationScore` verbatim into the column (`action_update_training_simulator_task_run.go:90`).
- Task-run proto documents `evaluation_score` as "(0.0 to 1.0)".
- Director's `agentScore.ts` confirms the round trip is 0–1 and scales ×100 only for display.
- Quiz scores are 0–1 too (`computeQuizScore` returns `passedWeight/totalWeight`), and the module-stats code correctly uses them raw — the two stores share one normalized scale (work-item 2026-08-13 decision).

Impact: `average_applicable_score` is 100× too small (a module averaging 85% reports 0.0085). The response contract (`module_stats.proto`: "in the range 0.0 to 1.0") then disagrees with what clients display.

Fix: use the stored value verbatim (`result.normalizedScore = convScore.Score.Float64`). The tests bake in the same wrong assumption (seed 80/90, expect 0.8/0.9) and must be re-seeded with realistic 0–1 values.

Root cause worth recording: the shipped session-stats test comment (`action_retrieve_training_simulator_stats_test.go:97-99`, "conversation score stored as a 0–100 percentage") is wrong about production storage; its fixture (85) is unrealistic. This implementation appears to have inherited that assumption. Session-stats production behavior is unaffected (it passes raw values through), but the fixture/comment should be corrected so the next reader doesn't inherit the bug (N5).

### B2 — Response order is nondeterministic; contract requires request order

`aggregateModuleStats` iterates `inputs.moduleIDs.All()` (Go set/map iteration) at line 632. The published request says module names are "in the order the response must preserve"; the response says "aligned one-to-one with the request's training_module_names: this list has the same length and order … An omitted entry is a contract violation." The sibling `RetrieveTrainingSimulatorLessonStats` contract ("in the same order") confirms the ordered-batch pattern, and the 2026-09-01 lazy-drawer session note records "Requested module names preserve request/response ordering" as explicit intent.

The end-to-end test works around this by indexing results into a map by name — i.e., the test suite already knows order is unreliable.

Fix: keep an ordered slice of requested module IDs from `parseModuleStatsRequest` and build the response in that order. Add an order+length assertion to the E2E test.

### B3 — `criteria` are unsorted; contract specifies exact ordering

`module_stats.proto` field 6: criteria are "ordered by failed-result count (applicable_result_count - passed_result_count) descending, then display_name and criterion_id ascending." The implementation appends from map iteration (lines 695-704).

Fix: sort by `(applicable-passed) desc, display_name asc, criterion_id asc` before returning; add a test that pins the order (e.g., two criteria with equal failed counts but different names).

### B4 — `training_lesson_names` ignored even when non-empty

Published contract (field 4, v2.22.11): "Optional training lesson resource names that narrow the scope of module statistics to assignments for those lessons. When empty, this field is ignored." The implementation never reads the field (`parsedModuleStatsRequest` doesn't even carry it), and the tests assert the ignoring behavior ("accepts valid request without reading lesson names"; `TestPrepareModuleStatsInputs_FiltersAndFacts` passes `other-lesson` and expects unscoped facts).

This field exists precisely for the lazy lesson-drawer call (2026-09-01 decision): Director will send the selected lesson + its ordered modules. Silently returning profile-wide aggregates for a lesson-scoped request makes the drawer numbers wrong without any error.

Fix: when non-empty, parse+validate lesson names against the parent (reject invalid/cross-profile names) and intersect `lessonsByID` in `findModuleAssignmentKeys` with the requested lessons. `active_lesson_count` scoping stays deferred per the work item, but assignment aggregation must narrow.

### B5 — `active_lesson_count` counts archived lessons

`findActiveLessonCounts` (lines 264-268) filters `state = ACTIVE` **inside** the MAX(created_at) subquery. Lesson updates create new revision rows with old revisions retained (`action_update_training_lesson.go:89-90`); archiving writes a new `STATE_ARCHIVED` revision. With the filter before MAX, an archived lesson's older ACTIVE revision still wins `MAX(created_at)` among ACTIVE rows, so the lesson is counted as active.

The established pattern (`action_list_training_lessons.go:43-58`) computes MAX over all revisions and filters state after the join.

Fix:

```go
subq := db.Model(&dbmodel.TrainingLessons{}).
    Select("resource_id, MAX(created_at) AS max_created_at").
    Where("customer = ? AND profile = ?", req.customerID, req.profileID).
    Group("resource_id")
// join latest, then Where("training_lessons.state = ?", STATE_ACTIVE)
```

Add a test: seed lesson rev1 ACTIVE, rev2 ARCHIVED (later created_at) → not counted.

## Contract decision needed

### C1 — Empty module list → all modules, and silent dedupe, contradict the published contract

Design choice #1 in the review prompt (empty `training_module_names` reports all profile modules) conflicts with the published contract: the field is `field_behavior REQUIRED` and documented "Contains between 1 and 200 unique, nonempty names", and the response is "aligned one-to-one with the request's training_module_names: this list has the same length and order". PGV does not enforce any of this (verified in the generated validate code — only `parent`/`time_range`), so the permissive path is reachable, but:

- Empty→all cannot satisfy the "same length" response contract.
- Silent dedupe of duplicate names returns fewer entries than requested, also breaking one-to-one alignment.
- The Director design always sends explicit ordered module names (module page lists them; drawer sends the lesson's modules), so the permissive behavior has no known consumer.

Options: (a) conform to the published contract — reject empty lists, reject duplicates (or echo them), enforce the 200 cap; or (b) amend the proto (docs-only change; field numbers untouched) and re-review. Recommendation: (a), matching the sibling lesson-stats API, unless Product explicitly wants a server-side "all modules" enumeration. Decide before Director integration; either way the behavior should be tested deliberately instead of incidentally.

## Suggestions

- **S1 — Unbounded reads vs session-stats cap.** Session stats cap task runs at 100k with an overflow probe and `ResourceExhausted` (`trainingSimulatorStatsMaxTaskRuns`, `action_retrieve_training_simulator_stats.go:293-307`). Module stats removed all caps (design note #4) and load every run for matched tasks×modules into memory. Restore an overflow probe or document measured bounds before release; "representative query/page-load measurement" is already a release gate in the work item.
- **S2 — Runs without score links are invisible to latest-attempt selection.** `findLatestModuleAttemptResults` (lines 471-498) only groups runs whose `conversation_score_id`/`quiz_score_id` resolves. Current writers always create the score row atomically with the run, so this is latent; but if such a row ever exists (legacy/migration edge), the code silently falls back to an older scored run — violating latest-attempt-first. Document the invariant (each run has exactly one resolvable score link) or treat link-less runs as incomplete latest attempts where the agent can still be determined.
- **S3 — Duplicated time-window predicate.** `moduleStatsDueTimeSQLExpr` + the two Where clauses replicate coaching's `applyTimeRangeIntervalOverlap` (`action_list_director_task.go:285-315`) exactly (verified, including inclusive bounds). Two copies can drift; consider exporting the shared predicate.
- **S4 — Inconsistent DB error logging.** Module/run queries log `Errorf` before returning `codes.Internal`; lesson/task/score queries don't. Pick one convention.
- **S5 — Discovery mutates the parsed request.** `findModuleStatsModuleMetadata` writes into `req.moduleIDs`/`req.moduleNameByID` as a side effect; a `find*` function mutating its input is surprising. Return discovered IDs/names explicitly.

## Nits

- **N1** `TestParseModuleStatsRequest_Validation` doesn't cover the empty-string-in-list or malformed-module-name rejections that the code implements.
- **N2** Time-range boundary equality untested (task `created_at == end_timestamp`, `dueTime == start_timestamp` — both inclusive per the coaching predicate).
- **N3** `seedQuizTaskRun` hardcodes `customer1`/`profile1` while sibling helpers take customer/profile parameters.
- **N4** Quiz `passingScore > 0` guard matches `fetchModulePassingScores` (0 = unset) and the creation-time requirement, so it is consistent today; note in a comment that if a quiz module's passing score is later zeroed/removed, historical quiz attempts report as never-passed here while run-level `QuizPassed` becomes unset.
- **N5** Correct the wrong session-stats test comment/fixture scale (see B1 root cause).

## What checked out

- **Identity joins are correct end to end.** Audience `user_ids` in the internal DB proto are bare user IDs (converted from API `user_names` at task creation); conversation `agent_user_id` is the bare user ID parsed from `agent_name` at run creation; quiz `submitter_user_id` comes from the auth context. All three align, so assignment keys and run keys match on the same identity space.
- **Latest-attempt selection** (sort by created_at, tie-break resource_id ascending, take last) mirrors session stats.
- **Time-window predicate** is exactly the shipped `ListDirectorTasks` interval-overlap semantics (`created_at <= end AND (dueTime IS NULL OR dueTime >= start)`), matching the proto comment added in `dd6f7086ab`.
- **Query shape**: 7 batched queries (modules, active lessons, containing lessons, tasks, runs, conversation scores, quiz scores); no N+1; the runs query uses the `(customer, profile, director_task_id, …)` composite index prefix. Lesson discovery via `training_module_ids && ?` is fine for content-table sizes.
- **Quiz scoring**: raw 0–1 score; threshold compared as `score >= passingScore/100` — same float-safe direction as `ConvertQuizScoreDBToTaskRunAPI`; passing score sourced from module `EvaluationConfig` like `fetchModulePassingScores`.
- **Auth roles** (ADMIN/SUPER_ADMIN/QA_ADMIN, no AGENT) match the 2026-08-27 work-item decision for content-level reporting.
- **Missing modules → zeroed identity entries**, matching "An omitted entry is a contract violation" for explicitly requested modules.
- Conversation `passed` uses the stored column (never re-derived), consistent with the Director pipeline ("never derived from score vs. threshold"); unscored latest attempts count toward totals but not pass/score, matching collapsed zero/false semantics.

## Missing test cases

1. Response preserves request order and length (after B2).
2. Criteria ordering per the proto rule, including tie-breaks (after B3).
3. `training_lesson_names` narrowing; invalid/cross-profile lesson names rejected (after B4).
4. Archived lesson (newer ARCHIVED revision over older ACTIVE revision) excluded from `active_lesson_count` (B5).
5. Realistic 0–1 conversation scores asserting the raw average (B1).
6. Empty module list and duplicate names — whichever behavior C1 selects.
7. Time-range boundary equality (inclusive start/end).
8. One module in two lessons of the same task → two facts per agent (grain check).
9. Run by an agent removed from the task audience → excluded; agent added later with no runs → never-started fact.
10. Quiz module passing score edited to 0/removed after attempts exist.

## Recommendation

**Request changes.** B1 and B5 are correctness bugs producing wrong numbers; B2/B3/B4 break a published contract that Director's lesson drawer and module page will consume. Suggested order: fix B1+B5 with regression tests, implement B2/B3 ordering, implement B4 narrowing, resolve C1 with Product/proto, then restore a runs overflow probe (S1).
