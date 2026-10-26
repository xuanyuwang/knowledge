# Voice-staging golden SQL verification (cresta / walter-dev)

## Target
- Config: `configv3/staging/cresta/walter-dev/config.yaml`
- Cluster: `voice-staging`
- Database: `cresta_walter_dev`
- Host: `clickhouse-conversations.voice-staging.internal.cresta.ai`

## Schema
CLO objects present: `moment_annotation_by_conversation_outcome`, `_d`, and `moment_annotation_mv_by_conversation_outcome`. Columns: typed `outcome_*` + `create_time`/`update_time`. No `conversation_end_time` on CLO storage.

## Workload
- Template: `7252ec08-0620-4c83-93aa-6ccc84b6750b` (string CLO)
- Window: `[2026-07-08, 2026-08-07)` UTC
- Value for full golden: `unresolved` (has score overlap)

## Results
1. Filter CTE string exact (`resolved`, 7d): raw vs MV conversation sets **1217 / 1217**, `raw_only=0`, `mv_only=0`.
2. Full golden QA score-stats shape (`unresolved`, 30d): raw vs MV aggregates match (`result_rows=74`, `sum_conversation_counts=568`, `sum_scorecard_counts=21128`, `sum_weight=24364`; weighted sum equal within float epsilon).
3. JSONHas numeric-bin regression on this string template (30d): all `36032` annotations lack `number_value`; unguarded `JSONExtractFloat` in `[0,10]` would false-match all `36032`. Guarded path matches `0`.
