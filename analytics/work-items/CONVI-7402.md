# CONVI-7402: Bswift QA score stats errors on Team + (no value)

**Ticket:** [CONVI-7402](https://linear.app/cresta/issue/CONVI-7402/bswift-users-getting-qa-score-stats-errors)
**Zendesk:** [#22883](https://crestasupport.zendesk.com/agent/tickets/22883)
**Slack:** [thread](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785449772055899)
**Customer:** Bswift (`bswift` / `us-east-1`)
**Template:** Automated Heartbeat Quality Scorecard (`0199abb9-fd82-7696-9302-54a27b5e33b6`)
**Status:** Fix implemented; ClickHouse QA OR parity validated
**Last validated:** 2026-08-19

## Verdict

HTTP 400 on `RetrieveQAScoreStats` (and parity gap on QA conversations) is fixed by porting Elasticsearch overlap OR semantics into ClickHouse QA.

When a metadata `MomentGroup` includes the same moment template id in both `moments` (concrete Team values) and `excluded_moments` (`(no value)`), the backend now treats it as:

- `(Team ∈ selected values)` OR `(Team moment template is absent (no annotation for that template id))`

Independent moment groups and unrelated exclusions continue to be AND-ed exactly as before.

Secondary: bswift scorecard PG→CH sync gap can still cause empty/stale PI data after the 400 is fixed — track separately from this validation error.

## Implemented fix

- `parseMomentConditionsForQAAttribute` (in `insights-server/internal/analyticsimpl/common_clickhouse.go`) no longer blanket-rejects include+exclude overlap for the same metadata moment template id. Instead it returns an `overlapMomentFilters` pair (valueFilter + missingFilter) representing the overlap as OR semantics. This behavior remains gated by `CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS`.
- `qaScoreStatsClickhouseQueryWithMetadataView` and `qaConversationsClickhouseQuery` (in the corresponding ClickHouse query builder files) generate the overlap as `LEFT JOIN` + `WHERE (valueExists OR missingBranchIsTrue)`, while leaving excluded-only moment groups as their prior exclusion behavior.

Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7402` (branch `convi-7402-bswift-users-getting-qa-score-stats-errors`)

## Validation

- Added parser unit test: `TestParseMomentConditionsForQAAttribute_MetadataMomentOverlap`.
- Added query-level SQL golden regressions + unit SQL coverage:
  - `testdata/clickhouse_RetrieveQAScoreStats_FilterByMetadataMomentGroups_OverlapValuePlusNoValue_request.sql`
  - `testdata/clickhouse_RetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue_request.sql`
- Ran `gofmt` on touched files and `bazel run //:gazelle`.
- Note: embedded-postgres-based full suite could not be executed here due to an `initdb` failure (`could not create shared memory segment: No space left on device`); this does not affect compilation or the SQL-builder/unit coverage.

## Key pointers

- Frontend: `director/.../useMomentGroupFilterFromFilterState.ts`
- Backend parsing: `go-servers/.../common_clickhouse.go` (`parseMomentConditionsForQAAttribute`)
- Backend ClickHouse query builders:
  - `retrieve_qa_score_stats_clickhouse.go`
  - `retrieve_qa_conversations_clickhouse.go`
- ES reference: `elasticsearch/request.go` `convertConvoMomentGroupToConvoFilters`
- Rollout of PI combined behavior: director [INSI-2621](https://github.com/cresta/director/pull/17329)
