# Manager Tab API Analysis

## Goal

For the Manager leaderboard tab, the feature needs two different kinds of data:

1. Aggregate: get the number of scorecards for each manager all at once.
2. Drill-down: after clicking `# of scorecards`, fetch the scorecards for one manager and group them by template for the side drawer.

The Manager tab is different from the Agent tab because the current page already has a `Scorecards completed` metric, and that metric is powered by a generic scorecard stats API rather than the QA score API family.

Product clarification: the side drawer should align with the current `Scorecards completed` metric.

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

The UI tooltip says this metric is `# of scorecards each manager completed`, but the backend implementation is more specific: it counts distinct scorecards with `scorecard_submit_time` in the selected date range, and attributes those scorecards to `creator_user_id`.

## What "Completed" Means Today

For `RetrieveScorecardStats`, a scorecard is counted as completed when it satisfies the scorecard stats query:

- the row is in `scorecard_d`
- `scorecard_id` is counted distinctly
- `scorecard_submit_time` falls inside the selected date range
- `creator_user_id` is non-empty
- the row matches the request filters, including manager-user filtering and conversation duration buckets when present

The query does not require:

- `publish_time`
- `scorecard_acknowledge_time`
- `manually_scored = true`
- `submitter_user_id = creator_user_id`
- a non-null or non-zero final score

The current grouping and attribution are based on `creator_user_id`, not `submitter_user_id`. That means the current Manager tab metric is best described as:

`Distinct submitted scorecards, attributed to the scorecard creator.`

This is close to "completed by manager" only if the creator is treated as the manager who completed the scorecard. If scorecard creator and scorecard submitter can differ, the current metric follows creator, not submitter.

Important time-basis detail:

- `RetrieveScorecardStats` rewrites the scorecard table time filter from `scorecard_time` to `scorecard_submit_time`.
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
| Completed by manager, matching current aggregate | Distinct submitted scorecards attributed to `creator_user_id`. |
| Submitted by manager | Scorecards whose `submitter_user_id` is the manager. |
| Reviewed by manager | QA scorecards whose reviewer audience or QA reviewer identity resolves to the manager. |
| Exact extension of current aggregate | Drawer rows must reconcile to the existing `RetrieveScorecardStats` number. |

These are not guaranteed to be the same row set.

## API Comparison

| API | Main purpose | Filter support on Manager tab | Can get aggregate counts for all managers at once? | Can get scorecard details for one manager grouped by template? | Main strengths | Main weaknesses |
|-----|--------------|-------------------------------|----------------------------------------------------|----------------------------------------------------------------|----------------|-----------------|
| `RetrieveQAScoreStats` | Aggregated QA score and QA scorecard metrics | Good QA filter support, but not the current Manager tab scorecard path. Can group by QA analyst in the QA API family, but that would define a reviewer-oriented metric. | Possible for reviewer-oriented aggregate, but not the current Manager `Scorecards completed` contract. | No, not directly. No template group-by and no row detail. | Efficient QA aggregate if product chooses reviewer-oriented semantics. | Does not match the current Manager tab baseline. Cannot power the template drawer directly. |
| `RetrieveQAConversations` | Detailed QA score rows / scorecard-level drill-down data | Good for Agent-style QA scorecard drill-down, but weak for current Manager parity. Although the proto has `scorecardReviewerAudience`, the traced ClickHouse implementation path does not read that field. | Technically possible by fetching all pages and aggregating on FE, but not recommended for the main table. | Not for current Manager parity. It can expose scorecard/template identifiers, but it cannot filter to the selected manager by `creator_user_id` or `submitter_user_id`. | Useful for QA detail rows on Agent-like flows. | Does not match current Manager `Scorecards completed`: wrong attribution field and usually wrong time basis. |
| `RetrieveScorecardStats` | Aggregated completed scorecard stats | Best parity with the current Manager tab because this is the API already used. | Yes, directly via group-by user/agent over manager users. | No. Response has no template dimension and no row detail. | Best source for current main table consistency. Efficient aggregate API. | Cannot power the template drawer. Exact row-level equivalent is not exposed. |
| `ListScorecards` | Raw scorecard listing API | Good for explicit scorecard fields such as submitter, creator, template, agent, group, and submit time. Weaker parity with generic Insights filters like duration buckets. | Technically possible by fetching pages and aggregating on FE, but not ideal for the main table. | Yes, for the current drawer requirement when using `creatorUserNames` plus submit-time range. | Best current raw API for scorecard rows grouped by template. `creatorUserNames` is the closer match for the current aggregate; `submitterUserNames` is a different metric. | Not guaranteed to match `RetrieveScorecardStats` without validation. Filter parity is weaker. |

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

This is not the right drawer API for the current Manager requirement.

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

This is the best drawer API if the product definition follows the current aggregate semantics. To align with `RetrieveScorecardStats`, use `creatorUserNames` and submit-time range first; use `submitterUserNames` only if product intentionally wants submitter-based semantics.

### `RetrieveQAScoreStats`

#### What it is best at

- Aggregated QA score metrics.
- Potential reviewer-oriented aggregate if the backend group-by is used for QA analyst style reporting.

#### Why it is weak for this tab

- It is not the current Manager tab scorecard stats path.
- It cannot produce template-grouped drawer details.
- It would shift the metric toward QA reviewer semantics.

#### Practical consequence

It is not the leading option for the Manager tab unless product explicitly redefines the metric as QA-reviewed scorecards and only needs aggregate data.

## Can One API Do Both Jobs?

### Option A: `RetrieveScorecardStats` only

Not enough. It handles the main table but cannot return template-grouped details.

### Option B: `RetrieveQAConversations` only

Not enough for the current Manager requirement. It would make the main table depend on paginated detail data, and it cannot filter the selected manager by the attribution fields that matter here (`creator_user_id` for current parity, or `submitter_user_id` for explicit submitter semantics).

### Option C: `ListScorecards` only

Possible if the entire Manager scorecard metric is redefined as "submitted/completed by manager". This gives row detail and template grouping, but the main table would no longer use the current Manager aggregate source.

### Option D: `RetrieveQAScoreStats` only

Not enough for this feature because it cannot power the template-grouped drawer.

## Recommended Design For Manager Tab

### Main table

| Job | Recommended API | Why |
|-----|-----------------|-----|
| Main leaderboard table: aggregate `# of scorecards` for all managers | `RetrieveScorecardStats` | This preserves the current Manager tab metric and keeps the table on the existing aggregate API. |

### Side drawer

Because the drawer should align with the current `Scorecards completed` metric, the recommended drawer API is no longer definition-dependent:

| Job | Recommended API | Why |
|-----|-----------------|-----|
| Side drawer for one manager, grouped by template | `ListScorecards` with `creatorUserNames`, `startSubmitTime`, and `endSubmitTime` | Current `RetrieveScorecardStats` counts distinct submitted scorecards attributed to `creator_user_id`. `ListScorecards` is the closest row-level API because it can filter by creator and submit time, then FE can group by template. |

Use `scorecardView = FULL` if the drawer needs template metadata and score details in the same response.

## Recommended Product Decision

If the UI column remains named `Scorecards completed`, the drawer definition should be:

`Submitted scorecards created by this manager, grouped by template`.

That points to `ListScorecards` with `creatorUserNames` for the drawer, with validation against `RetrieveScorecardStats`.

If product wants "the user who actually submitted the scorecard", then the drawer should use `submitterUserNames`, but that is a different metric from the current aggregate.

If product later wants reviewer behavior, the column or drawer copy should make that clear:

`Scorecards reviewed by this manager, grouped by template`.

That would likely need a different API path or backend support. The traced `RetrieveQAConversations` implementation does not currently apply `scorecard_reviewer_audience`.

## Bottom Line

For the Manager tab:

- Best API for job 1: `RetrieveScorecardStats`
- Best API for job 2 when matching current "completed" semantics: `ListScorecards` with `creatorUserNames`
- Best API for job 2 if "submitted by manager": `ListScorecards` with `submitterUserNames`
- Do not use `RetrieveQAConversations` for the Manager drawer if the drawer must reconcile with current `Scorecards completed`
- Biggest open question: whether `ListScorecards(creatorUserNames + submit-time range)` reconciles with the current `RetrieveScorecardStats` aggregate on sample data, especially around ClickHouse/Postgres sync lag and filter-surface differences

The recommended implementation should keep the main table on `RetrieveScorecardStats` and use `ListScorecards` for the drawer.
