# Training Simulator Lesson and Module Statistics — Frontend Design

Authors: xuanyu.wang@cresta.ai
Status: Draft for review
Last updated: 2026-08-28
Related: [requirements brief](./lesson-module-statistics-reporting.md), [backend design](./lesson-module-statistics-eng-design.md)

## Design summary

Integrate lesson and module statistics into the existing Lesson Configuration sub-tabs first:

- fill the existing `Sessions assigned` lesson column from the lesson-stats API
- fill the existing `Active Lessons` module column from the module-stats API
- open a lesson statistics drawer and a module statistics drawer from those reporting cells/actions
- reuse the page's Assignee and Date Range filters for both tables and drawers

This is the smallest path through the current Director architecture. If product chooses a manager-visible Insights tab, keep the data hooks/view models/drawers and mount them under the new route; do not fork the reporting logic.

The UI must surface backend historical-correctness warnings. It must not display an ambiguous legacy/partial training result as a real 0%, failure, or N/A result.

The API boundary closed on 2026-08-28 with dedicated lesson and module statistics RPCs selected. Each active sub-tab loads content, then issues one batch stats request for the loaded resource names. Keep view-model construction separate from transport, use distinct statistics query keys, preserve content when reporting fails, and mitigate cold tab switching through cached content names and inactive-tab prefetch. See [the decision record](../decisions/2026-08-28-dedicated-lesson-module-stats-apis.md).

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

Missing-result-status warnings are visible but non-blocking. Empty rates render `--`, not `0%`.

### Module table

Replace `activeLessons: 0` with `moduleStats.activeLessonCount`. Make the reporting cell or explicit results action open the module drawer. The module name remains the authoring link.

### Module drawer

Header:

- module display name
- active lesson count
- assigned, started, completed
- average score and the current passing score from the listed module's evaluation config
- pass rate

Body:

- criterion list for conversation modules
- display name
- passed/failed/N-A counts
- pass fraction and pass rate
- most-missed criteria sorted by failed count descending, then display name

Quiz modules are excluded from statistics in this project because quiz is not yet mature enough to define reliable reporting semantics.

### Filters

The existing page filters apply consistently to training-result metrics in Training Sessions, Lessons, and Modules:

- Date Range maps to backend `time_range`.
- selected users map to `user_names`.
- selected dynamic/virtual groups map directly from the filter's `groupNames` to each list request's `stats_virtual_group_names`.
- selected teams map directly from the filter's `teamNames` to each list request's `stats_team_group_names`.
- Director does not need to expand group membership before the stats request. The backend resolves membership using the already-separated group types.
- content search remains local and does not change the stats request; it only filters rendered rows.

`Sessions assigned` follows the selected users and date range. `Active Lessons` is current content metadata and remains stable when date/assignee filters change.

Changing a filter refetches stats and keeps the content list cached. Close an open drawer when its content item leaves the filtered content list; otherwise keep it open and update its numbers.

## Data flow

```text
TrainingSimulator
  └─ filterState
      └─ TrainingSimulatorTabs
          └─ LessonConfiguration
              ├─ LessonsTab
              │   ├─ ListTrainingLessons
              │   └─ RetrieveTrainingSimulatorLessonStats(names + typed user/team/group selection)
              └─ ModulesTab
                  ├─ ListTrainingModules
                  └─ RetrieveTrainingSimulatorModuleStats(names + typed user/team/group selection)
```

Do not issue one statistics request per row. Each active sub-tab issues one dedicated batch statistics request for its loaded content names (currently at most 200).

### Loading and prefetch contract

- Start a tab's content-list request immediately and start its batch statistics request as soon as content names are available.
- Reuse cached content names to start list and statistics work effectively in parallel on later visits; prefetch the inactive tab only when its content-name list and access gates are available.
- Use the same normalized user-selection and query-key builders for foreground loads, background prefetch, and lesson-to-module drill-through.
- Do not block the authoring table on stats failure; content remains interactive while reporting cells show `--`.
- Instrument cold first-tab, cold tab-switch, and cached tab-switch latency. Consolidate transport only if measured latency misses the agreed target after prefetch/cache tuning.

## API layer

After regenerated web-client types are available:

1. Update `packages/director-api/src/services/cresta-api/training-simulator/trainingSimulatorApi.ts` with dedicated lesson/module statistics request support.
2. Include the complete statistics filter and ordered content-name batch in distinct, stable statistics query keys.
3. Add hooks under `packages/director-app/src/hooks/training-simulator/`:
   - `useTrainingSimulatorLessonStats.ts`
   - `useTrainingSimulatorModuleStats.ts`
4. Export them from the scoped training-simulator hook barrel.

Query keys include the full normalized request. Build stable arrays (sorted user, virtual-group, and team-group names; content names in list order) so harmless rerenders do not refetch.

Enable a stats query only when:

- the content list succeeded
- there is at least one valid content resource name
- the feature flag and access gate allow the reporting UI

## Filter adapter

The page filter already stores users, teams, and dynamic groups separately in `UserTeamGroupSelection`. Preserve that information instead of merging groups or expanding them to users in Director. Add a small request mapper in a shared Training Simulator location, for example:

```text
features/training-simulator/reporting/buildReportingUserSelection.ts
```

Map the existing filter state directly:

```ts
type ReportingUserSelection = {
  userNames: string[];
  virtualGroupNames: string[]; // filterState.usersTeamsGroups.groupNames
  teamGroupNames: string[];    // filterState.usersTeamsGroups.teamNames
  directTeamOnly: boolean;
};
```

When every array is empty, the request has no user-selection narrowing beyond authorization. A non-empty group array remains a real filter even if that group ultimately has no members; the backend must return zero matching assignments rather than treat it as an all-user request.

On the backend, map these fields directly to `UserFilterConditions.SelectedUserNames`, `SelectedVirtualGroupNames`, and `SelectedTeamGroupNames`. User Service still resolves group membership, but the backend avoids a separate `GroupsByGroupType` RPC used only to discover whether each group is a team or a dynamic group.

## View models

Keep protobuf types out of presentation components. Add pure adapters close to the reporting feature:

```text
features/training-simulator/reporting/
  trainingResultSummary.ts
  lessonStatsViewModel.ts
  moduleStatsViewModel.ts
  formatReportingValue.ts
```

Suggested types:

```ts
interface TrainingResultSummaryViewModel {
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
  resultStatusMissing: boolean;
}
```

The backend contract returns scores as 0.0–1.0. Convert scores to the FE's 0–100 display scale once in the adapter, following `agentScore.ts`; leave pass rate as 0.0–1.0 for `fmtPercentage`.

Index response entries by full resource name, not parsed ID. Merge with content rows without mutating the original lesson/module objects.

Missing stats behavior:

- while request is loading: reporting cells show skeleton/loading state
- valid response with zero assignments: show `0` counts and `--` rates
- response warns that result status is missing: show a data-quality warning and `--` for affected score/pass/N-A values
- response omits a requested name: treat as an API-contract error, show `--`, and emit client telemetry
- request fails: content authoring table remains usable; reporting cells show `--` and the standard toast appears once

## Component plan

```text
features/training-simulator/
  reporting/
    buildReportingUserSelection.ts
    trainingResultSummary.ts
    components/
      TrainingResultStatGrid.tsx
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
- shared training-result components between drawers

Do not reuse session-specific view models directly: session rows contain per-agent data, while these APIs return already-aggregated training results.

## State and interaction

Each sub-tab owns only the selected content resource name and drawer disclosure:

```ts
const [selectedLessonName, setSelectedLessonName] = useState<string>();
```

Resolve the current drawer data from the stats map on every render. Do not copy the full stats object into state, which would become stale after filter refetch.

When opening module details from the lesson drawer, first reuse module statistics already cached by the Modules tab. If that tab has not loaded, fetch the module through the selected API contract. Use the same query-key and request-building helpers so matching requests deduplicate.

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
- zero, incomplete, and all-N/A results
- partial/legacy result warning and criterion sort
- a selected team or virtual group with zero members does not become an all-user query

Hook/API tests:

- parent injection in `TrainingSimulatorApi`
- correct query keys and request payloads
- request maps `groupNames` to `virtualGroupNames` and `teamNames` to `teamGroupNames`
- query waits for content loading but not a frontend group-membership expansion
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

- QA admin selects users, opens lesson and module drawers, and sees numbers matching the session filters
- manager does not gain authoring access
- more than 100 content items are fetched in one bounded batch without row fan-out

## Delivery sequence

1. Approve the API boundary, direct per-request reporting fields, persisted conversation result status/N-A, score/empty, warning, and loading contracts.
2. Land generated API types and Director API wrapper methods.
3. Add query keys, hooks, user-selection adapter, and pure view models.
4. Thread `filterState` through `TrainingSimulatorTabs` → `LessonConfiguration` → both sub-tabs.
5. Replace placeholder count columns.
6. Build the lesson drawer and module drill-through.
7. Build the module drawer and criterion breakdown, including missing-result-status behavior.
8. Add localization, accessibility, unit/component tests.
9. Validate against staging user selections and enable behind the selected feature flag.

## Review gates

- Product/design: first route, drawer trigger, visible metric set, and criterion thresholds/colors.
- Backend: API boundary and proto examples, conversation result-status prerequisite, score scale, warning/empty behavior, batch size, response ordering, and filter behavior.
- Security: roles for the first surface and manager rollout plan.
- QA: cross-level reconciliation, missing-result-status warnings, and cold/cached tab-loading measurements.
