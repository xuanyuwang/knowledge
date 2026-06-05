# API Decision Table

## Leaderboard Scorecard Data Options

| API | Primary entity / grain | Filter parity with current leaderboard usage | Template grouping support | Best fit for Agent tab | Best fit for Manager tab | Notes |
|------|-------------------------|---------------------------------------------|---------------------------|------------------------|--------------------------|-------|
| `RetrieveQAScoreStats` | Aggregated QA score / scorecard stats | High for current QA filters. Uses `QAAttribute`, time range, `scoreResource`, `conversationTimeRangeField`, and `filterToAgentsOnly`. | No direct template dimension | Yes, for the Agent submitted-scorecard aggregate column, but use a separate query with `scorecardStatuses = [MANUALLY_SUBMITTED]` | No | Best semantic match for the Agent leaderboard QA aggregate path. Do not reuse the existing Performance score query for the new column because the default empty `scorecardStatuses` request includes all matching statuses. |
| `RetrieveQAConversations` | Detailed QA-scored conversation / scorecard rows | Medium-high for Agent QA drill-down. With `filterToAgentsOnly` support added, it can match the Agent tab more closely. For Manager parity, it needs future support for filtering by scorecard submitter and scorecard submit time. | Yes, indirectly. FE can group returned scorecards by `scorecardTemplateId`. | Yes, for on-demand Agent drawer template grouping with `scorecardStatuses = [MANUALLY_SUBMITTED]` | Future target for Manager drawer after submitter filtering and submit-time filtering are added | Closest way to derive scorecard detail rows from QA data without a new backend aggregate API. It reads from ClickHouse, which is more consistent with the other Insights APIs and should be faster than the MVP `ListScorecards` path. |
| `RetrieveScorecardStats` | Aggregated completed scorecard stats | High for the current Manager tab because this is what the page already uses | No direct template dimension | No | Yes, for consistency with existing Manager leaderboard behavior | Current Manager `Scorecards completed` metric source. Request is grouped by agent and FE reads `averageScorecardCompletedPerUser`. |
| `ListScorecards` | Raw scorecard list | Low-medium for leaderboard parity. Strong scorecard filters, but not a full match for leaderboard QA-style filters. | Yes, directly from scorecard/template data | Possible, but inconsistent with existing Agent QA path | MVP source for Manager drawer details | Use `submitterUserNames` to align with current `RetrieveScorecardStats` as closely as possible for MVP. Wrap this behind a drawer data abstraction so the implementation can switch to `RetrieveQAConversations` when submitter filtering and submit-time filtering are available. |

## Consistency Recommendation

| Tab | Most consistent API with current page behavior | Why | Main limitation |
|-----|-----------------------------------------------|-----|-----------------|
| Agent | `RetrieveQAScoreStats` with explicit submitted status for the new column | The Agent leaderboard already treats QA score metrics as the source of truth for performance-related data, but the new column is specifically "submitted scorecards". | No template grouping in the response, and the existing score query cannot be reused because empty `scorecardStatuses` means no status filter. |
| Manager | `RetrieveScorecardStats` | The Manager leaderboard already defines `Scorecards completed` with this API. | No template grouping in the response. |

## Why `RetrieveQAScoreStats` Cannot Do Agent x Template

| Constraint | What it means |
|------------|----------------|
| `QAAttributeType` has no scorecard-template group-by value | The API cannot be asked to group results by template alongside agent. |
| `QAScoreGroupBy` has no template field in the response | Even if FE wanted to merge rows by `agent + template`, the response does not expose template in grouped results. |
| Current FE transformer only reads time/team/group/user/criterion/tier/quintile from QA score group-by | There is no hidden template dimension already available in the frontend model. |
| Template is only available as a filter on this API | `RetrieveQAScoreStats` can answer "per-agent counts for one template at a time", but not "all templates broken out per agent in one call". |

### Practical implication

`RetrieveQAScoreStats` can support a per-template breakdown only with a fan-out strategy:

- select one template
- call `RetrieveQAScoreStats` grouped by agent
- repeat for each template
- merge results on FE

That is workable for a small, known template set, but it is not true `agent x template` grouping and scales worse than a single detail or aggregate API.

## Practical Decision Framing

| Requirement | Recommended API | Tradeoff |
|-------------|-----------------|----------|
| Preserve current Agent tab QA metric semantics as closely as possible | `RetrieveQAScoreStats` | Need backend support or a second API to add template grouping. |
| Show Agent "Number of submitted scorecards" in the table | Separate `RetrieveQAScoreStats` query with `scorecardStatuses = [MANUALLY_SUBMITTED]` | Required because the default/empty scorecard status filter includes all statuses. |
| Ship Agent tab template grouping from FE with current APIs | `RetrieveQAConversations` with `scorecardStatuses = [MANUALLY_SUBMITTED]` | Reasonable for the on-demand drawer once `filterToAgentsOnly` is supported. |
| Preserve current Manager tab `Scorecards completed` semantics | `RetrieveScorecardStats` | Cannot group by template. |
| Show manager-reviewed scorecards by template | Not supported cleanly by the traced current APIs | `RetrieveQAConversations` exposes scorecard/template rows, but the traced implementation does not apply `scorecardReviewerAudience`; current Manager aggregate is submitter-attributed completed scorecards. |
| Show manager-completed scorecards by template, matching current aggregate as closely as possible for MVP | `ListScorecards` with `submitterUserNames`, `startSubmitTime`, and `endSubmitTime` | Current `RetrieveScorecardStats` counts submitted scorecards attributed to `submitter_user_id`, not `creator_user_id`. Still needs sample-data validation. |
| Future Manager drawer implementation | `RetrieveQAConversations` after adding scorecard submitter filtering and submit-time filtering | Better long-term source because it reads from ClickHouse, aligns with other Insights APIs, and should be faster than listing scorecards from the scorecard service path. |
| Show manager-created scorecards by template | `ListScorecards` with `creatorUserNames` | Becomes a new metric definition rather than a direct extension of current Manager tab scorecard stats. |

## Recommended Two-API Pattern For Agent Tab

| Interaction | API | Why |
|-------------|-----|-----|
| Main leaderboard table: `Number of submitted scorecards` | Separate `RetrieveQAScoreStats` query with `scorecardStatuses = [MANUALLY_SUBMITTED]` | Keeps the aggregate metric in the QA stats API family while making the submitted-only semantics explicit. |
| Side drawer after clicking `Number of submitted scorecards` for one agent | `RetrieveQAConversations` with `scorecardStatuses = [MANUALLY_SUBMITTED]` | Fetches detailed submitted QA scorecard rows for the selected agent so FE can group them by template only when needed. |

### Why this pattern is reasonable

- The main page only needs submitted aggregate counts by agent.
- The existing Performance score query should not be reused for this column because default/empty `scorecardStatuses` applies no status predicate and includes all matching submitted, draft, and auto scorecards.
- The expensive "all scorecards" fetch only happens on demand when the user opens the drawer.
- This keeps the table aligned with the existing Agent leaderboard semantics while still enabling template breakdown in the drill-down experience.
- If `RetrieveQAConversations` adds `filterToAgentsOnly`, the drawer path can match the Agent tab filter semantics more closely.

## Manager Drill-Down Choice

| Drill-down meaning | Recommended API | Why | Caveat |
|--------------------|-----------------|-----|--------|
| Reviewed by manager | Not supported cleanly by `RetrieveQAConversations` today | The proto has `scorecardReviewerAudience`, but the traced `RetrieveQAConversations` ClickHouse path does not read it. | Would need backend support or a different API path if product wants reviewer semantics. |
| Completed/submitted by manager, matching current aggregate as closely as possible for MVP | `ListScorecards` with `submitterUserNames`, `startSubmitTime`, and `endSubmitTime` | Current aggregate counts distinct scorecards with submit time in range, attributed to `submitter_user_id`. | Needs sample-data validation because the aggregate and list APIs have different filter surfaces and different storage systems. |
| Completed/submitted by manager, future target | `RetrieveQAConversations` after adding submitter filtering and submit-time filtering | Would read from ClickHouse like the other Insights APIs and should be faster. | Requires backend support for filtering scorecards by submitter and submit time before FE can switch. |
| Created by manager | `ListScorecards` with `creatorUserNames` | The API explicitly supports `creatorUserNames` and scorecard/template filtering. | This is a different metric definition relative to the current Manager `Scorecards completed` column. |
| Exact extension of current Manager aggregate | MVP: `ListScorecards` with `submitterUserNames`; future: `RetrieveQAConversations` with submitter filter and submit-time support | Current aggregate comes from `RetrieveScorecardStats`; row-level equivalent is inferred from the backend query. | Needs validation on sample data or backend clarification. |

### Manager data-provider abstraction decision

Build the Manager drawer data fetching behind a small abstraction that returns a normalized shape such as:

- selected manager resource name
- total scorecard count
- template groups
- scorecard rows per template
- loading/error state

The MVP implementation can use `ListScorecards`. The abstraction should not leak `ListScorecards`-specific fields into the drawer UI. When `RetrieveQAConversations` supports filtering by scorecard submitter and submit time, only the data provider should change; the table cell, drawer shell, grouping UI, and tests for the normalized contract should remain stable.

## `RetrieveQAConversations` Scorecard Criteria

| Aspect | Current behavior |
|--------|------------------|
| Underlying table | Uses `score_d` by default, or `scorecard_d` when `ScoreResource = QA_SCORE_RESOURCE_SCORECARD`. |
| Deduping | Chooses the latest `scorecard_last_update_time` per `scorecard_id`. |
| Submitted/completed status | `scorecard_statuses = MANUALLY_SUBMITTED` becomes `scorecard_submit_time <> 0`. |
| Draft status | `scorecard_statuses = DRAFT` becomes `scorecard_submit_time = 0 AND manually_scored = true`. |
| Auto status | `scorecard_statuses = AUTO` becomes `scorecard_submit_time = 0 AND manually_scored = false`. |
| User filter | `QAAttribute.users` filters `agent_user_id`, not creator or submitter. |
| Template filter | `QAAttribute.scorecard_templates` filters `scorecard_template_id`. |
| Reviewer audience | Not applied in the traced `RetrieveQAConversations` ClickHouse implementation, despite the proto field existing. Other QA/manual-QA stats paths do use reviewer audience against `submitter_user_id`. |
| Time basis | Default time range maps to `scorecard_time` on score/scorecard tables, not `scorecard_submit_time`. Conversation-ended time uses a conversation join. |
| Manager parity implication | Cannot produce the same row set as current Manager `Scorecards completed`, which is submit-time-filtered and submitter-attributed. |

## ClickHouse `scorecard_time` Source

`scorecard_time` is not the scorecard submission timestamp.

For conversation scorecards, ClickHouse `scorecard_time` is populated from Postgres `chats.started_at`. The score row builder sets both `conversation_start_time` and `scorecard_time` from `conversation.StartedAt`; the scorecard row builder copies that same score-row conversation start time into the scorecard-level row.

For process scorecards, ClickHouse `scorecard_time` is populated from Postgres `scorecards.process_interaction_at` when present and non-zero. If that field is absent or zero, the builder falls back to the ClickHouse default time sentinel.

Separate ClickHouse fields hold other scorecard timestamps:

- `scorecard_submit_time` comes from Postgres `scorecards.submitted_at`.
- `scorecard_create_time` comes from Postgres `scorecards.created_at`.
- `scorecard_last_update_time` comes from Postgres `scorecards.updated_at`.

Implication for this project: APIs whose default time filter maps to `scorecard_time` are filtering by conversation start time for conversation scorecards, not by scorecard submission time. The current Manager `Scorecards completed` metric avoids that by using `RetrieveScorecardStats`, which rewrites the scorecard table time filter to `scorecard_submit_time`.
