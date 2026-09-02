# Lesson/module statistics API boundary

Date: 2026-08-18
Status: Superseded 2026-08-28 by dedicated statistics APIs

> The review closed on 2026-08-28 with Option 3 selected. See [the accepted decision](2026-08-28-dedicated-lesson-module-stats-apis.md). The history below is retained because it explains how the alternatives were evaluated.

## Previous direction (2026-08-18)

Add two batch-oriented read RPCs:

- `RetrieveTrainingSimulatorLessonStats`
- `RetrieveTrainingSimulatorModuleStats`

Keep externally visible APIs aligned to one result level. Share `TrainingSimulatorStatsFilter`, training-result messages, user-selection logic, storage loaders, and aggregation implementation between the handlers.

Within `TrainingSimulatorStatsFilter`, keep `virtual_group_names` and `team_group_names` separate. Director already knows each selected group's type, so preserving it avoids an extra backend group-classification call before membership resolution.

## Reopened review (2026-08-20)

The earlier local selection is no longer final. Reviewers prefer reuse, with option 2 currently ahead of option 1. Option 3 still provides the cleanest boundaries. Because all three APIs are expected to remain scoped mainly to the Training Simulator page, their risk of accumulating the broad complexity seen in Analytics APIs is lower.

The three active options are:

1. Extend `ListTrainingSimulatorTaskRuns` with optional lesson/module statistics.
2. Add optional statistics to `ListTrainingLessons` and `ListTrainingModules`.
3. Add dedicated lesson and module statistics APIs.

No option changes the data source: the backend performs direct, filtered database aggregation rather than calculating statistics from any list response.

The canonical backend [API design](../deliverables/lesson-module-statistics-eng-design.md#api-design) records the high-level implementation, protobuf example, pros, and cons for all three options.

## Criteria to close the review

- Keep aggregation semantics independent from list pagination and returned task-run rows.
- Define authorization and failure behavior for statistics explicitly.
- Measure the selected option's cold and cached page latency.
- Keep shared filters, result messages, loaders, and calculation code independent of the public API boundary.

## Evidence

- [Backend API design](../deliverables/lesson-module-statistics-eng-design.md#api-design)
- [Frontend loading contract](../deliverables/lesson-module-statistics-fe-design.md#loading-and-prefetch-contract)
- [API design review session](../sessions/2026-08-17/codex-stats-api-design-review.md)
