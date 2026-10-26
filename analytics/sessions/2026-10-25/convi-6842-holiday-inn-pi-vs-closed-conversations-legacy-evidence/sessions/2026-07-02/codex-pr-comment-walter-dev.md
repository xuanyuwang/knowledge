# CONVI-6842 PR Comment: Walter Dev Before/After

Source repo: `/Users/xuanyu.wang/repos/director-convi-6842`  
Branch: `xwang/convi-6842-conversation-volume`  
PR comment: https://github.com/cresta/director/pull/20153#issuecomment-4843907620

## Comment

Reviewer attached two screenshots from `voice-staging`, `cresta/walter-dev`, asking whether the large before/after discrepancy is normal.

The screenshots show the same visible filters:

- `Template: All`
- `This month | Weekly`
- `All minutes`
- `Exclude voicemail: Yes`
- `Calculation: Criteria adherence`

The displayed conversation-volume values differ:

- `22,574` in one screenshot
- `8,803` in the other screenshot

## Verification

The current PR code makes the no-template `ConversationCountChart` request `RetrieveQAScoreStats` through `useQAScoreStatsRequestParams(filtersState, [QA_ATTRIBUTE_TYPE_TIME_RANGE])` and `useQAScoreStats`.

`PerformanceConversations` passes `filtersWithNAValues` into `ConversationCountChart`, setting `includeNaScored: true`.

The old no-template path used `RetrieveConversationStats`, which is built from `message_d` joined to `conversation_d`. The new path uses QA score stats, which counts distinct `conversation_id` from score/scorecard data and can include N/A scorecards. These are intentionally different populations.

Backend query semantics checked in `go-servers`:

- `RetrieveQAScoreStats` computes `COUNT(DISTINCT conversation_id)` from score/scorecard CTEs.
- `RetrieveConversationStats` computes distinct conversations from `message_d` with non-empty `agent_user_id` and joins to conversation data.
- The QA stats response total is the sum of the grouped rows, so a weekly chart headline is the sum of weekly distinct counts.

## Conclusion

The discrepancy makes sense for this PR and does not by itself indicate a code bug. It is the expected consequence of changing the no-template card from strict conversation/message stats to scorecard-backed QA stats with `includeNaScored: true`. On `walter-dev`, the scorecard-backed population appears much larger than the message-backed conversation population.

If stakeholders expect the no-template number to remain close to Closed Conversations or the previous `RetrieveConversationStats` count, that is a product-definition disagreement rather than an implementation mismatch.
