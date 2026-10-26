## Example composed SQL — all-4 CLO groups, 7d window

Window: `[2026-07-25 04:00:00, 2026-08-01 03:59:59)`. Same scorecard/score body for both paths (`usecase_id IN ('walter-dev')`, Toronto day trunc). Only CLO CTEs differ.

### Flag OFF — CLO CTE prefix (raw)

```sql
WITH

t1_moment_annotation_filter_0 AS (
  SELECT conversation_id, MAX(create_time) AS latest_update_time
  FROM moment_annotation_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_type = 14) AND (moment_template_id = '01964a99-d3ce-7b0d-823f-b7dbb50195c2'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_0 AS (
  SELECT conversation_id, create_time
  FROM moment_annotation_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_type = 14) AND (moment_template_id = '01964a99-d3ce-7b0d-823f-b7dbb50195c2') AND ((JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') = true AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') >= -1 AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') <= -1) OR (JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') = true AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') >= 0 AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') <= 6) OR (JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') = true AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') > 6 AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') <= 10)))
),
moment_annotation_filter_0 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_0
  FROM t1_moment_annotation_filter_0 AS t1
  JOIN t2_moment_annotation_filter_0 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.create_time
),

t1_moment_annotation_filter_1 AS (
  SELECT conversation_id, MAX(create_time) AS latest_update_time
  FROM moment_annotation_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_type = 14) AND (moment_template_id = '0199e72b-551b-74ca-b5de-5c1d9491a983'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_1 AS (
  SELECT conversation_id, create_time
  FROM moment_annotation_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_type = 14) AND (moment_template_id = '0199e72b-551b-74ca-b5de-5c1d9491a983') AND ((JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') = true AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') >= -1 AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') <= -1) OR (JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') = true AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') >= 0 AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') <= 6) OR (JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') = true AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') > 6 AND JSONExtractFloat(moment_annotation_payload, 'conversation_outcome_payload', 'number_value') <= 10)))
),
moment_annotation_filter_1 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_1
  FROM t1_moment_annotation_filter_1 AS t1
  JOIN t2_moment_annotation_filter_1 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.create_time
),

t1_moment_annotation_filter_2 AS (
  SELECT conversation_id, MAX(create_time) AS latest_update_time
  FROM moment_annotation_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_type = 14) AND (moment_template_id = '7252ec08-0620-4c83-93aa-6ccc84b6750b'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_2 AS (
  SELECT conversation_id, create_time
  FROM moment_annotation_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_type = 14) AND (moment_template_id = '7252ec08-0620-4c83-93aa-6ccc84b6750b') AND ((JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = true AND JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = 'resolved') OR (JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = true AND JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = 'unresolved') OR (JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = true AND JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = 'not_care')))
),
moment_annotation_filter_2 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_2
  FROM t1_moment_annotation_filter_2 AS t1
  JOIN t2_moment_annotation_filter_2 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.create_time
),

t1_moment_annotation_filter_3 AS (
  SELECT conversation_id, MAX(create_time) AS latest_update_time
  FROM moment_annotation_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_type = 14) AND (moment_template_id = '921efbb7-7797-4dcb-874e-16103e1b5102'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_3 AS (
  SELECT conversation_id, create_time
  FROM moment_annotation_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_type = 14) AND (moment_template_id = '921efbb7-7797-4dcb-874e-16103e1b5102') AND ((JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value') = true AND JSONExtractBool(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value') = true) OR (JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value') = true AND JSONExtractBool(moment_annotation_payload, 'conversation_outcome_payload', 'boolean_value') = false)))
),
moment_annotation_filter_3 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_3
  FROM t1_moment_annotation_filter_3 AS t1
  JOIN t2_moment_annotation_filter_3 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.create_time
),
-- ... shared scorecard/score body + GROUP BY truncated_time ...

```

### Flag ON — CLO CTE prefix (MV)

```sql
WITH

t1_moment_annotation_filter_0 AS (
  SELECT conversation_id, MAX(update_time) AS latest_update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '01964a99-d3ce-7b0d-823f-b7dbb50195c2'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_0 AS (
  SELECT conversation_id, update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '01964a99-d3ce-7b0d-823f-b7dbb50195c2') AND ((outcome_value_type = 'number' AND outcome_number_value >= -1 AND outcome_number_value <= -1) OR (outcome_value_type = 'number' AND outcome_number_value >= 0 AND outcome_number_value <= 6) OR (outcome_value_type = 'number' AND outcome_number_value > 6 AND outcome_number_value <= 10)))
),
moment_annotation_filter_0 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_0
  FROM t1_moment_annotation_filter_0 AS t1
  JOIN t2_moment_annotation_filter_0 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
),

t1_moment_annotation_filter_1 AS (
  SELECT conversation_id, MAX(update_time) AS latest_update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '0199e72b-551b-74ca-b5de-5c1d9491a983'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_1 AS (
  SELECT conversation_id, update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '0199e72b-551b-74ca-b5de-5c1d9491a983') AND ((outcome_value_type = 'number' AND outcome_number_value >= -1 AND outcome_number_value <= -1) OR (outcome_value_type = 'number' AND outcome_number_value >= 0 AND outcome_number_value <= 6) OR (outcome_value_type = 'number' AND outcome_number_value > 6 AND outcome_number_value <= 10)))
),
moment_annotation_filter_1 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_1
  FROM t1_moment_annotation_filter_1 AS t1
  JOIN t2_moment_annotation_filter_1 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
),

t1_moment_annotation_filter_2 AS (
  SELECT conversation_id, MAX(update_time) AS latest_update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '7252ec08-0620-4c83-93aa-6ccc84b6750b'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_2 AS (
  SELECT conversation_id, update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '7252ec08-0620-4c83-93aa-6ccc84b6750b') AND ((outcome_value_type = 'string' AND outcome_string_value = 'resolved') OR (outcome_value_type = 'string' AND outcome_string_value = 'unresolved') OR (outcome_value_type = 'string' AND outcome_string_value = 'not_care')))
),
moment_annotation_filter_2 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_2
  FROM t1_moment_annotation_filter_2 AS t1
  JOIN t2_moment_annotation_filter_2 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
),

t1_moment_annotation_filter_3 AS (
  SELECT conversation_id, MAX(update_time) AS latest_update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '921efbb7-7797-4dcb-874e-16103e1b5102'))
  GROUP BY conversation_id
),
t2_moment_annotation_filter_3 AS (
  SELECT conversation_id, update_time
  FROM moment_annotation_by_conversation_outcome_d
  WHERE ((conversation_start_time >= '2026-07-25 04:00:00') AND (conversation_start_time < '2026-08-01 03:59:59') AND (moment_template_id = '921efbb7-7797-4dcb-874e-16103e1b5102') AND ((outcome_value_type = 'boolean' AND outcome_bool_value = true) OR (outcome_value_type = 'boolean' AND outcome_bool_value = false)))
),
moment_annotation_filter_3 AS (
  SELECT DISTINCT t1.conversation_id AS conversation_id_3
  FROM t1_moment_annotation_filter_3 AS t1
  JOIN t2_moment_annotation_filter_3 AS t2
    ON t1.conversation_id = t2.conversation_id AND t1.latest_update_time = t2.update_time
),
-- ... shared scorecard/score body + GROUP BY truncated_time ...

```

Full generated SQL artifacts (all combos/windows): `/tmp/clo-mv-pressure/full/` and `/tmp/clo-mv-pressure/filter_diff/`.
Results CSVs: `full_results.csv`, `filter_diff_results_reduced.csv`.
