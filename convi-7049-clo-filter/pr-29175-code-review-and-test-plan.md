# PR 29175 Code Review and Test Plan

**Date:** 2026-06-23
**PR:** `cresta/go-servers#29175`
**Branch/worktree:** `/Users/xuanyu.wang/repos/go-servers-convi-7049`

## Goal

Make the CLO backend PR easier to review line by line by anchoring the review around executable tests.

The most useful review path is:

```text
request/filter shape
-> qaAttributeHasConversationOutcomeMomentGroup
-> parseMomentConditionsForQAAttribute
-> convertConversationOutcomeMomentGroupFilter
-> qaScoreStatsClickhouseQueryWithMetadataView
-> generated ClickHouse SQL
```

## Tests Added

### 1. Parser-Level Unit Test: Exact Outcome Value

File:

```text
insights-server/internal/analyticsimpl/common_clickhouse_test.go
```

Test:

```text
TestParseMomentConditionsForQAAttribute_ConversationOutcomeMomentGroup
```

Purpose:

- Builds a `QAAttribute` with a `CONVERSATION_OUTCOME` moment group.
- Verifies `qaAttributeHasConversationOutcomeMomentGroup` returns true.
- Verifies `parseMomentConditionsForQAAttribute` emits:
  - `moment_type = CONVERSATION_OUTCOME`
  - `moment_template_id = outcome_moment_id`
  - string-value JSON extraction from `moment_annotation_payload`

### 2. Parser-Level Unit Test: Numeric Outcome Bin

File:

```text
insights-server/internal/analyticsimpl/common_clickhouse_test.go
```

Test:

```text
TestParseMomentConditionsForQAAttribute_ConversationOutcomeMomentGroupNumericBin
```

Purpose:

- Builds a `QAAttribute` with a `CONVERSATION_OUTCOME` moment group and a numeric-bin `OutcomeValueAttribute`.
- Verifies `qaAttributeHasConversationOutcomeMomentGroup` returns true.
- Verifies `parseMomentConditionsForQAAttribute` emits:
  - `moment_type = CONVERSATION_OUTCOME`
  - `moment_template_id = outcome_moment_id`
  - number-value JSON extraction from `moment_annotation_payload`
  - numeric-bin lower/upper bound predicates:
    - `JSONExtractFloat(...) >= ?`
    - `JSONExtractFloat(...) < ?`
  - numeric args `10.0` and `20.0`

Best breakpoints:

```text
qaAttributeHasConversationOutcomeMomentGroup
parseMomentConditionsForQAAttribute
convertConversationOutcomeMomentGroupFilter
conversationOutcomeStringValueExpression
conversationOutcomeNumberValueExpression
conversationOutcomeBoolValueExpression
```

### 3. Query-Generation Golden Test

File:

```text
insights-server/internal/analyticsimpl/retrieve_qa_score_stats_test.go
```

Test:

```text
TestRetrieveQAScoreStatsClickhouseQuery_FilterByConversationOutcomeMomentGroup
```

Golden SQL:

```text
insights-server/internal/analyticsimpl/testdata/clickhouse_RetrieveQAScoreStats_FilterByConversationOutcomeMomentGroup_request.sql
```

Purpose:

- Builds QA score stats ClickHouse query inputs from a CLO filter.
- Asserts the query uses `moment_annotation_d`.
- Asserts the query does not use `moment_annotation_mv_by_metadata_d`.
- Asserts the query filters:
  - `moment_type = 14`
  - `moment_template_id = 'outcome_moment_id'`
  - `JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') = 'resolved'`
  - numeric bin bounds on `conversation_outcome_payload.number_value`
- Compares the full generated SQL against a golden file.

Best breakpoints:

```text
parseCommonConditionsForQAAttribute
parseScorecardConditionsForQAAttribute
parseScoreConditionsForQAAttribute
qaAttributeHasConversationOutcomeMomentGroup
parseMomentConditionsForQAAttribute
qaScoreStatsClickhouseQueryWithMetadataView
concatConditionsAndArgsWithPredicate
combineQueryAndArgs
```

## Fast Test Commands

These tests do **not** need ClickHouse or Postgres containers. They are pure query-building tests and should be the first choice for local review/debugging.

From:

```bash
cd /Users/xuanyu.wang/repos/go-servers-convi-7049
```

Run the parser and query-generation tests:

```bash
go test ./insights-server/internal/analyticsimpl \
  -run 'TestParseMomentConditionsForQAAttribute_ConversationOutcomeMomentGroup|TestRetrieveQAScoreStatsClickhouseQuery_FilterByConversationOutcomeMomentGroup' \
  -count=1 -v
```

The parser regex above intentionally matches both parser tests:

```text
TestParseMomentConditionsForQAAttribute_ConversationOutcomeMomentGroup
TestParseMomentConditionsForQAAttribute_ConversationOutcomeMomentGroupNumericBin
```

Run only the exact-value parser test:

```bash
go test ./insights-server/internal/analyticsimpl \
  -run '^TestParseMomentConditionsForQAAttribute_ConversationOutcomeMomentGroup$' \
  -count=1 -v
```

Run only the numeric-bin parser test:

```bash
go test ./insights-server/internal/analyticsimpl \
  -run '^TestParseMomentConditionsForQAAttribute_ConversationOutcomeMomentGroupNumericBin$' \
  -count=1 -v
```

Run only the query-generation test:

```bash
go test ./insights-server/internal/analyticsimpl \
  -run TestRetrieveQAScoreStatsClickhouseQuery_FilterByConversationOutcomeMomentGroup \
  -count=1 -v
```

The query-generation test prints the final SQL because it calls:

```go
fmt.Printf("got ClickHouse Query: \n%v\n", gotCHQuery)
```

Use `-v` if you want to see the generated SQL in test output.

## Container-Backed Setup For Broader Tests

The focused tests above do not require containers. Broader `AnalyticsTestSuite` tests may start embedded Postgres and ClickHouse test infrastructure. For a more stable local debugging environment, use a real Postgres plus the reusable ClickHouse test container.

### Start Postgres Container

Use Docker Postgres 15:

```bash
docker rm -f go-servers-local-postgres 2>/dev/null || true
docker run -d \
  --name go-servers-local-postgres \
  -e POSTGRES_USER=cresta \
  -e POSTGRES_PASSWORD=cresta \
  -e POSTGRES_DB=postgres \
  -p 5432:5432 \
  postgres:15
```

Wait until Postgres is ready:

```bash
until docker exec go-servers-local-postgres pg_isready -U cresta -d postgres; do
  sleep 1
done
```

Export `TEST_DATABASE_URL`:

```bash
export TEST_DATABASE_URL='postgres://cresta:cresta@127.0.0.1:5432/postgres?sslmode=disable'
```

Why this works:

- `shared/db/testing.NewTestDatabase` checks `TEST_DATABASE_URL`.
- If set, it connects to that existing server.
- It drops/creates a per-test database name, so test data isolation is preserved.

### Start ClickHouse Container

From the go-servers worktree:

```bash
cd /Users/xuanyu.wang/repos/go-servers-convi-7049
export REUSABLE_CLICKHOUSE_CONTAINER_NAME=go-servers-local-clickhouse-node1
mage StartClickhouseTestContainers
```

The mage target starts:

```text
go-servers-local-clickhouse-zookeeper
go-servers-local-clickhouse-node1
```

For Bazel-based runs, also add the env passthroughs from the repo README:

```bash
echo 'test --action_env=HOME' >> ~/.bazelrc
echo 'test --action_env=DOCKER_HOST' >> ~/.bazelrc
echo 'test --test_env=REUSABLE_CLICKHOUSE_CONTAINER_NAME=go-servers-local-clickhouse-node1' >> ~/.bazelrc
echo 'test --test_env=TEST_DATABASE_URL=postgres://cresta:cresta@127.0.0.1:5432/postgres?sslmode=disable' >> ~/.bazelrc
```

### Run Broader QA Score Stats Suite

Once Postgres and ClickHouse are running:

```bash
cd /Users/xuanyu.wang/repos/go-servers-convi-7049
export TEST_DATABASE_URL='postgres://cresta:cresta@127.0.0.1:5432/postgres?sslmode=disable'
export REUSABLE_CLICKHOUSE_CONTAINER_NAME=go-servers-local-clickhouse-node1

go test ./insights-server/internal/analyticsimpl \
  -run 'TestRetrieveQaScoreStats' \
  -count=1 -v
```

For just the existing query-focused suite cases, prefer the fast tests above. The full suite does more setup and is slower.

## Embedded Postgres Cleanup

If running without `TEST_DATABASE_URL`, the suite may use embedded Postgres. If a previous run was interrupted, it can fail with:

```text
could not create shared memory segment: No space left on device
```

Cleanup steps:

```bash
pkill -9 -f '/.embedded-postgres-go/extracted/' || true
```

Remove stale shared memory segments with zero attachments:

```bash
for id in $(ipcs -m -o | awk -v user="$USER" '$5==user && $7==0 {print $2}'); do
  ipcrm -m "$id" || true
done
```

Then remove stale embedded Postgres directories. If there are many directories, avoid one huge glob expansion:

```bash
find ~/.embedded-postgres-go/extracted -mindepth 1 -maxdepth 1 -type d -exec rm -rf {} +
```

Using `TEST_DATABASE_URL` with the Postgres container avoids this embedded-Postgres path.

## Suggested Review Order

1. Read `moment-and-moment-annotation-background.md` for the data model.
2. Run the parser-level unit test with breakpoints.
3. Run the query-generation test with `-v` and inspect the generated SQL.
4. Compare the generated SQL against `clickhouse_RetrieveQAScoreStats_FilterByConversationOutcomeMomentGroup_request.sql`.
5. Review `common_clickhouse.go`:
   - how CLO moment groups are detected
   - how outcome values become JSON extraction conditions
6. Review `retrieve_qa_score_stats_clickhouse.go`:
   - how `fetchFromMetadataView` is chosen
   - how `moment_annotation_d` vs `moment_annotation_mv_by_metadata_d` is selected
   - how moment filters become CTEs and joins
7. Review `shared/clickhouse/conversations/converters.go`:
   - why oneof type switching preserves `0` and `false`
8. Review `shared/clickhouse/conversations/conversation.go`:
   - why `CONVERSATION_OUTCOME` always writes `moment_annotation_payload`

## What The Tests Prove

The new tests prove the backend code now builds the correct query shape for a CLO filter:

- CLO filters are recognized as `CONVERSATION_OUTCOME` moment groups.
- CLO filters use `moment_annotation_d`, not the metadata-only materialized view.
- CLO values are read from `moment_annotation_payload`.
- String and numeric-bin CLO filters are expressed as ClickHouse JSON extraction predicates.
- The filtered conversation IDs are joined into the QA score aggregation query.

They do not prove actual ClickHouse execution against seeded data. That would require a heavier integration test with real ClickHouse rows and should be a separate follow-up only if we want runtime data validation beyond query correctness.

## Review Questions Captured

### Is `shared/clickhouse/conversations/converters.go` just a refactor?

Mostly yes. The important behavior is preserving protobuf `oneof` value presence when converting moment annotation payloads. For CLO values, `false` and `0` are valid values, so review this file with that invariant in mind: code should not rely on truthiness to decide whether a value exists.

### Why does `shared/clickhouse/conversations/conversation.go` check `momentAnnotation.Type`?

Because `moment_annotation_payload` should be populated for the moment types the analytics query can safely interpret. `clickHouseEnableMomentAnnotationPayload` only controls whether payload writing is enabled; it does not make every materialized view or query path support every moment type.

For CONVI-7049, `CONVERSATION_OUTCOME` must be included because CLO filters read:

```text
moment_annotation_payload.conversation_outcome_payload
```

The type check also avoids silently changing payload behavior for unrelated moment annotation types.

### Why not silently support CLO as long as `clickHouseEnableMomentAnnotationPayload` is true?

The flag and the moment type solve different problems:

```text
flag enabled      -> payload may be written
moment type check -> this row type is intended to write/read payload
query table/view  -> this table actually contains the row and columns needed
```

For example, `moment_annotation_mv_by_metadata_d` cannot serve CLO rows even if payload writing is enabled because the view is metadata-only and does not expose the CLO payload.

### Why use `moment_annotation_d` instead of `moment_annotation_mv_by_metadata_d`?

`moment_annotation_mv_by_metadata_d` is pre-filtered to conversation metadata:

```sql
WHERE moment_type = 19
```

CLO is:

```text
moment_type = 14
```

Therefore CLO filters must query `moment_annotation_d` unless a new CLO-specific materialized view is created.

### Should we create a materialized view for `moment_type = 14`?

Not as part of the correctness fix. A dedicated CLO materialized view could improve performance later, but it requires schema design and rollout work. It must also preserve value presence explicitly so missing values do not get confused with valid `false` or `0` values.

### Why do tests use both `Moments` and `OutcomeValueAttributes`?

`Moments` identifies which outcome moment/template is being filtered:

```text
moment_type = CONVERSATION_OUTCOME
moment_template_id = outcome_moment_id
```

`OutcomeValueAttributes` identifies which selected values or numeric bins are allowed:

```text
outcome_value.string_value = "resolved"
outcome_value.boolean_value = false
numeric_bin = [10, 20)
```

Both are needed. One selects the CLO definition; the other selects the accepted values for that definition.

### Does `CONVERSATION_OUTCOME` support both exact values and numeric bins?

For filter requests, yes. `OutcomeValueAttributes` can represent:

```text
OutcomeValue -> exact boolean/string/number
NumericBin   -> range over a numeric outcome value
```

The stored protobuf value is still a `ConversationOutcomeValue` oneof:

```text
boolean_value | string_value | number_value
```

Numeric bins are filter ranges over stored numeric values, not a fourth stored CLO value type.

### Why guard exact boolean and exact number filters with `JSONHas`?

ClickHouse JSON extraction returns default values for missing keys:

```text
JSONExtractBool(...missing...)  -> false
JSONExtractFloat(...missing...) -> 0
```

Without a key-existence guard:

```text
boolean_value = false
number_value = 0
```

could match rows where the key is absent. The fix is to combine:

```text
JSONHas(payload, ..., key) = true
AND JSONExtract...(payload, ..., key) = expected_value
```

### Why is `create_time` the latest-time column for CLO filters?

The metadata materialized view branch uses `update_time` because that view exposes `update_time` as the freshness column.

CLO filters query `moment_annotation_d`. The broad moment annotation table path uses `create_time` for the latest-row CTE. The implementation keeps that existing broad-table behavior and only switches to `update_time` for the metadata view branch.
