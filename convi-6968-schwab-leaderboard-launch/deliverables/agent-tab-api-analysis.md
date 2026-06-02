# Agent Tab API Analysis

## Goal

For the Agent leaderboard tab, the feature needs two different kinds of data:

1. Aggregate: get the number of scorecards for each agent all at once.
2. Drill-down: after clicking `# of scorecards`, fetch the scorecards for one agent and group them by template for the side drawer.

The key design question is whether one API can do both jobs well, or whether the table and the drawer should use different APIs.

## Filters That Matter On This Tab

The current Agent tab QA score path is built from the QA filter state. From the frontend request builders, the relevant filter surface is:

- date range
- date range target
- users / teams / groups
- conversation duration buckets
- selected scorecard template
- selected criteria within the template
- include N/A scored
- voicemail / moment-group filtering
- use case
- score range
- exclude deactivated users
- scorecard status
- score resource
- `listAgentOnly`

Any API chosen for the Agent tab should be judged against this filter surface, not just against raw ability to return scorecards.

## API Comparison

| API | Main purpose | Filter support on Agent tab | Can get aggregate counts for all agents at once? | Can get scorecard details for one agent grouped by template? | Main strengths | Main weaknesses |
|-----|--------------|-----------------------------|--------------------------------------------------|--------------------------------------------------------------|----------------|-----------------|
| `RetrieveQAScoreStats` | Aggregated QA score and QA scorecard metrics | Best parity with current Agent QA path. Supports the QA filter family plus `scoreResource`, `conversationTimeRangeField`, and `filterToAgentsOnly`. | Yes, directly via group-by agent. | No, not directly. It has no template group-by dimension. | Best semantic match to the current Agent tab performance data. Efficient for the main table. | Cannot return `agent x template` in one call. Template is only a filter, not a group-by dimension. |
| `RetrieveQAConversations` | Detailed QA score rows / scorecard-level drill-down data | Almost the same QA filter family as `RetrieveQAScoreStats`, but currently missing `filterToAgentsOnly`. | Yes, technically, by fetching all pages and aggregating on FE. | Yes, by filtering to one agent and grouping rows by `scorecardTemplateId` on FE. | Best current API for template drill-down. QA-scoped, not a generic scorecard list. | Heavy for the main table because it is paginated detail data. Not full parity until `filterToAgentsOnly` is supported. |
| `RetrieveScorecardStats` | Aggregated completed scorecard stats in generic Insights APIs | Partial parity only. Good for generic attribute filters, but not a full match for the Agent tab QA score filter surface. | Yes, directly via group-by agent. | No, not directly. No template breakdown in response. | Efficient aggregate API. | Not the current Agent QA source of truth. Missing QA-specific semantics and template drill-down. |
| `ListScorecards` | Raw scorecard listing API | Lowest parity. Good scorecard-list filters, but not a full match for Agent-tab QA filters like duration buckets, criterion filters, score ranges, include N/A, voicemail, score resource, or `listAgentOnly`. | Yes, technically, by fetching all pages and aggregating on FE. | Yes, directly from raw scorecard rows grouped by template on FE. | Most explicit raw scorecard data source. | Semantically farthest from the current Agent leaderboard QA path. Highest parity risk. |

## Per-API Assessment

### `RetrieveQAScoreStats`

#### What it is best at

- Aggregate QA metrics by agent.
- Staying consistent with the current Agent tab's performance data path.
- Respecting the current QA filter semantics.

#### What it cannot do

- It cannot do `agent x template` grouping in one request.
- `QAAttributeType` has no template group-by value.
- `QAScoreGroupBy` has no template field in the response.

#### Practical consequence

It is a strong choice for job 1, but not enough for job 2 unless FE fans out one request per template and merges the results. That fan-out design is possible, but it is not ideal.

### `RetrieveQAConversations`

#### What it is best at

- Returning detailed QA-scored rows that can be grouped by scorecard and then by template.
- Supporting an on-demand drawer where FE only fetches one agent's detailed data when the user clicks.

#### What it can do

- Job 1: possible, but only by fetching all relevant detailed rows across agents and aggregating on FE.
- Job 2: strong fit, because FE can fetch one agent's rows and group by `scorecardTemplateId`.

#### Practical consequence

This is the best current API for the drawer, but not the best current API for the main table.

### `RetrieveScorecardStats`

#### What it is best at

- Generic scorecard-completed aggregation.

#### Why it is weak for this tab

- The Agent tab today treats QA score APIs as the source of truth for performance-related data.
- `RetrieveScorecardStats` is not the current Agent-tab QA path.
- It does not expose row detail or template breakdown.
- It does not naturally match the full QA filter surface of the Agent tab.

#### Practical consequence

It is not the leading option for either job on the Agent tab.

### `ListScorecards`

#### What it is best at

- Raw scorecard listing.
- Direct access to template and agent fields on scorecards.

#### Why it is weak for this tab

- It is not QA-specific.
- It loses too much filter parity relative to the current Agent tab.
- It pushes both jobs into FE-side pagination and aggregation.

#### Practical consequence

It can technically do both jobs, but it is the least aligned with the current Agent-tab semantics.

## Can One API Do Both Jobs?

### Option A: `RetrieveQAScoreStats` only

Possible only if FE fans out per template and merges results. This is not ideal.

### Option B: `RetrieveQAConversations` only

Possible if FE fetches all detailed QA rows and aggregates them for the main table, then also reuses it for the drawer.

This is the strongest single-API option, but it has two downsides:

- it is heavier than using an aggregate API for the main table
- it does not fully match Agent-tab filter parity until `filterToAgentsOnly` is supported

### Option C: `RetrieveScorecardStats` only

Not enough, because it cannot power the template-grouped drawer.

### Option D: `ListScorecards` only

Technically possible, but the filter and semantics gap is too large for the Agent tab.

## Recommended Design For Agent Tab

### Recommended split

| Job | Recommended API | Why |
|-----|-----------------|-----|
| Main leaderboard table: aggregate `# of scorecards` for all agents | `RetrieveQAScoreStats` | Best alignment with the current Agent tab QA score semantics and filters. Efficient aggregate API. |
| Side drawer: scorecards for one agent grouped by template | `RetrieveQAConversations` | Best available detailed QA API for on-demand drill-down by template. |

### Why this split is the best current design

- The table only needs aggregate counts, so it should use the aggregate API.
- The drawer only opens for one agent at a time, so it is acceptable to fetch detail data on demand.
- This preserves consistency with the current Agent leaderboard behavior on the main table.
- It avoids fetching all detailed scorecard rows for all agents upfront.

## Conditional Improvement

If `RetrieveQAConversations` adds `filterToAgentsOnly`, then the drawer path becomes much cleaner:

- it stays in the same QA API family as the main Agent tab score path
- it supports the same practical filter semantics more closely
- it remains a good fit for template-grouped drill-down

Even with that improvement, `RetrieveQAScoreStats` is still the better fit for the main aggregate table because it is purpose-built for grouped QA stats.

## Bottom Line

For the Agent tab:

- Best API for job 1: `RetrieveQAScoreStats`
- Best API for job 2: `RetrieveQAConversations`
- Best overall design: two APIs, one for the table and one for the drawer

If the team insists on a single API, `RetrieveQAConversations` is the only realistic candidate, but it is a weaker design than the split approach.
