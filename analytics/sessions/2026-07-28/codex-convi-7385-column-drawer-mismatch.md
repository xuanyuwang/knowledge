# Codex Investigation - CONVI-7385 Column vs Drawer Scorecard Count

**Date:** 2026-07-28
**Ticket:** CONVI-7385
**Source repo:** `/Users/xuanyu.wang/repos/director`
**Related backend:** `/Users/xuanyu.wang/repos/go-servers`
**Domain:** analytics / leaderboard

## Symptom

Agent Leaderboard for CNG `us-east-1`, agent Agnes Long (`2bdb21d567b2118f`), day 2026-07-15 America/Toronto:

- Column count = 1 (`RetrieveQAScoreStats.totalScorecardCount`)
- Drawer count = 3 (`RetrieveQAConversations.conversationInfos`)

## Request Diff

Shared:

- parent `customers/cng/profiles/us-east-1`
- same time range
- same agent user
- same usecase `retail-store-voice`
- `scoreResource = QA_SCORE_RESOURCE_SCORECARD`
- empty `scorecardStatuses`

Material difference:

- Stats request includes:

```json
"momentGroups":[{
  "moments":[],
  "excludedMoments":[{
    "name":"customers/cng/profiles/us-east-1/moments/019bae74-cd81-766d-8b25-0a507e4f8393",
    "taxonomy":"voice_mail",
    "type":"VOICE_MAIL"
  }]
}]
```

- Conversations request has `"momentGroups":[]`.

## Frontend Source

Agent drawer rebuilds filters and clears voicemail:

`packages/director-app/src/features/insights/leaderboard/scorecard-template-breakdown-drawer/useAgentScorecardTemplateBreakdown.ts`

```ts
voicemailMoment: undefined,
```

Agent column uses page `qaScoreFiltersState` via `useGetQAStats` → `useQAScoreStats`, which keeps `voicemailMoment` and converts it through `useMomentGroupFilterFromFilterState` into an excluded VOICE_MAIL moment group.

Manager drawer has the same clear:

`useManagerScorecardTemplateBreakdown.ts` also sets `voicemailMoment: undefined`.

## Backend Behavior

In `go-servers/insights-server/internal/analyticsimpl/common_clickhouse.go`:

- `excludeVoiceMail(momentGroups, ...)` detects excluded moments of type `VOICE_MAIL`
- `parseConversationConditionsForQAAttribute` adds `is_voice_mail <> 1`
- That causes a join/filter against `conversation_d`

So once the FE omits the voicemail exclusion, the conversations API correctly returns more rows.

## ClickHouse Validation (`us-east-1-prod`, db `cng_us_east_1`)

Conversation voicemail flags:

| conversation_id | is_voice_mail |
|---|---|
| 019f6687-16cd-781d-bc2f-5d82694bfb51 | true |
| 019f6735-3c15-7add-ae07-7352132f686a | false |
| 019f67ad-7483-77e0-889d-94634ece9d3a | true |

All three scorecards exist for agent `2bdb21d567b2118f` with `score = 100` and no submit time (`scorecard_submit_time = 1970-01-01`, `manually_scored = false`), matching AI/auto scorecards.

With voicemail excluded: 1 scorecard remains. Without exclusion: 3 scorecards.

## Conclusion

APIs are consistent with their request filters. The mismatch is caused by the drawer FE dropping the page voicemail exclusion.

Recommended fix: preserve `voicemailMoment` (and any other shared page filters) when constructing Agent/Manager scorecard drawer requests; add a regression test on request params.
