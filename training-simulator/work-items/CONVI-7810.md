# CONVI-7810: Training Simulator CSV exports

## Current state

[Proto PR #10413](https://github.com/cresta/cresta-proto/pull/10413) now contains one commit `09a3f10c27` extending the existing `ListTrainingSimulatorTaskRuns` request with `time_range = 10`, filtering task-run record `create_time` in `[start_timestamp, end_timestamp)`. The reviewer and user agreed to fetch and join historical definitions in FE, so `include_training_module_revisions` and response `training_module_revisions` were removed. No separate CSV RPC or response change remains. Existing filters, pagination, access annotations, and omitted-option behavior remain.

Frontend CSV construction uses names from all filtered table rows and consumes every attempt page. Leave conversation/agent/quiz-submitter filters unset: Assignee selects sessions, then export their full audience. Preserve every retake. FE deduplicates revision references, fetches exact modules through ListTrainingModules and quiz questions through GetQuizTemplate, then joins and unions criteria/questions, keeping absent definitions blank and distinguishing recorded N/A or pending results. Assigned users without attempts have no data rows. The approved attempt-date filter is record creation, not assignment-window overlap or attempt start; quiz runs are created on submission.

This PR defines the contract only. Backend and Director implementation is prepared in new worktrees and validated locally. The backend dependency bump remains pending the proto release. The replacement changes only the Training Simulator service proto and reuses existing message types/imports; do not modify LLM task enums, build dependencies, or generated files.

## Source

- Primary source for this work item: `/Users/xuanyu.wang/repos/cresta-proto`.
- Worktree: `/Users/xuanyu.wang/repos/cresta-proto-training-simulator-csv`.
- Branch: `xw/convi-7810-training-simulator-export-protos`.
- Ticket: https://linear.app/cresta/issue/CONVI-7810/support-training-sessions-csv-export
- Session: [List extension and history replacement](../sessions/2026-10-27/codex-convi-7810-list-api-extension.md).

## Validation and next steps

Old export proposal head `fd2e479c76` is preserved locally as `xw/convi-7810-export-proposal-backup-20261027`. The branch remains one commit above base `590a10a1a3`; amended commit `d6be44b611` removes enrichment and changes one service proto (13 additions, one deletion). Full proto/API lint, focused Buf build/compatibility, formatting, diff checks, and package Bazel build were rerun successfully after removal. GitHub CI and user review remain pending. Backend must implement date filtering; FE definition fetching/joining and CSV generation remain follow-up work. API definitions alone do not establish runtime behavior.

## Existing export precedent (2026-10-26)

Group calibration uses shared template criterion grade/comment columns across reviewer rows. The closer cross-module precedent is the backend scorecard export, which unions criteria by identifier across templates/revisions and leaves unused row cells blank. Training Simulator should apply that pattern to configurations referenced by exported attempts, preserving historical rule identity and each retake. See [source verification](../sessions/2026-10-26/codex-convi-7810-criterion-column-precedent.md).

## Earlier CSV schema design

The earlier CSV RPC proposal defined fixed headers in order: Agent, Agent ID, Session, Session ID, Lesson, Lesson ID, Module, Module ID, Module Type, Attempt ID, Started At, Completed At, Score (%), Result. Dynamic criterion and quiz question columns followed, ordered by stable identity and unioned over exported historical configurations. Criterion values were Pass/Fail/N/A/Pending; question values Correct/Incorrect/Pending. This remains frontend design context, not a CSV response contract in the List extension. See [earlier proposal](../sessions/2026-10-26/codex-convi-7810-single-export.md).

## Frontend CSV option

The user and reviewer selected separate existing reads plus frontend joins on 2026-10-27 in [discussion r4391901212](https://github.com/cresta/cresta-proto/pull/10413#discussion_r4391901212). Existing task-run results and pagination are reused; exact module and quiz definitions come from ListTrainingModules and GetQuizTemplate. Some identity labels/timings may still require existing reads. See the [earlier evaluation](../sessions/2026-10-26/codex-convi-7810-frontend-csv-option.md).

The [deeper comparison](../sessions/2026-10-26/codex-convi-7810-task-run-reuse-comparison.md) verified both enriched List and orchestration without protobuf changes are feasible. Its assignment-window discussion describes table/session selection; the later approved List time range explicitly filters task-run create_time. Session Assignee filters still select whole sessions, not individual attempt owners.

## Backend and frontend implementation (2026-10-27)

- Backend: `/Users/xuanyu.wang/repos/go-servers-convi-7810-csv`, `xw/convi-7810-task-run-time-range`; time-range validation/filter implemented; full Training Simulator suite passes with local pending proto override. [Session](../sessions/2026-10-27/codex-convi-7810-backend.md).
- Frontend: `/Users/xuanyu.wang/repos/director-convi-7810-csv`, `xw/convi-7810-training-results-csv`; three table download buttons, shared paginated loader, historical-definition joins and CSV builder. 11 export tests and one pagination API test, TypeScript, ESLint and Biome pass. [Session](../sessions/2026-10-27/codex-convi-7810-frontend.md).
- Module/lesson exports resolve referenced sessions after fetching matching attempts; they do not reapply assignment-window/status/usecase filters. Session export uses already-filtered table session names. Audience is checked per run against its own session, with quiz submitter normalization.
- Source changes remain uncommitted; no implementation PRs created. Release proto PR #10413, bump backend dependency, deploy backend before frontend downloads. No authenticated browser validation yet.
