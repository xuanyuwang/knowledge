# Proto investigation: QA request time-basis fields (CONVI-7162)

Date: 2026-07-28
Repo: `/Users/xuanyu.wang/repos/cresta-proto`

## Findings

1. **Request messages live in** `cresta/v1/analytics/analytics_service.proto`, not `qa_stats.proto`.
   - `RetrieveQAScoreStatsRequest` @ L3263–3328
   - `RetrieveQAConversationsRequest` @ L3337–3389
2. **`ScorecardTimeBasis` does not exist** anywhere in cresta-proto.
3. **`conversation_time_range_field` field numbers:**
   - QA Score Stats: field **11**
   - QA Conversations: field **9**
   - Also on `RetrieveConversationStatsRequest` (9) and `SuggestManualQAConversationsRequest` (10)
4. **`TargetFieldForTimeRange`** nested enum in `RetrieveClosedConversationsRequest` @ L4005–4013:
   - UNSPECIFIED=0, CONVERSATION_STARTED_AT=1, CONVERSATION_ENDED_AT=2
5. Similar time-field enums elsewhere: `job_filter.TimeFilterType`, `SharePointTimeFilterField`, `ConversationTimeScope` (different semantics).
