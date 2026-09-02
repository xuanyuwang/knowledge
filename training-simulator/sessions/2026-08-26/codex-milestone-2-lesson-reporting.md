# Milestone 2 Lesson Reporting

Date: 2026-08-26
Source repo: `/Users/xuanyu.wang/repos/go-servers`
Branch/worktree context: `codex/milestone-2-lesson-reporting` in `/Users/xuanyu.wang/repos/go-servers-milestone-2`, with matching proto and Director worktrees listed below
Scope: lesson-level Training Simulator reporting only

> **Superseded transport (2026-08-28):** The assignment/result aggregation findings remain applicable, but the `ListTrainingLessons` extension is no longer the selected public contract. Replace it with `RetrieveTrainingSimulatorLessonStats` per [the dedicated API decision](../../decisions/2026-08-28-dedicated-lesson-module-stats-apis.md).

## Worktrees

- `/Users/xuanyu.wang/repos/cresta-proto-milestone-2`, based on `convi-7582-persist-evaluation-result-contract`
- `/Users/xuanyu.wang/repos/go-servers-milestone-2`, based on `convi-7582-persist-evaluation-result`
- `/Users/xuanyu.wang/repos/director-milestone-2`, based on Director `main`

The original dirty `cresta-proto` checkout was inspected but not modified. Its `lesson_stats.proto` content was preserved in the dedicated worktree. No generated protobuf, GORM, or web-client source was retained.

## Implemented scope

- Extended `ListTrainingLessons` with Option 2 direct request fields and an aligned `lesson_stats` response.
- Added assignment-rooted Sessions assigned, average score, lesson pass rate, and ordered per-module pass rates.
- Added one bounded backend load pipeline: current definitions, assignment/run/result loading, latest-attempt-first normalization, then pure lesson aggregation.
- Reused current lesson/module definitions and excluded quiz modules from score/pass denominators while retaining an unavailable chip for them.
- Added a separate reporting authorization check for QA admin/global admin roles.
- Added one batched Director stats request for the loaded lesson set, lesson table values, and a lesson reporting drawer. Authoring links remain intact and a stats failure leaves content usable with `--` values.
- Did not add module-level reporting, criterion reporting, tables, indexes, summary storage, or generated code.

## Findings and decisions

### Observed behavior

- `ListTrainingLessons` currently loads the latest lesson revisions in one DB statement and ignores its pagination fields; the Director lesson page requests up to 200 lessons and renders client-side pagination.
- Training assignments are shared DirectorTasks. The DB audience config stores user IDs, so one task expands to one lesson fact per assigned agent while `session_count` remains the number of distinct matching task IDs.
- Training task date filtering in `ListDirectorTasks` is inclusive interval overlap: `created_at <= end` and `due_time IS NULL OR due_time >= start`. The lesson loader matches this behavior.
- Task runs do not store agent ID. Conversation-score rows provide the agent identity, evaluation status, overall N/A, score, and pass result.
- The Lesson Configuration route is available to QA admin and global-admin users; ordinary `ListTrainingLessons` content access is broader.
- The current worktree is intentionally missing generated CONVI-7582 GORM/proto artifacts, and the Director dependency tree is not usable for focused Vitest execution.

### Confirmed decision

- Use Option 2 with direct `include_stats` and `stats_*` request fields; do not add `TrainingSimulatorStatsFilter`.
- Preserve the content-only query path when `include_stats=false`.
- Select the latest attempt by `(created_at, resource_id)` before classifying its result. A newer pending retry makes the module fact incomplete; it never falls back to an older complete result.
- Calculate lesson score as the average of each completed lesson fact's mean applicable module score. Calculate lesson pass rate across completed applicable agent-lesson facts, requiring every applicable module to pass.
- Exclude incomplete, all-N/A, and ambiguous facts from lesson score/pass denominators. Preserve Sessions assigned when score/pass values are unavailable.
- Keep lesson-level quiz aggregation out of Milestone 2. Quiz modules remain visible in the drawer with `--`.

### Proposed behavior

- Include ACTIVE and ARCHIVED training assignments so archival does not erase reporting history.
- Apply current DirectorTask active-window overlap to the Date Range filter.
- On any ambiguous matching conversation result, preserve session count but mask lesson and module score/pass values as `--` rather than presenting potentially incomplete rates.
- Bound the initial direct reads at 10,000 tasks, 100,000 assignment facts, and 10,000 task runs, returning `RESOURCE_EXHAUSTED` instead of truncating. These bounds require representative-volume measurement before release.

### Unresolved assumption

- Product has not confirmed ACTIVE+ARCHIVED assignment inclusion or active-window date semantics.
- Revision-exact reporting remains intentionally unresolved; current definitions can change historical membership and denominators.
- `has_missing_result_status` is also used for malformed complete rows (missing N/A/score/pass or an out-of-range score), not only legacy rows lacking evaluation status. The contract name is narrower than the current safety behavior.
- The 10,000-run bound may need chunking or another bounded strategy after representative query measurement.
- End-to-end generated field names and Director structural typing still require verification after normal code generation.

## Worked example and session reconciliation

One lesson has conversational modules M1 and M2 plus quiz module Q. The fixture expands four DirectorTasks to five agent-lesson facts:

| Session / agent | Latest module results | Lesson result |
|---|---|---|
| Task 1 / Agent A | M1 `0.90 pass`; M2 `0.70 fail` | score `0.80`, failed |
| Task 1 / Agent B | M1 `0.80 pass`; M2 complete N/A | score `0.80`, passed |
| Task 2 / Agent C | no runs | incomplete |
| Task 3 / Agent D | M1 old `0.95 pass`, then newer pending retry; M2 `0.85 pass` | incomplete; old M1 is not reused |
| Task 4 / Agent E | M1 N/A; M2 N/A | all-N/A, excluded |

Reconciliation:

- Sessions assigned = four distinct tasks, not five assigned agents.
- Average score = `(0.80 + 0.80) / 2 = 0.80`.
- Pass rate = one passed / two passed-or-failed lesson facts = `0.50`.
- M1 pass rate = Agent A + Agent B passed / two applicable completed M1 facts = `1.00`.
- M2 pass rate = Agent A failed and Agent D passed / two applicable completed M2 facts = `0.50`; module facts are independent of whole-lesson completeness.
- Quiz Q has no lesson-level reporting denominator and returns no pass rate (`--`).

This reconciliation is encoded in `lesson_stats_test.go`. No customer or staging data was accessed.

## Query and page-loading evidence

- The backend query-count test installs a GORM callback and expects one SQL statement for `include_stats=false` and three for `include_stats=true` with zero assignments: lessons, current module definitions, then tasks. With runs/results present, the code adds at most one task-run statement and one conversation-score statement; it never queries per lesson.
- Runtime execution of that measurement is blocked because the worktree intentionally lacks CONVI-7582 generated GORM/proto fields. Therefore these counts are instrumented expectations, not a completed measurement.
- Director code inspection shows one content request followed by one stats request containing all loaded lesson names. There are no per-row requests. The second request is deliberately separate so reporting failure does not fail authoring content.
- Live page-load timing/request measurement is blocked by the worktree's unusable web-client dependency tree and absent `vitest`; no latency target is asserted.

## Validation

- Passed: `buf lint` after removing Bazel convenience symlinks.
- Passed: `buf build -o /private/tmp/milestone-2-lesson-reporting.binpb`.
- Passed: `bazel build //cresta/v1/trainingsimulator:all`.
- Passed: `git diff --check` in all three worktrees.
- Passed: Go formatting for all changed backend Go files.
- Blocked: focused Go tests fail during package compilation because CONVI-7582 generated GORM and proto fields (`EvaluationStatus`, `EvaluationNotApplicable`) are absent. They were not generated because generated code is explicitly out of scope.
- Blocked: focused Director test cannot start because the available dependency tree has no `vitest` binary; a temporary `node_modules` symlink was removed after the check.
- Blocked: proto clang-format check because `clang-format` is not installed; Buf and Bazel validation passed.

## Next steps / release gates

1. Run normal CI generation for CONVI-7582 plus the lesson stats contract, then execute the focused backend and Director tests.
2. Run the instrumented SQL-count test with representative assignments/runs and capture query timings/plan before changing indexes or read bounds.
3. Measure the Director lesson page's two-request loading behavior with the generated client available.
4. Reconcile at least one representative environment cohort against underlying DirectorTasks, assignments, task runs, and conversation-score rows.
5. Obtain Product confirmation for ACTIVE+ARCHIVED and time-window semantics.

Credentials used for PR publication: the GitHub SSH identity `~/.ssh/id_ed25519` for push and the existing GitHub CLI authenticated context for PR operations. The SSH configuration and public-key comment were inspected first; the separately marked restricted credential was not read or used.

## Proto pull request

- Latest commit: `99dfc72d37` (`[CONVI-7600] Reuse module stats in lesson reporting`)
- PR: [cresta-proto#9677](https://github.com/cresta/cresta-proto/pull/9677)
- Linear: [CONVI-7600](https://linear.app/cresta/issue/CONVI-7600/add-training-simulator-lesson-reporting-api-contract)
- Base: `codex/milestone-3-module-reporting` from [module reporting PR #9676](https://github.com/cresta/cresta-proto/pull/9676), because lesson results reuse its `TrainingSimulatorModuleStats` message. Relative to that base, the PR contains only the three lesson-reporting contract files.
- The description identifies #9676 as a merge dependency and links [evaluation result persistence PR #9656](https://github.com/cresta/cresta-proto/pull/9656) as related work, without internal milestone terminology.

## 2026-08-27 review follow-up

- **Observed behavior:** `TrainingLesson.training_modules` is explicitly ordered, while `TrainingSimulatorLessonStats.modules` initially did not state an ordering contract.
- **Confirmed decision:** `TrainingSimulatorLessonStats.modules` follows the exact order of `TrainingLesson.training_modules`; the protobuf comment now makes that alignment explicit.
- **Observed behavior:** Running `bazel run //:gazelle` produced no `BUILD.bazel` diff, confirming the checked-in `lesson_stats.proto` source entry is Gazelle-generated rather than a divergent manual edit.
- Validation passed: `buf lint`, `buf build`, `bazel build //cresta/v1/trainingsimulator:all`, and `git diff --check`.

## 2026-08-28 API-lint follow-up

- **Observed behavior:** the full CI API linter rejected `lesson_stats` as a non-standard AIP-132 list-response field and rejected adding `OPTIONAL` behavior only to the new request fields while the existing request fields remain unannotated.
- **Confirmed decision:** preserve the Option 2 contract and parallel ordering guarantee. Add a narrow AIP-132 suppression explaining why stats remain outside the existing `TrainingLesson` resource.
- **Confirmed decision:** remove the three new `OPTIONAL` annotations to match the established request style; this does not change field presence or request semantics.
- Pushed commit `2733d8005b`. Scoped `buf lint`, `buf build`, `bazel build //cresta/v1/trainingsimulator:all`, and `git diff --check` passed locally; authoritative CI reran after the push.

## 2026-08-28 shared module-stats follow-up

- **Confirmed decision:** reuse `TrainingSimulatorModuleStats` for the ordered module entries inside `TrainingSimulatorLessonStats`; remove the lesson-only `TrainingSimulatorModulePassRate` message.
- **Confirmed decision:** clients derive each lesson module chip from `passed_count / total_count` when the denominator is nonzero. This keeps the lesson projection and later module drill-down on one contract.
- **Observed behavior:** the shared message exists only in #9676, so #9677 now targets the #9676 branch. Its three-dot diff contains only `lesson_stats.proto`, the service additions, and the BUILD source entry.
- Pushed `99dfc72d37`, updated the PR description, replied to review comment `r3875814042`, and resolved the thread. Scoped Buf/Bazel/whitespace validation passed.
