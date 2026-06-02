# API Decision Table

## Leaderboard Scorecard Data Options

| API | Primary entity / grain | Filter parity with current leaderboard usage | Template grouping support | Best fit for Agent tab | Best fit for Manager tab | Notes |
|------|-------------------------|---------------------------------------------|---------------------------|------------------------|--------------------------|-------|
| `RetrieveQAScoreStats` | Aggregated QA score / scorecard stats | High for current QA filters. Uses `QAAttribute`, time range, `scoreResource`, `conversationTimeRangeField`, and `filterToAgentsOnly`. | No direct template dimension | Yes, if consistency with current QA performance logic matters most | No | Best semantic match for the current Agent leaderboard QA score path, but cannot directly answer "count by template per agent". |
| `RetrieveQAConversations` | Detailed QA-scored conversation / scorecard rows | Medium-high for Agent QA drill-down. With `filterToAgentsOnly` support added, it can match the Agent tab more closely. Weak for Manager parity because it does not apply `scorecardReviewerAudience` in the traced ClickHouse path and cannot filter by `creator_user_id` or `submitter_user_id`. | Yes, indirectly. FE can group returned scorecards by `scorecardTemplateId`. | Yes, for on-demand Agent drawer template grouping | No, if the drawer must align with current `Scorecards completed` | Closest way to derive agent x template counts from QA data without a new backend aggregate API. Not the right source for Manager completed-scorecard drawer parity. |
| `RetrieveScorecardStats` | Aggregated completed scorecard stats | High for the current Manager tab because this is what the page already uses | No direct template dimension | No | Yes, for consistency with existing Manager leaderboard behavior | Current Manager `Scorecards completed` metric source. Request is grouped by agent and FE reads `averageScorecardCompletedPerUser`. |
| `ListScorecards` | Raw scorecard list | Low-medium for leaderboard parity. Strong scorecard filters, but not a full match for leaderboard QA-style filters. | Yes, directly from scorecard/template data | Possible, but inconsistent with existing Agent QA path | Possible for Manager drawer details | Best raw source for Manager drawer rows. Use `creatorUserNames` to align with current `RetrieveScorecardStats` as closely as possible; use `submitterUserNames` only for explicit submitter semantics. |

## Consistency Recommendation

| Tab | Most consistent API with current page behavior | Why | Main limitation |
|-----|-----------------------------------------------|-----|-----------------|
| Agent | `RetrieveQAScoreStats` | The Agent leaderboard already treats QA score metrics as the source of truth for performance-related data. | No template grouping in the response. |
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
| Ship Agent tab template grouping from FE with current APIs | `RetrieveQAConversations` | Reasonable for the on-demand drawer once `filterToAgentsOnly` is supported. |
| Preserve current Manager tab `Scorecards completed` semantics | `RetrieveScorecardStats` | Cannot group by template. |
| Show manager-reviewed scorecards by template | Not supported cleanly by the traced current APIs | `RetrieveQAConversations` exposes scorecard/template rows, but the traced implementation does not apply `scorecardReviewerAudience`; current Manager aggregate is creator-attributed completed scorecards, not reviewer-attributed scorecards. |
| Show manager-completed scorecards by template, matching current aggregate as closely as possible | `ListScorecards` with `creatorUserNames`, `startSubmitTime`, and `endSubmitTime` | Current `RetrieveScorecardStats` counts submitted scorecards attributed to `creator_user_id`, not `submitter_user_id`. Still needs sample-data validation. |
| Show manager-submitted scorecards by template | `ListScorecards` with `submitterUserNames` | Becomes a new metric definition rather than a direct extension of current Manager tab scorecard stats. |

## Recommended Two-API Pattern For Agent Tab

| Interaction | API | Why |
|-------------|-----|-----|
| Main leaderboard table | `RetrieveQAScoreStats` | Keeps the aggregate agent metric consistent with the current Agent tab QA score path. |
| Side drawer after clicking `# of scorecards` for one agent | `RetrieveQAConversations` | Fetches detailed QA scorecard rows for the selected agent so FE can group them by template only when needed. |

### Why this pattern is reasonable

- The main page only needs aggregate counts by agent.
- The expensive "all scorecards" fetch only happens on demand when the user opens the drawer.
- This keeps the table aligned with the existing Agent leaderboard semantics while still enabling template breakdown in the drill-down experience.
- If `RetrieveQAConversations` adds `filterToAgentsOnly`, the drawer path can match the Agent tab filter semantics more closely.

## Manager Drill-Down Choice

| Drill-down meaning | Recommended API | Why | Caveat |
|--------------------|-----------------|-----|--------|
| Reviewed by manager | Not supported cleanly by `RetrieveQAConversations` today | The proto has `scorecardReviewerAudience`, but the traced `RetrieveQAConversations` ClickHouse path does not read it. | Would need backend support or a different API path if product wants reviewer semantics. |
| Completed by manager, matching current aggregate as closely as possible | `ListScorecards` with `creatorUserNames`, `startSubmitTime`, and `endSubmitTime` | Current aggregate counts distinct scorecards with submit time in range, attributed to `creator_user_id`. | Needs sample-data validation because the aggregate and list APIs have different filter surfaces and different storage systems. |
| Submitted by manager | `ListScorecards` with `submitterUserNames` | The API explicitly supports `submitterUserNames` and scorecard/template filtering. | This is a new metric definition relative to the current Manager `Scorecards completed` column. |
| Exact extension of current Manager aggregate | `ListScorecards` with `creatorUserNames` is the closest current candidate | Current aggregate comes from `RetrieveScorecardStats`; row-level equivalent is inferred from the backend query. | Needs validation on sample data or backend clarification. |

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
| Manager parity implication | Cannot produce the same row set as current Manager `Scorecards completed`, which is submit-time-filtered and creator-attributed. |
