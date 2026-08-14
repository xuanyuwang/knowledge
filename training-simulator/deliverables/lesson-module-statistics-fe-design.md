# Training Simulator Lesson and Module Statistics — Frontend Design

Authors: xuanyu.wang@cresta.ai
Status: Draft for review
Last updated: 2026-08-13
Related: [requirements brief](./lesson-module-statistics-reporting.md), [backend design](./lesson-module-statistics-eng-design.md)

## Decision summary

Integrate lesson and module statistics into the existing Lesson Configuration sub-tabs first:

- fill the existing `Sessions assigned` lesson column from the lesson-stats API
- fill the existing `Active Lessons` module column from the module-stats API
- open a lesson statistics drawer and a module statistics drawer from those reporting cells/actions
- reuse the page's Assignee and Date Range filters for both tables and drawers

This is the smallest path through the current Director architecture. If product chooses a manager-visible Insights tab, keep the data hooks/view models/drawers and mount them under the new route; do not fork the reporting logic.

## Current frontend state

Validated against `director/main` at `4a0962688d`.

- `TrainingSimulator.tsx` owns the global Assignee and Date Range filters.
- `TrainingSimulatorTabs.tsx` passes `filterState` only to `TrainingSessions`; `LessonConfiguration` currently ignores it.
- `LessonsTab.tsx` loads up to 200 lessons and hard-codes `sessionsAssigned: 0` behind `enableTrainingSimulatorV2` (`CONVI-6995`).
- `ModulesTab.tsx` loads up to 200 modules and hard-codes `activeLessons: 0` behind the same flag (`CONVI-6996`).
- The API wrapper and React Query hook exist only for `RetrieveTrainingSimulatorTaskStats`.
- The session screen already supplies reusable patterns for filter expansion, aggregate view models, `FullDrawer`, percentage formatting, loading states, and tests.
- Lesson/module names currently navigate to the authoring routes. Reporting interactions must not replace those edit links.

## User experience

### Lesson table

Keep the current columns. Replace the placeholder `Sessions assigned` value with `lessonStats.sessionCount`.

The count is a button/link-style cell that opens the lesson statistics drawer. The lesson name remains the edit-content link. If design prefers an explicit `View results` row action, only the trigger changes; the drawer contract remains the same.

### Lesson drawer

Header:

- lesson title
- assigned, started, completed
- average score
- pass rate
- total attempts/retried agents when design confirms placement

Body:

- one row/card per required module
- module display name
- passed/failed fraction and pass rate
- average score
- incomplete/N-A state when applicable
- link/button to open the module drawer without leaving the page

Mixed-revision or missing-snapshot warnings are visible but non-blocking. Empty rates render `--`, not `0%`.

### Module table

Replace `activeLessons: 0` with `moduleStats.activeLessonCount`. Make the reporting cell or explicit results action open the module drawer. The module name remains the authoring link.

### Module drawer

Header:

- module display name
- active lesson count
- assigned, started, completed
- average score and current passing score
- pass rate

Body:

- criterion list for conversation modules
- display name
- passed/failed/N-A counts
- pass fraction and pass rate
- most-missed criteria sorted by failed count descending, then display name

Quiz-only modules show the aggregate outcome section and a localized “No criterion breakdown for quiz modules” state.

### Filters

The existing page filters apply consistently to outcome metrics in Training Sessions, Lessons, and Modules:

- Date Range maps to backend `time_range`.
- selected users, teams, and groups resolve to user resource names before the stats request.
- content search remains local and does not change the stats request; it only filters rendered rows.

`Sessions assigned` follows the selected outcome cohort. `Active Lessons` is current content metadata and remains stable when date/assignee filters change.

Changing a filter refetches stats and keeps the content list cached. Close an open drawer when its content item leaves the filtered content list; otherwise keep it open and update its numbers.

## Data flow

```text
TrainingSimulator
  └─ filterState
      └─ TrainingSimulatorTabs
          └─ LessonConfiguration
              ├─ LessonsTab
              │   ├─ ListTrainingLessons
              │   └─ RetrieveTrainingSimulatorLessonStats(names + cohort)
              └─ ModulesTab
                  ├─ ListTrainingModules
                  └─ RetrieveTrainingSimulatorModuleStats(names + cohort)
```

Do not issue one stats request per row. Each active sub-tab issues one batch request for its loaded content names (currently at most 200).

## API layer

After regenerated web-client types are available:

1. Add `retrieveTrainingSimulatorLessonStats` and `retrieveTrainingSimulatorModuleStats` to `packages/director-api/src/services/cresta-api/training-simulator/trainingSimulatorApi.ts`.
2. Add distinct query keys to `DirectorQueryKey`:
   - `USE_TRAINING_SIMULATOR_LESSON_STATS`
   - `USE_TRAINING_SIMULATOR_MODULE_STATS`
3. Add hooks under `packages/director-app/src/hooks/training-simulator/`:
   - `useTrainingSimulatorLessonStats.ts`
   - `useTrainingSimulatorModuleStats.ts`
4. Export them from the scoped training-simulator hook barrel.

Query keys include the full normalized request. Build stable arrays (sorted user names, content names in list order) so harmless rerenders do not refetch.

Enable a stats query only when:

- the content list succeeded
- there is at least one valid content resource name
- team/group membership resolution is complete
- the feature flag and access gate allow the reporting UI

## Filter adapter

`TrainingSessions.tsx` already expands teams/groups to users, but the helper currently lives under the training-sessions feature. Move or extract that pure mapping to a shared Training Simulator location, for example:

```text
features/training-simulator/reporting/buildReportingCohort.ts
```

Return a discriminated state instead of overloading `undefined`:

```ts
type ReportingCohort =
  | { status: 'loading' }
  | { status: 'ready'; userNames: string[] | undefined };
```

`undefined` means “no assignee filter”; an empty array after a selected team/group resolves means “the selected cohort has no users” and should short-circuit to zero stats rather than query all users.

## View models

Keep protobuf types out of presentation components. Add pure adapters close to the reporting feature:

```text
features/training-simulator/reporting/
  outcomeSummary.ts
  lessonStatsViewModel.ts
  moduleStatsViewModel.ts
  formatReportingValue.ts
```

Suggested types:

```ts
interface OutcomeSummaryViewModel {
  assignedCount: number;
  startedCount: number;
  completedCount: number;
  incompleteCount: number;
  passedCount: number;
  failedCount: number;
  notApplicableCount: number;
  averageScore: number | null; // FE 0–100
  passRate: number | null;     // fraction for fmtPercentage
  totalAttemptCount: number;
  retriedAgentCount: number;
}
```

The backend contract returns scores as 0.0–1.0. Convert scores to the FE's 0–100 display scale once in the adapter, following `agentScore.ts`; leave pass rate as 0.0–1.0 for `fmtPercentage`.

Index response entries by full resource name, not parsed ID. Merge with content rows without mutating the original lesson/module objects.

Missing stats behavior:

- while request is loading: reporting cells show skeleton/loading state
- valid response with zero assignments: show `0` counts and `--` rates
- response omits a requested name: treat as an API-contract error, show `--`, and emit client telemetry
- request fails: content authoring table remains usable; reporting cells show `--` and the standard toast appears once

## Component plan

```text
features/training-simulator/
  reporting/
    buildReportingCohort.ts
    outcomeSummary.ts
    components/
      OutcomeStatGrid.tsx
      ReportingWarning.tsx
      PassRateValue.tsx
  tabs/lesson-configuration/
    LessonConfiguration.tsx
    LessonsTab.tsx
    ModulesTab.tsx
    lesson-stats-drawer/
      LessonStatsDrawer.tsx
      LessonModuleResults.tsx
    module-stats-drawer/
      ModuleStatsDrawer.tsx
      CriterionResults.tsx
```

Reuse:

- `FullDrawer` for both drawers
- `DirectorTable` for the content and criterion/module lists
- existing empty placeholder and percentage formatters
- `useMemoDisclosure` for drawer state
- shared outcome-stat components between drawers

Do not reuse session-specific view models directly: session rows contain per-agent data, while these APIs return already-aggregated content outcomes.

## State and interaction

Each sub-tab owns only the selected content resource name and drawer disclosure:

```ts
const [selectedLessonName, setSelectedLessonName] = useState<string>();
```

Resolve the current drawer data from the stats map on every render. Do not copy the full stats object into state, which would become stale after filter refetch.

When opening module details from the lesson drawer, prefer a single module stats request already cached by the Modules tab. Because that tab may never have mounted, the lesson drawer may lazily call `RetrieveTrainingSimulatorModuleStats` for one module. Use the same query key/request builder so it deduplicates with the table query when the cohorts match.

## Access and routing

The current Lesson Configuration route is restricted to global admins and QA admins. That is safe for the first integration and matches the backend's initial recommended roles.

Managers are a primary reporting persona but cannot reach Lesson Configuration. Product must choose one of these before manager rollout:

1. add a manager-visible Insights tab and mount the same reporting components there; or
2. split Lesson Configuration into view/report permissions versus authoring permissions.

Do not broaden `canConfigureLessons` merely to expose reports; that would also expose create/edit routes.

## Feature flags

Use `enableTrainingSimulatorV2` for the existing placeholder columns and drawers if they ship together. If backend/reporting rollout must be independent from archive/edit V2 behavior, introduce a dedicated reporting flag rather than coupling release safety to unrelated V2 features.

When disabled:

- do not issue the new stats requests
- preserve the current hidden reporting columns
- keep existing authoring/session behavior unchanged

## Accessibility and localization

- Drawer triggers are real buttons or links with content-specific accessible labels.
- Counts alone are not color-coded; pass/fail state has text and icon semantics.
- Drawer focus returns to its trigger on close.
- All visible text and empty/error labels use the existing `director-app-coaching` Training Simulator namespace.
- Do not rely on red/amber/green alone for criterion performance.

## Testing plan

Pure adapter tests:

- score scaling and pass-rate formatting
- absent optionals become `null`, not zero
- response indexing and stable ordering
- zero, incomplete, all-N/A, and mixed-revision outcomes
- criterion sort and quiz empty state
- selected team/group resolving to zero users does not become an all-user query

Hook/API tests:

- parent injection in `TrainingSimulatorApi`
- correct query keys and request payloads
- query disabled during cohort/content loading
- refetch on date/assignee change
- no per-row request fan-out

Component tests:

- lesson/module counts replace placeholders
- authoring links still navigate to edit routes
- reporting trigger opens the correct drawer
- loading/error/zero states
- filter changes refresh an open drawer
- module drill-through from lesson drawer
- feature flag and role gates
- keyboard open/close/focus behavior

End-to-end:

- QA admin filters a cohort, opens lesson and module drawers, and sees numbers matching the session cohort
- manager does not gain authoring access
- more than 100 content items are fetched in one bounded batch without row fan-out

## Delivery sequence

1. Land generated API types and Director API wrapper methods.
2. Add query keys, hooks, cohort adapter, and pure view models.
3. Thread `filterState` through `TrainingSimulatorTabs` → `LessonConfiguration` → both sub-tabs.
4. Replace placeholder count columns.
5. Build the lesson drawer and module drill-through.
6. Build the module drawer and criterion breakdown.
7. Add localization, accessibility, unit/component tests.
8. Validate against staging cohorts and enable behind the selected feature flag.

## Review gates

- Product/design: first route, drawer trigger, visible metric set, criterion thresholds/colors, quiz empty state.
- Backend: score scale, missing-row behavior, batch size, response ordering, filter semantics.
- Security: roles for the first surface and manager rollout plan.
- QA: cross-grain reconciliation with Training Sessions and snapshot/mixed-revision warnings.
