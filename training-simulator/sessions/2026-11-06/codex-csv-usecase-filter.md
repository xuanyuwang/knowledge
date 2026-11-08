# Training Simulator CSV use-case filtering verification

- Date: 2026-11-06 (America/Toronto)
- Primary source repo: `/Users/xuanyu.wang/repos/director`
- Branch/worktree: `main`, main checkout; local HEAD `e520b8985e`, freshly fetched `origin/main` `7d8ce5f6ba`.
- Related backend source: go-servers `origin/main` `8eb33888ef`; local backend checkout is older, so current backend conclusions use the fetched ref.
- Request: verify whether Training Simulator CSV export applies use-case filters. Read-only source investigation; no product changes or customer data queries.

## Conclusion

Export inherits the current use case indirectly through the selected table resources, but does not apply an explicit use-case filter to task runs. Therefore it is not a strict filter on each exported attempt's `usecaseId`.

## Evidence

1. Sessions: `src/hooks/training-simulator/useTrainingSessions.ts:78-86` sends the current use-case resource name in `usecaseNames` to ListDirectorTasks. `src/features/training-simulator/tabs/training-sessions/TrainingSessions.tsx:174-176` passes filtered session tasks to export.
2. Lessons/modules: `src/hooks/training-simulator/useListTrainingLessons.ts:20-24` and `useListTrainingModules.ts:20-24` default `usecaseIds` to the currently selected use-case ID. LessonsTab:531-535 and ModulesTab:405-409 pass filtered resource names to export. Backend content lists apply `training_lessons.usecase_id` / `training_modules.usecase_id` constraints.
3. `src/features/training-simulator/export/loadTrainingResults.ts:64-75` sends the selected task/lesson/module names, date range, and page size to ListTrainingSimulatorTaskRuns, without `usecaseIds`. Its selection type and function arguments carry no use case. Session exports additionally filter to task audience members, while lesson/module exports retain all returned matching runs.
4. `packages/director-api/src/services/cresta-api/training-simulator/trainingSimulatorApi.ts:146-154` only adds the profile parent; the base service builds `customers/{customer}/profiles/{profile}`, without implicit use-case filtering.
5. Current backend `apiserver/internal/trainingsimulator/action_list_training_simulator_task_runs.go:142-144` applies `usecase_id IN (...)` only when `req.GetUsecaseIds()` is nonempty. The export omits that field. Current backend includes `TestListByUsecaseIDs` demonstrating distinct run use cases under one task; it was inspected, not executed.
6. CSV row assembly maps returned runs without an additional use-case check. Thus a different-use-case or unset-use-case run matching a selected resource can be exported. Actual occurrence in customer data was not checked.

All frontend files traced above are unchanged between local HEAD and freshly fetched origin/main. Backend use-case support is newer than the local checkout, which is why inspecting current origin/main matters.

## Validation and limits

Ran `yarn test src/features/training-simulator/export/loadTrainingResults.test.ts src/features/training-simulator/export/trainingResultsCsv.test.ts src/hooks/training-simulator/useTrainingSessions.test.ts` in director/packages/director-app. CSV formatting suite passed (12 tests). Loader and session suites failed before collecting tests because installed `@cresta/web-client/v1/userpreference` cannot resolve. Existing loader assertions explicitly expect resource names + timeRange + pageSize, with no usecaseIds. No dependencies were changed. No live browser/export/deployment verification was performed.

Git fetched using its existing authentication; no credential/profile was explicitly selected or inspected.

## Follow-up

If intended behavior is strict attempt-level filtering, pass the current use-case ID from the export button to the loader and include `usecaseIds` in task-run requests; explicitly decide whether attempts with unset use cases should be retained. This is a recommendation, not an implemented change.

## Follow-up: identify the recently added protobuf field

Fetched cresta-proto main (`305ff083a4`) and inspected history. Commit `729a3f9e2d55ea76abf739467db8dafe50834440`, authored/landed 2026-10-29 per main history, is `[CONVI-7869] Add usecase_id to preview runs and usecase filters to run lists` (cresta-proto PR #10517). It adds `repeated string usecase_ids = 11 [(google.api.field_behavior) = OPTIONAL];` to `ListTrainingSimulatorTaskRunsRequest` at current service proto lines 992-994. Documentation states empty returns task runs for all use cases. Generated TypeScript field: `usecaseIds`. Same PR adds `ListTrainingModulePreviewRunsRequest.usecase_ids = 7` and immutable `TrainingModulePreviewRun.usecase_id = 11`.

Backend main commit `4a8d324d9548c479d57e16d5834fddb023aad5fa`, 2026-10-30, go-servers PR #34850 implements run filtering and fills missing use case from the DirectorTask when creating task runs. The Director Admin tab already sends `usecaseIds: [usecaseId]` in `trainingRunQueries.ts:32,51`, landed in PR #24469 on 2026-11-02. CSV loader still omits it. These are source-history dates; GitHub merge timestamps were not queried separately.
