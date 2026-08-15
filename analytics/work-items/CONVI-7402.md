# CONVI-7402: Bswift QA score stats errors on Team + (no value)

**Ticket:** [CONVI-7402](https://linear.app/cresta/issue/CONVI-7402/bswift-users-getting-qa-score-stats-errors)
**Zendesk:** [#22883](https://crestasupport.zendesk.com/agent/tickets/22883)
**Slack:** [thread](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1785449772055899)
**Customer:** Bswift (`bswift` / `us-east-1`)
**Template:** Automated Heartbeat Quality Scorecard (`0199abb9-fd82-7696-9302-54a27b5e33b6`)
**Status:** Investigation complete; fix not implemented
**Last validated:** 2026-08-14

## Verdict

HTTP 400 on `RetrieveQAScoreStats` is caused by a **frontend/backend contract mismatch**: PI sends a metadata `MomentGroup` with both `moments` and `excluded_moments` when the user selects a concrete Team value **and** `(no value)`. ClickHouse QA APIs reject that shape; Elasticsearch (Closed Conversations) already implements OR semantics for it.

Secondary: bswift scorecard PG→CH sync gap can still cause empty/stale PI data after the 400 is fixed — track separately from this validation error.

## Proposed fix

Port ES OR semantics for overlapping include+exclude metadata moment groups into ClickHouse QA paths (`parseMomentConditionsForQAAttribute` + QA score/conversations query builders). Do not forbid the UI multi-select.

## Key pointers

- Frontend: `director/.../useMomentGroupFilterFromFilterState.ts`
- Validation: `go-servers/.../common_clickhouse.go`
- ES reference: `elasticsearch/request.go` `convertConvoMomentGroupToConvoFilters`
- Rollout of PI combined behavior: director [INSI-2621](https://github.com/cresta/director/pull/17329)
