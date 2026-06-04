# Manager Tab API Analysis

## Goal

For the Manager leaderboard tab, the feature needs two different kinds of data:

1. Aggregate: get the current `Scorecards completed` count for each manager all at once.
2. Drill-down: after clicking `Scorecards completed`, fetch the completed scorecards for one manager and group them by template for the side drawer.

The Manager tab is different from the Agent tab because the current page already has a `Scorecards completed` metric, and that metric is powered by a generic scorecard stats API rather than the QA score API family.

Product clarification: the side drawer should align with the current `Scorecards completed` metric.

MVP/future API clarification:

- For MVP, use `ListScorecards` for the Manager drawer because it already supports `submitterUserNames`, submit-time range, and template grouping on FE.
- Build the Manager drawer data fetching behind an abstraction so the UI is not coupled to `ListScorecards`.
- In the future, prefer `RetrieveQAConversations` once it supports filtering by scorecard submitter and submit-time range. The main reasons are that it reads from ClickHouse, aligns better with other Insights APIs, and should be faster than the MVP list path.

## Current Manager Tab Baseline

The current Manager tab uses `RetrieveScorecardStats` through `useScorecardStats(...)`.

The request construction does the following:

- derives manager-like users from selected child teams
- included roles are `MANAGER`, `MANAGER_2ND`, `QA_SPECIALIST`, `QA_ADMIN`, and `ADMIN`
- filters out dev users
- builds `usersTeamsGroups.userNames` from those manager users
- sends `conversationDurationBuckets`
- uses the selected date range
- groups by `ATTRIBUTE_TYPE_AGENT`
- uses frequency `daily`

The row value shown in the table is `averageScorecardCompletedPerUser`, not `scorecardCompleted`.

The UI tooltip says this metric is `# of scorecards each manager completed`, but the backend implementation is more specific: it counts distinct scorecards with `scorecard_submit_time` in the selected date range, and attributes those scorecards to `submitter_user_id`.

## What "Completed" Means Today

For `RetrieveScorecardStats`, a scorecard is counted as completed when it satisfies the scorecard stats query:

- the row is in `scorecard_d`
- `scorecard_id` is counted distinctly
- `scorecard_submit_time` falls inside the selected date range
- `submitter_user_id` is non-empty
- the row matches the request filters, including manager-user filtering and conversation duration buckets when present

The query does not require:

- `publish_time`
- `scorecard_acknowledge_time`
- `manually_scored = true`
- `submitter_user_id = creator_user_id`
- a non-null or non-zero final score

The current grouping and attribution are based on `submitter_user_id`, not `creator_user_id`. That means the current Manager tab metric is best described as:

`Distinct submitted scorecards, attributed to the scorecard submitter.`

This is close to "completed by manager" when the submitter is treated as the manager who completed the scorecard. If scorecard creator and scorecard submitter can differ, the current metric follows submitter, not creator.

Important time-basis detail:

- `RetrieveScorecardStats` rewrites the scorecard table time filter from `scorecard_time` to `scorecard_submit_time`.
- `RetrieveScorecardStats` also rewrites `agent_user_id` to `submitter_user_id`, so `ATTRIBUTE_TYPE_AGENT` grouping/filtering is submitter-attributed for this endpoint.
- `ListScorecards` can mirror this with `startSubmitTime` and `endSubmitTime`, which filter Postgres `scorecards.submitted_at`.
- `RetrieveQAConversations` does not mirror this by default. Its score/scorecard table time range uses `scorecard_time` unless the request switches to conversation-ended time, in which case it joins the conversation table and filters `conversation_end_time`.

## Filters That Matter On This Tab

For the existing Manager `Scorecards completed` metric, the currently wired filter surface is narrower than the Agent QA path:

- date range
- selected teams, transformed into manager-like users
- conversation duration buckets

Important caveat:

The broader leaderboard page may carry more filter state, but the current Manager scorecard stats request does not use the full QA filter surface. For this tab, consistency should be judged against what the Manager tab actually sends today.

## Metric Semantics To Choose

Before choosing the drawer API, the product definition needs to be explicit:

| Meaning | Practical interpretation |
|---------|--------------------------|
| Completed/submitted by manager, matching current aggregate | Distinct submitted scorecards attributed to `submitter_user_id`. |
| Created by manager | Scorecards whose `creator_user_id` is the manager. |
| Reviewed by manager | QA scorecards whose reviewer audience or QA reviewer identity resolves to the manager. |
| Exact extension of current aggregate | Drawer rows must reconcile to the existing `RetrieveScorecardStats` number. |

These are not guaranteed to be the same row set.

## API Comparison

| API | Main purpose | Filter support on Manager tab | Can get aggregate counts for all managers at once? | Can get scorecard details for one manager grouped by template? | Main strengths | Main weaknesses |
|-----|--------------|-------------------------------|----------------------------------------------------|----------------------------------------------------------------|----------------|-----------------|
| `RetrieveQAScoreStats` | Aggregated QA score and QA scorecard metrics | Good QA filter support, but not equivalent to the current Manager tab scorecard path because QA user grouping is still `agent_user_id`. | Possible for agent-attributed QA aggregate, but not the current submitter-attributed Manager `Scorecards completed` contract. | No, not directly. No template group-by and no row detail. | Efficient QA aggregate for agent QA scorecard metrics. | Does not match the current Manager tab baseline. Cannot group by `submitter_user_id` and cannot power the template drawer directly. |
| `RetrieveQAConversations` | Detailed QA score rows / scorecard-level drill-down data | Good for Agent-style QA scorecard drill-down, but weak for current Manager parity until it can filter by scorecard submitter and submit-time range. Although the proto has `scorecardReviewerAudience`, the traced ClickHouse implementation path does not read that field. | Technically possible by fetching all pages and aggregating on FE, but not recommended for the main table. | Future target after adding submitter filtering and submit-time support. | Reads from ClickHouse, aligns with other Insights APIs, and should be faster than the MVP list path. | Today it cannot filter to the selected manager by `submitter_user_id`, so it cannot match the current Manager `Scorecards completed` drawer requirement yet. |
| `RetrieveScorecardStats` | Aggregated completed scorecard stats | Best parity with the current Manager tab because this is the API already used. | Yes, directly via group-by user/agent over manager users. | No. Response has no template dimension and no row detail. | Best source for current main table consistency. Efficient aggregate API. | Cannot power the template drawer. Exact row-level equivalent is not exposed. |
| `ListScorecards` | Raw scorecard listing API | Good for explicit scorecard fields such as submitter, creator, template, agent, group, and submit time. Weaker parity with generic Insights filters like duration buckets. | Technically possible by fetching pages and aggregating on FE, but not ideal for the main table. | Yes, for the MVP drawer requirement when using `submitterUserNames` plus submit-time range. | Best current raw API for submitter-attributed scorecard rows grouped by template. `submitterUserNames` is the closest row-level match for the current aggregate. | Not guaranteed to match `RetrieveScorecardStats` without validation. Filter parity is weaker. Should be hidden behind a data-provider abstraction for future replacement. |

## Per-API Assessment

### `RetrieveScorecardStats`

#### What it is best at

- Current Manager leaderboard aggregate.
- Returning scorecard completed counts for all manager rows at once.
- Matching the existing Manager tab's metric and widget behavior.

#### What it cannot do

- It cannot group by template.
- It cannot return row-level scorecard details for the drawer.
- It does not expose a proven row-level equivalent in the frontend model.

#### Practical consequence

It is the best API for job 1 if we want consistency with the existing Manager tab, but it is not enough for job 2.

### `RetrieveQAConversations`

#### What it is best at

- Returning detailed QA scorecard rows.
- Providing scorecard and template identifiers that FE can group by template.

#### What it can do

- Job 1: possible only by fetching all detailed rows and aggregating on FE, which is not recommended for the main Manager table.
- Job 2: not a fit for the current Manager drawer requirement because the drawer must align with `Scorecards completed`.

#### Practical consequence

This is not the right drawer API for the current Manager requirement today.

Future target:

If `RetrieveQAConversations` gains scorecard submitter filtering and scorecard submit-time filtering, it should become the preferred Manager drawer provider. That would keep the row-level drill-down on ClickHouse, align it more closely with the other Insights APIs, and likely improve performance versus `ListScorecards`.

#### How it gets "reviewed" or completed scorecards

`RetrieveQAConversations` does not have a separate "reviewed by manager" criterion in the traced ClickHouse implementation.

The relevant scorecard criteria are:

- It reads score detail from `score_d` by default, or scorecard-level rows from `scorecard_d` when `ScoreResource = QA_SCORE_RESOURCE_SCORECARD`.
- It dedupes to the latest `scorecard_last_update_time` per `scorecard_id`.
- If `scorecard_statuses` contains `MANUALLY_SUBMITTED`, it filters `scorecard_submit_time <> 0`.
- If `scorecard_statuses` contains `DRAFT`, it filters `scorecard_submit_time = 0 AND manually_scored = true`.
- If `scorecard_statuses` contains `AUTO`, it filters `scorecard_submit_time = 0 AND manually_scored = false`.
- It can filter agent/user through `agent_user_id`.
- It can filter template through `scorecard_template_id`.
- It can filter score type, score range, criteria, moments, and conversation duration.

What it does not do for this use case:

- It does not filter scorecards by `creator_user_id`.
- It does not filter scorecards by `submitter_user_id`.
- It does not apply `scorecard_reviewer_audience` in the traced `RetrieveQAConversations` ClickHouse path, even though that field exists in proto and is used by other QA/manual-QA stats paths.
- It does not use `scorecard_submit_time` as the default time range basis.

So `RetrieveQAConversations` can answer "scorecards for this agent/template/filter set that are manually submitted", but it cannot answer "scorecards completed by this manager" in the same sense as the current Manager tab.

### `ListScorecards`

#### What it is best at

- Raw scorecards created or submitted by a specific user.
- Grouping one manager's scorecards by template in FE.

#### What it can do

- Job 1: technically possible by fetching many raw scorecards and aggregating by submitter, but this is not ideal.
- Job 2: strong fit if the drawer means "scorecards created by this manager" or "scorecards submitted by this manager".

#### Practical consequence

This is the best MVP drawer API if the product definition follows the current aggregate semantics. To align with the updated `RetrieveScorecardStats`, use `submitterUserNames` and submit-time range first; use `creatorUserNames` only if product intentionally wants creator-based semantics.

The implementation should normalize `ListScorecards` results into a drawer-specific data model instead of passing raw `ListScorecards` response shapes through the UI. That keeps the future switch to `RetrieveQAConversations` small.

### `RetrieveQAScoreStats`

#### What it is best at

- Aggregated QA score metrics.
- Potential reviewer-oriented aggregate if the backend group-by is used for QA analyst style reporting.

#### Why it is weak for this tab

- It is not the current Manager tab scorecard stats path.
- It cannot produce template-grouped drawer details.
- Its `QA_ATTRIBUTE_TYPE_AGENT` grouping is agent-attributed, not scorecard-submitter-attributed.

#### Practical consequence

It is not the leading option for the Manager tab unless backend adds a submitter group-by/filter path and uses `scorecard_submit_time` as the time basis for this metric.

## Can One API Do Both Jobs?

### Option A: `RetrieveScorecardStats` only

Not enough. It handles the main table but cannot return template-grouped details.

### Option B: `RetrieveQAConversations` only

Not enough for the current Manager requirement. It would make the main table depend on paginated detail data, and it cannot filter the selected manager by the attribution field that matters here: `submitter_user_id`.

### Option C: `ListScorecards` only

Possible if the entire Manager scorecard metric is fetched from raw scorecards. This gives row detail and template grouping, but the main table would no longer use the current Manager aggregate source.

### Option D: `RetrieveQAScoreStats` only

Not enough for this feature because it cannot power the template-grouped drawer.

## Recommended Design For Manager Tab

### Main table

| Job | Recommended API | Why |
|-----|-----------------|-----|
| Main leaderboard table: aggregate `Scorecards completed` for all managers | `RetrieveScorecardStats` | This preserves the current Manager tab metric and keeps the table on the existing aggregate API. |

### Side drawer

Because the drawer should align with the current `Scorecards completed` metric, the recommended drawer API is no longer definition-dependent:

| Job | Recommended API | Why |
|-----|-----------------|-----|
| Side drawer for one manager, grouped by template | MVP: `ListScorecards` with `submitterUserNames`, `startSubmitTime`, and `endSubmitTime`; future: `RetrieveQAConversations` after adding submitter filtering and submit-time support | Current `RetrieveScorecardStats` counts distinct submitted scorecards attributed to `submitter_user_id`. `ListScorecards` is the closest available row-level API today. `RetrieveQAConversations` is preferred long term because it reads from ClickHouse and should be faster. |

Use `scorecardView = FULL` if the drawer needs template metadata and score details in the same response.

### Data-provider abstraction

Create a Manager drawer data-provider hook with a normalized return type, for example:

- `groups`: template title, template resource name or ID, count, and scorecard rows
- `totalCount`
- `isLoading`
- `isError`

The drawer UI should depend on this normalized contract only. The MVP provider can use `ListScorecards`; a future provider can use `RetrieveQAConversations` after backend submitter filtering and submit-time filtering exist.

## Recommended Product Decision

If the UI column remains named `Scorecards completed`, the drawer definition should be:

`Submitted scorecards submitted by this manager, grouped by template` -- equivalently, `scorecards whose submitter is this manager, grouped by template`.

That points to `ListScorecards` with `submitterUserNames` for the drawer, with validation against `RetrieveScorecardStats`.

If product wants "the user who created the scorecard", then the drawer should use `creatorUserNames`, but that is a different metric from the updated aggregate.

If product later wants reviewer behavior, the column or drawer copy should make that clear:

`Scorecards reviewed by this manager, grouped by template`.

That would likely need a different API path or backend support. The traced `RetrieveQAConversations` implementation does not currently apply `scorecard_reviewer_audience`.

## Bottom Line

For the Manager tab:

- Best API for job 1: `RetrieveScorecardStats`
- Best API for job 2 when matching current "completed" semantics: `ListScorecards` with `submitterUserNames`
- Best API for job 2 if "created by manager": `ListScorecards` with `creatorUserNames`
- Do not use `RetrieveQAConversations` for the Manager drawer in MVP because it cannot filter by scorecard submitter yet and does not use submit time as the default time basis
- Prefer `RetrieveQAConversations` for the Manager drawer in the future once submitter filtering and submit-time filtering are supported
- Biggest open question: whether `ListScorecards(submitterUserNames + submit-time range)` reconciles with the current `RetrieveScorecardStats` aggregate on sample data, especially around ClickHouse/Postgres sync lag and filter-surface differences

The recommended implementation should keep the main table on `RetrieveScorecardStats`, use `ListScorecards` for the MVP drawer, and hide that choice behind a data-provider abstraction so the future switch to `RetrieveQAConversations` is localized.
