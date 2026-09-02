# CONVI-7402 QA metadata filter OR semantics (Team value + (no value))

**Ticket:** [CONVI-7402](https://linear.app/cresta/issue/CONVI-7402/bswift-users-getting-qa-score-stats-errors)
**Domain:** analytics (`qa-score`)
**Source repo / branch:** `/Users/xuanyu.wang/repos/go-servers-convi-7402` / `convi-7402-bswift-users-getting-qa-score-stats-errors`

## Background

Bswift users hit HTTP 400 when filtering QA score stats by a metadata field (example: Team) when the UI selects:

- one concrete value (e.g. `POD 1`), and
- `(no value)` at the same time.

Frontend represents that selection as a `MomentGroup` that includes both `moments` and `excluded_moments` for the *same metadata moment template id*.

Elasticsearch already treats this pattern as OR semantics for the “overlap” id set; ClickHouse needed parity.

## Implementation overview

### 1) Parser: detect overlap and emit paired predicate

In `insights-server/internal/analyticsimpl/common_clickhouse.go`:

- `parseMomentConditionsForQAAttribute` was extended to recognize the overlap pattern:
  - `len(momentGroup.Moments) > 0`
  - `len(momentGroup.ExcludedMoments) > 0`
  - and both reference the same metadata moment template id
- Instead of rejecting include+exclude overlap, the parser now builds an `overlapMomentFilters` entry (a `qaMomentOverlapFilter` pair):
  - `valueFilter`: concrete selected metadata values
  - `missingFilter`: the “moment template exists” anchor used to represent `(no value)`

This behavior remains gated by `CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS` (defaults enabled).

### 2) ClickHouse query builders: express overlap as LEFT JOIN + OR predicate

In both ClickHouse QA paths:

1. `insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go`
2. `insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go`

Overlap is implemented as:

- LEFT JOIN the overlap `valueFilter`
- LEFT JOIN the overlap `missingFilter`
- Add an OR predicate for the overlap template id:
  - overlap matches if the value join found a row OR the missing join indicates absence

Unrelated moment groups and excluded-only moment groups continue to be AND-ed exactly as before.

## Tests and golden SQL updates

- Parser unit test:
  - `TestParseMomentConditionsForQAAttribute_MetadataMomentOverlap`
- Query-level unit regression + SQL golden comparisons:
  - `TestRetrieveQAScoreStatsClickhouseQuery_FilterByMetadataMomentGroups_OverlapValuePlusNoValue`
    - golden SQL:
      - `insights-server/internal/analyticsimpl/testdata/clickhouse_RetrieveQAScoreStats_FilterByMetadataMomentGroups_OverlapValuePlusNoValue_request.sql`
  - `TestQaConversationsClickhouseQuery_FilterByMetadataMomentGroups_OverlapValuePlusNoValue`
    - golden SQL:
      - `insights-server/internal/analyticsimpl/testdata/clickhouse_RetrieveQaConversations_FilterByMetadataMomentGroups_OverlapValuePlusNoValue_request.sql`

## Validation run

- `gofmt` on touched `analyticsimpl` Go sources.
- `go test ./insights-server/internal/analyticsimpl -run '...Overlap...'` (overlap-focused tests).
- `bazel run //:gazelle` (no BUILD changes produced).

Note: embedded-postgres-based full suites were not run in this environment due to an `initdb` shared-memory failure.

