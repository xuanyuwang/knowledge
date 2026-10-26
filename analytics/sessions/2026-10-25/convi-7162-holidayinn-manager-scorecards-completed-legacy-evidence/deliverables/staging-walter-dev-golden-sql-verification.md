# Staging verification: updated QA submit-time golden SQL

**Date:** 2026-08-04
**Target:** customer `cresta`, profile `walter-dev`, cluster `voice-staging`
**Database:** `cresta_walter_dev`
**Host:** `clickhouse-conversations.voice-staging.internal.cresta.ai`
**PR:** https://github.com/cresta/go-servers/pull/30635

## Adaptations from goldens

- Date window: `2026-07-01 00:00:00` → `2026-08-01 00:00:00` (has submit-time rows in staging)
- Removed unit-test-only `scorecard_template_id IN ('Test…')` filters
- Conversations query: `LIMIT 20 OFFSET 0` (golden used `OFFSET 50`)
- Otherwise kept query shape (CTEs, `scorecard_submit_time` projection, `DATE_TRUNC`)

## Results

| Adapted file | Golden source | Exit | Notes |
|---|---|---:|---|
| `01_GroupByAgentTime_ScorecardSubmitTime.sql` | score_d + daily submit | 0 | Returned agent/day rows |
| `02_GroupByAgentTime_SourceScorecard_ScorecardSubmitTime.sql` | **Manager path**: scorecard_d + daily submit | 0 | `DATE_TRUNC` on projected `scorecard_submit_time` works |
| `03_GroupByAgent_SourceScorecard_ScorecardSubmitTime.sql` | scorecard_d + submit filter, no time group | 0 | OK |
| `04_GroupByAgent_SourceScorecard.sql` | scorecard_d + interaction time (projection includes submit col) | 0 | OK (broader row set via `scorecard_time`) |
| `05_RetrieveQaConversations_ScorecardSubmitTime.sql` | conversations submit filter | 0 | Returned rows; some process/empty conversation ids show epoch start times (data quirk, not SQL failure) |

All five queries executed successfully on staging. The critical regression covered by #02 confirms the scorecard-table projection fix for Manager daily Scorecards Completed.
