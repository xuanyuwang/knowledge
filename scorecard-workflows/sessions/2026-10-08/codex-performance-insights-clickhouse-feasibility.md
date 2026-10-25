# Performance Insights: criterion/use-case query feasibility

- Date: 2026-10-08
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only; current `origin/main` differences checked)
- Related source repos: `/Users/xuanyu.wang/repos/director` and `/Users/xuanyu.wang/repos/clickhouse-schema` (`main`, read-only)
- Scope: first pass on the Performance Insights conversations page's primary stats APIs and staging ClickHouse `EXPLAIN`; no production queries or mutations.

## Frontend API trace

`director/packages/director-app/src/features/insights/performance/Performance.tsx` routes the conversations tab to `PerformanceConversations`. That page renders StatsGraphContainer, ConversationCountChart, PerformanceProgression, LeaderboardByScorecardTemplateItem, and LeaderboardPerCriterion. Their stats hooks use two main APIs: `RetrieveQAScoreStats` (scores, criterion progression, leaderboards and much of volume) and `RetrieveConversationStats` (handle-time views). The conversation examples drawer also uses `RetrieveQAConversations`, outside this first performance comparison.

`getQAFilterByAttribute.ts` currently sends at most one selected use-case name, while `QAAttribute.usecase_names` already accepts a repeated list. It may send an explicit list of criterion IDs from the selected template; some requests leave that list absent or use scorecard-level scoring. We need to compare representative request shapes, not presume every call filters criteria.

## ClickHouse and API feasibility

The `clickhouse-schema` conversations schema includes `usecase_id` on `score`, `scorecard`, `conversation`, and `message`, and `criterion_id` on `score`. The corresponding distributed `score_d`, `scorecard_d`, `conversation_d`, and `message_d` tables are what the two APIs read. The local `score` sort key begins `(toStartOfHour(scorecard_time), scorecard_template_id, criterion_id, agent_user_id, ...)`; `usecase_id` is not a sort-key column. The local `scorecard` sort key begins `(toStartOfHour(scorecard_time), scorecard_template_id, agent_user_id, ...)`; `usecase_id` is not a sort-key column there either.

`RetrieveQAScoreStats` already parses `QAAttribute.usecase_names` into `usecase_id IN (...)` and, for criterion-score requests, `criterion_identifiers` into `criterion_id IN (...)`. This supports selecting multiple use cases and criteria without a schema change. The score-stats SQL filters the scorecard stream by use case and the score stream by criterion, then joins by scorecard ID and latest-update time. New evaluations should omit score rows for inapplicable criteria, so no current-scope pair predicate is needed for those rows. Such a predicate could incorrectly discard historical scores from scorecards pinned to an older template revision; its reporting semantics must be decided before use.

`RetrieveConversationStats` parses repeated use-case names for conversation/message filters. It has no criterion-level score definition in its source tables. Its handle-time metrics can follow the selected use cases, but a criterion filter would require a separate score-based population or join and a clarified denominator; it should not be added merely because the page has a criterion picker.

`QAAttributeType` currently has no use-case group-by value, and `parseClickhouseGroupByForQAAttribute` has no `usecase_id` branch. Per-use-case rows in Performance Insights' second and third tables therefore require an API/backend grouping extension or separate requests per selected use case. This is distinct from the already-supported multi-use-case WHERE filter.

## Staging comparison

Target requested by Xuanyu: `cresta/walter-dev` on `voice-staging`, conversations ClickHouse service. Latest config path `origin/master:configv3/staging/cresta/walter-dev/config.yaml` resolves to cluster `voice-staging`, host `clickhouse-conversations.voice-staging.internal.cresta.ai`, database `cresta_walter_dev`. Server version was `26.3.12.3`. The selected AWS SSO profile `voice-staging_dev` was manually cleared under the workspace security policy and used through the `connect-customer-clickhouse` skill. No password or credential URI was recorded.

The actual row filters are `customer_id='cresta'`, `profile_id='walter-dev'`. For `2026-10-01` through `2026-10-08`, staging has a populated `walter-dev` template (`019626be-e502-75ae-9d3f-ee414b4db792`, criterion `019eacb9-3141-7330-8b80-cc8c301a0d5a`) and a populated `chat` template (`019e1047-8d4a-758a-aafa-f5fe974d5d9a`, criterion `019e1047-8d49-70c1-98d9-a174e2b30851`). A 30-day grouping found no template ID under more than one use case. The two-template sample approximates a broader selected population, **not** an actual shared template. Raw `scorecard_d` counts include ReplacingMergeTree versions and are not unique scorecard counts.

Read-only `EXPLAIN ESTIMATE` was run on the **local** tables. The corresponding distributed `_d` tables returned no estimate rows, although production API queries read those distributed tables. Each comparison held the time window, customer/profile, two template IDs, and two criterion IDs constant; only the use-case filter changed from `('walter-dev')` to `('walter-dev','chat')`. The score breakdown variant also grouped on the scorecard's latest use case.

| Representative query | One use case: local estimated reads | Two use cases: local estimated reads |
| --- | --- | --- |
| Latest scorecard joined to distinct criterion scores | `scorecard`: 3 parts, 139,252 rows, 19 marks; `score`: 2 parts, 271,924 rows, 37 marks | Same estimates, including use-case grouping |
| Latest conversations joined to message handle-time aggregate | `conversation`: 2 parts, 9,567 rows, 2 marks; `message`: 1 part, 26,617 rows, 4 marks | Same estimates |

On a direct `score` scan with the same two templates and criteria, `EXPLAIN ESTIMATE` showed 3 parts, 262,252 rows, 36 marks for both one and two use cases. Replacing the independent use-case/criterion filters with two allowed `(usecase_id, criterion_id)` pairs produced the same estimate; this was a query feasibility check, not a recommendation to filter out historical scores. `EXPLAIN indexes=1` for a representative score scan showed the primary key pruning on hour, template ID, and criterion ID (3/7 parts and 36/2,480 granules); `usecase_id` was not part of the reported primary-key condition. This explains why broadening the use-case predicate did not change these local scan estimates. It does not show that extra returned rows, joins, grouping, or network work are free.

These queries are representative simplifications, not captured production SQL. The APIs' actual builders have additional status, audience, metadata, and date-target branches. `EXPLAIN ESTIMATE` is a storage-read estimate, not execution time or end-to-end API latency. This sample contains no cross-use-case template, and local-table estimates do not model distributed fanout. The next performance gate is to capture actual generated SQL and request sizes, then compare `EXPLAIN` and timed runs against a shared-template fixture or representative population. The first pass is enough to establish schema and query-shape feasibility, not a latency guarantee.

### Draft query shapes for the comparison

Use actual staging IDs and a populated date window after connection. The score query below preserves the APIs' important shape: latest scorecard version, DISTINCT score rows, then weighted criterion aggregation. The exact production builder has additional status, audience, metadata, and date-target branches, so this is a representative baseline rather than a captured API query.

```sql
EXPLAIN indexes = 1
WITH
  latest_scorecards AS (
    SELECT scorecard_id,
           max(scorecard_last_update_time) AS last_update,
           argMax(usecase_id, scorecard_last_update_time) AS group_usecase_id
    FROM scorecard_d
    WHERE customer_id = '<customer>' AND profile_id = '<profile>'
      AND scorecard_time >= '<start>' AND scorecard_time < '<end>'
      AND scorecard_template_id = '<template>'
      AND usecase_id IN ('<A>')
    GROUP BY scorecard_id
  ),
  score_rows AS (
    SELECT DISTINCT scorecard_id, scorecard_last_update_time,
           criterion_id, conversation_id, percentage_value, float_weight
    FROM score_d
    WHERE customer_id = '<customer>' AND profile_id = '<profile>'
      AND scorecard_time >= '<start>' AND scorecard_time < '<end>'
      AND scorecard_template_id = '<template>'
      AND criterion_id IN ('<criterion-1>', '<criterion-2>')
  )
SELECT score_rows.criterion_id,
       sumIf(percentage_value * float_weight, percentage_value >= 0) AS weighted_sum,
       sumIf(float_weight, percentage_value >= 0) AS weight_sum,
       uniqExact(score_rows.scorecard_id) AS scorecard_count
FROM score_rows INNER JOIN latest_scorecards
  ON score_rows.scorecard_id = latest_scorecards.scorecard_id
 AND score_rows.scorecard_last_update_time = latest_scorecards.last_update
GROUP BY score_rows.criterion_id;
```

For the multi-use-case filter, change `IN ('<A>')` to `IN ('<A>', '<B>')`. For a breakdown candidate, additionally select and group by `latest_scorecards.group_usecase_id`. The distinct alias avoids ClickHouse resolving a same-name select alias inside the CTE's `WHERE`. For an explicit applicability-pair candidate, add `AND (usecase_id, criterion_id) IN (('<A>','<criterion-1>'), ('<B>','<criterion-2>'))` to `score_rows`; use this only if the intended historical-scope semantics require it. Each is compared to the same one-use-case baseline.

The conversation-stats comparison uses the existing source shape: one latest conversation row plus a per-conversation/agent handle-time aggregate from `message_d`. Apply the same A versus A+B change to both `conversation_d.usecase_id` and `message_d.usecase_id` predicates. The local `EXPLAIN ESTIMATE` reads stayed at 2 conversation and 4 message marks for both filters; no criterion predicate belongs in this base query.

```sql
EXPLAIN indexes = 1
WITH
  conversations AS (
    SELECT conversation_id, agent_user_id
    FROM conversation_d
    WHERE customer_id = '<customer>' AND profile_id = '<profile>'
      AND conversation_start_time >= '<start>'
      AND conversation_start_time < '<end>'
      AND usecase_id IN ('<A>')
      AND is_dev_user = 0 AND agent_user_id != ''
    ORDER BY update_time DESC
    LIMIT 1 BY agent_user_id, conversation_id
  ),
  handle_time AS (
    SELECT conversation_id, agent_user_id,
           max(handle_time_secs) AS aht_sec
    FROM message_d
    WHERE customer_id = '<customer>' AND profile_id = '<profile>'
      AND conversation_start_time >= '<start>'
      AND conversation_start_time < '<end>'
      AND usecase_id IN ('<A>')
      AND is_dev_user = 0 AND agent_user_id != ''
    GROUP BY conversation_id, agent_user_id
  )
SELECT uniqExact(conversations.conversation_id) AS conversation_count,
       sum(handle_time.aht_sec) AS total_handle_time
FROM conversations INNER JOIN handle_time
  USING (conversation_id, agent_user_id);
```
