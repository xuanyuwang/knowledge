# analyticsimpl

## Package Overview

This package implements the `AnalyticsService` gRPC server for the insights-server. It handles 57+ RPC methods covering conversation retrieval, analytics chart evaluation, insights topic management, keyword detection, and 40+ stats retrieval variants across three query backends (Elasticsearch, ClickHouse, PostgreSQL).

## Code Structure

### Core Files

- **`analyticsimpl.go`** — Defines `AnalyticsServiceImpl` struct (the main service) and `Init()` function. The struct holds all dependencies (DB clients, gRPC registries, caches, feature flags).
- **`module.go`** — DI wiring. Provides `Module` (binds `AnalyticsService` gRPC) and `UserOutcomeModule` (binds `UserOutcomeService` via a wrapper).

### RPC Implementation Pattern

Each RPC method lives in its own file named after the method:
- `retrieve_conversation_messages.go` — `RetrieveConversationMessages`
- `evaluate_analytics_chart.go` — `EvaluateAnalyticsChart`
- `retrieve_agent_stats.go` — `RetrieveAgentStats`
- `action_retrieve_conversation_assistance_usage_stats.go` — `RetrieveConversationAssistanceUsageStats` resolves the shared eligible population and user ACLs. `action_retrieve_conversation_assistance_usage_stats_clickhouse.go` reuses seven feature-family row queries, filters sources by eligible agents and joins reduced feature pairs to the external eligible-pair table without GLOBAL IN, bounds concurrency to three and fails the request on required-evidence errors. The action file validates requests and intersects conversation/agent pairs, returns distinct overall/Agent counts, summed-agent Team counts and start-time DAILY/WEEKLY buckets. Population selection owns metadata filters; Notes and Guided Workflow evidence use the events-cluster predicates. Opt-in conversation usage details return Unimplemented in this phase.

**Typical RPC flow:** validate request -> parse parent (customer/profile) -> apply ACL filtering -> build query (ES/ClickHouse/Postgres) -> execute -> transform response -> return.

### EvaluateAnalyticsChart (multi-file)

See [`docs/chart-evaluation.md`](../../docs/chart-evaluation.md) for the full request shape, metric/group-by/filter semantics, composite aggregation, and combo charts.

This complex RPC is split across several files:
- `evaluate_analytics_chart.go` — Entry point
- `evaluate_analytics_chart_filter.go` — Filter construction
- `evaluate_analytics_chart_groupby.go` — GROUP BY logic
- `evaluate_analytics_chart_metric.go` — Metric calculations
- `evaluate_analytics_chart_es_response.go` — ES response processing
- `evaluate_analytics_chart_acl.go` — ACL filtering
- `evaluate_analytics_chart_common.go` — Shared helpers

### ClickHouse Helper Files (`*_clickhouse.go`)

Many stats RPCs have a companion `_clickhouse.go` file (e.g., `retrieve_agent_stats_clickhouse.go`). The main RPC file (e.g., `retrieve_agent_stats.go`) calls helper methods defined in the `_clickhouse.go` file for ClickHouse query building and execution. The `_clickhouse.go` files define private methods on `AnalyticsServiceImpl` that the main RPC orchestrates.

### Shared Helpers (`common_*.go`)

- **`common.go`** — Generic utilities: safe division, trend line, sorting, keyword expression parsing
- **`common_clickhouse.go`** — ClickHouse table enums, column constants, query helpers
- **`common_customer_insights.go`** — ES query construction (`getESQuery`, `getESQueryInternal`, `inferTargetDocumentType`), message filter parsing. These are methods on `AnalyticsServiceImpl`.
- **`common_insights_topic.go`** — Insights topic operations and moment fetching
- **`common_moment_traversal.go`** — Moment hierarchy traversal and evaluation
- **`common_user_filter.go`** — User/group ACL filtering (`ParseUserFilterResult`)

### Subdirectories

- **`elasticsearch/`** — ES query building and client wrapper
  - `request.go` — ES query construction: filters, aggregations, field mappings, feature flags (`envflag.Bool`). This is the largest file.
  - `lucene_regex.go` — RE2 to Lucene RegExp translation for regex moments. Handles case-insensitive mode, character classes, quantifiers, groups. Rejects anchors (^, $) and word boundaries (\b, \B).
  - `lucene_regex_test.go` — Regex translation tests (56 test cases)
  - `response.go` — ES response parsing
  - `client.go` — ES client wrapper and JSON utilities
  - `common.go` — ES constants and helpers
  - `detect_keyword_moments.go` — Keyword moment detection queries
- **`postgres/`** — PostgreSQL query building (`common.go`, `sql_query_builder.go`)
- **`converter/`** — ClickHouse query/response converters for specific RPCs
- **`testing/`** — Fake gRPC clients for tests (`fake_config.go`, `fake_moment.go`, `fake_trend_and_anomaly.go`)
- **`testdata/`** — Golden JSON files for ES request verification

### Configuration & Feature Flags

There are two layers of feature control in this package:

#### Cluster-Wide Env Flags

Set via environment variables in Helm/K8S, applied to all customers in a cluster:

1. **`envflag.Bool()` in `elasticsearch/request.go`** — package-level `*bool` pointers, parsed once at startup via `envflag.Parse()`
2. **`os.Getenv()` in `analyticsimpl.go` `Init()`** — read into struct fields on `AnalyticsServiceImpl`

Both read the same env var. In production a single Helm value suffices.

#### Per-Customer/Per-Profile Config Flags

Fetched at the start of each RPC via `configClient`:

```go
profileConfig, err := a.configClient.GetProfileConfig(parent.CustomerID, parent.ProfileID)
```

- **`configClient.GetProfileConfig(customerID, profileID)`** — returns `*config.ProfileConfig` (per-profile settings)
- **`configClient.Get(customerID)`** — returns `*config.CustomerConfig` (all profiles for a customer)

**Accessing flags:** Chain through the Features proto:
```go
profileConfig.GetFeatures().GetInsights().GetIncludeEmptyConversationInElasticsearchQuery()
profileConfig.GetFeatures().GetConversation().GetEnableReadingDenormalizedConversationMetadataFromMessageDoc()
```

Feature sub-objects include `Insights`, `Conversation`, `Voice`, etc., each containing booleans and settings specific to that domain.

**Common pattern — env flag as global override, config flag as per-customer control:**
```go
func (a AnalyticsServiceImpl) includeEmptyConversations(..., profileConfig *config.ProfileConfig, ...) (bool, error) {
    if *includeEmptyConversations == TrueString {  // cluster-wide env flag
        return true, nil
    }
    return profileConfig.GetFeatures().GetInsights().GetIncludeEmptyConversationInElasticsearchQuery(), nil  // per-customer
}
```

Config is read once per RPC request and threaded through the call stack.

**Types are defined in** `config/shared/grpcclient/config/` — `ProfileConfig` wraps `*npconfigpb.ProfileConfig`, `CustomerConfig` wraps `*nonpubconfigpb.CustomerConfig`.

---

## Testing Guide

There are two base test suites corresponding to the two query backends, each with distinct patterns.

### Running these tests locally (embedded Postgres)

Both suites embed `shared/db/testing.DBTestSuite`, which boots an **embedded Postgres v15** per
suite (cached under `~/.embedded-postgres-go`) unless `TEST_DATABASE_URL` is set.

- **Do not run these via Bazel without a real DB.** Bazel's `darwin-sandbox` cannot fetch the
  embedded-Postgres binary and fails instantly with `no version found matching 15.1.0`. Run with
  `go test` instead (it has network + the binary cache), e.g.
  `go test -count=1 -run TestRetrieveClosedConversations ./insights-server/internal/analyticsimpl/`.
  Bazel only works here if `TEST_DATABASE_URL` points at a real Postgres.
- **If a prior `go test` was interrupted**, the next run may fail with
  `could not create shared memory segment: No space left on device` — leaked embedded-Postgres
  processes + SysV shm segments. Clean up:
  ```bash
  pkill -9 -f "/.embedded-postgres-go/extracted/"
  rm -rf ~/.embedded-postgres-go/extracted/*
  # remove the orphaned (NATTCH=0) shm segments left by kill -9
  ipcs -m -o | awk '$5=="'"$USER"'" && $7==0 {print $2}' | xargs -n1 ipcrm -m
  ```

### Two Base Test Suites

| Suite | File | Used by | Server | Query backend |
|-------|------|---------|--------|---------------|
| `AnalyticsV2TestSuite` | `v2_base_test.go` | ES-based RPCs (RetrieveConversationMessages, EvaluateAnalyticsChart, RetrieveClosedConversations, etc.) | Full gRPC test server via module DI | Elasticsearch (mock HTTP server) |
| `AnalyticsTestSuite` | `base_test.go` | Stats RPCs (RetrieveAgentStats, RetrieveHintStats, RetrieveScorecardStats, etc.) | Direct `AnalyticsServiceImpl` struct | ClickHouse (mock connection) |

### Elasticsearch Tests (`AnalyticsV2TestSuite`)

`AnalyticsV2TestSuite` sets up:

- Real PostgreSQL database (via `DBTestSuite`) with historic + director schemas
- Mock ES server (`shared/elasticsearch/testing`) that intercepts HTTP requests and compares against golden JSON
- Mock gRPC clients (moment, behavior, coaching, user, config, etc.) via `gomock`
- All files from `testdata/` loaded into `s.testData` map, keyed by filename
- Full gRPC test server — tests call via `r.Client` (a real gRPC client)

**Writing a new ES query test:**

1. Creates a suite: `r := SomeTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}`
2. Calls `r.SetupSuite()` (which starts the server and loads testdata)
3. Defines test cases with:
   - `req`: the gRPC request proto
   - `esReqPath`: filename of the golden JSON file in `testdata/` (the expected ES request)
   - `esRespPath`: filename of a mock ES response (can be empty string if response content doesn't matter)
   - `want`: expected gRPC response proto (nil if only verifying the ES request)
   - Mock response fields for gRPC dependencies (e.g., `getCompositeMomentResp`, `getChildMomentResp`)
4. For each test case:
   - Sets up mock expectations (`EXPECT()`)
   - Appends mock ES responses: `r.mockESServer.Resps = append(r.mockESServer.Resps, r.getFile(tc.esRespPath))`
   - Sets ES request expectation: `r.mockESServer.EXPECT().Search(r.IndexName, r.getFile(tc.esReqPath))`
   - Calls the gRPC method via `r.Client`
   - The mock ES server automatically compares the actual request against the golden file using `go-cmp` with JSON normalization

**Generating ES golden files:**

Golden files cannot be handwritten for complex ES queries. Use this workflow:

1. Write the test pointing to a new golden file path
2. Create a placeholder file with content `{}`
3. Run the test — it will fail but the mock ES server logs the actual request body
4. Extract the actual JSON: `go test -v -run TestName ... 2>&1 | grep "body:" | head -1 | sed 's/.*body: //' | python3 -m json.tool > testdata/golden_file.json`
5. Re-run the test to confirm it passes

The JSON comparison is structure-based (parsed JSON), so formatting differences don't matter.

### Environment Flags (envflag) in Tests

Many ES query behaviors are gated behind environment flags. There are two flag mechanisms that may need to be set:

- **`envutil.SetEnv(key, value)`** — calls `os.Setenv`. Used by code that reads flags via `os.Getenv()` during `Init()` (sets struct fields on `AnalyticsServiceImpl`).
- **`envflag.Set(key, value)`** — directly updates the `*bool` pointer registered by `envflag.Bool()`. Used by package-level flags in `elasticsearch/request.go`.

**Why both are needed in tests:** `envflag.Parse()` runs only once per process. If a prior test already triggered parsing, `os.Setenv` alone won't update the already-parsed `*bool`. `envflag.Set` bypasses this by directly mutating the pointer.

**In production**, only one env var is needed — both mechanisms read from the same environment before the process starts.

**Pattern for flag-dependent tests:**

```go
func TestWithFlag(t *testing.T) {
    envutil.SetEnv("FLAG_NAME", "true")
    require.NoError(t, envflag.Set("FLAG_NAME", "true"))
    t.Cleanup(func() {
        envutil.SetEnv("FLAG_NAME", "false")
        _ = envflag.Set("FLAG_NAME", "false")
    })

    r := SomeTestSuite{AnalyticsV2TestSuite: newTestSuite(t)}
    r.SetupSuite()
    // ... test cases ...
}
```

**For flags only needed before suite setup** (read once during `Init()`), setting `envutil.SetEnv` before `SetupSuite()` is sufficient — see `SetupSuite` in `retrieve_conversation_messages_test.go` for examples.

### Mocking Per-Customer Config in Tests

Two approaches depending on test needs:

**Simple (most ES query tests):** Use `mockconfig.NewMockClient` and return empty configs:
```go
r.mockConfigClient.EXPECT().
    GetProfileConfig(gomock.Any(), gomock.Any()).Times(1).
    Return(&config.ProfileConfig{}, nil)
```

**With feature overrides:** Use `FakeClient` from `config/shared/grpcclient/config/testing/`:
```go
resetFunc := fakeConfigClient.OverrideProfileFeatures(customerID, profileID, func(f *configpb.Features) {
    f.Insights.IncludeEmptyConversationInElasticsearchQuery = true
})
defer resetFunc()
```

`FakeClient` initializes all feature sub-objects to empty values to prevent nil pointer panics. The `OverrideProfileFeatures` / `OverrideCustomerFeatures` methods return a reset function for cleanup.

### Mock ES Server (`shared/elasticsearch/testing`)

- `MockElasticSearchServer` is an `httptest.Server` that intercepts ES HTTP requests
- `Resps` field: append mock response strings (one per expected ES call, in order)
- `EXPECT().Search(indexName, expectedJSON)`: registers an expectation that compares the actual request body against `expectedJSON` using `go-cmp` with `cmputil.TransformJSONString()` (JSON-aware comparison)
- The server logs each request at INFO level, useful for debugging: `mock ES server: URL: ... body: ...`

### ClickHouse Tests (`AnalyticsTestSuite`)

`AnalyticsTestSuite` (in `base_test.go`) sets up:

- Real PostgreSQL database (same as V2 suite)
- **Direct `AnalyticsServiceImpl` struct** — no gRPC server, tests call methods directly via `r.server.RetrieveAgentStats(r.ctx, req)`
- Mock gRPC clients wired directly into the struct (user, config, ACL, etc.)
- ClickHouse mock connections (`convoMockConn`, `authEventMockConn`, `eventMockConn`) — activated by calling `r.UseMockedClickhouseConnections()` in `SetupSuite`
- Golden `.sql` files from `testdata/` loaded into `s.testData` map

**Writing a new ClickHouse stats test:**

1. Create a suite embedding `AnalyticsTestSuite` and call `r.UseMockedClickhouseConnections()` in `SetupSuite`
2. Define test cases with:
   - `req`: the gRPC request proto
   - `mockValues`: `[][]any` — rows returned by mock ClickHouse (each row is a `[]any` of column values)
   - `want`: expected gRPC response proto
   - `chQueryPath`: filename of the golden `.sql` file in `testdata/` (optional, for query verification)
3. For each test case:
   - Mock ClickHouse response: `mockRows := clickhouseConvoTesting.NewMockRows(tc.mockValues)` then `r.convoMockConn.On("Query", mock.Anything, mock.Anything, mock.Anything).Return(mockRows, nil)`
   - Mock config client (often with per-customer features via text proto)
   - Call the RPC method directly: `got, err := r.server.RetrieveSomeStats(r.ctx, tc.req)`
   - Verify the ClickHouse SQL query against golden file:
     ```go
     gotQuery := r.convoMockConn.Calls[0].Arguments.Get(1).(string)
     gotArgs := r.convoMockConn.Calls[0].Arguments.Get(2).([]any)
     expectedCHQuery := r.getFile(tc.chQueryPath)
     r.Assert().Equal(formatSQL(expectedCHQuery), formatSQL(combineQueryAndArgs(gotQuery, gotArgs)))
     ```
   - Verify response with `cmp.Diff(tc.want, got, protocmp.Transform())`
4. Use `defer r.convoMockConn.AssertExpectationsAndReset(r.T())` to verify all mock expectations and reset between subtests

**ClickHouse mock details:**
- `clickhouseConvoTesting.MockConn` is a testify/mock-based mock (not gomock)
- `NewMockRows([][]any)` creates mock rows from a slice of row data
- The `Query` method receives `(ctx, query string, args ...any)` — query is a parameterized SQL string with `?` placeholders, args are the values
- `combineQueryAndArgs` (in `base_test.go`) substitutes `?` with actual arg values for golden file comparison
- `formatSQL` strips all whitespace for comparison so formatting differences don't matter

**Golden SQL file naming:** `clickhouse_{RPCName}_{TestCaseDesc}_request.sql`

**Config mocking in ClickHouse tests** often uses text proto for richer configs:
```go
mockCustomerConfig := fn.Must(config.NewCustomerConfig(sharedproto.ParseFromTextProto(`
    customer: "customers/customer_id"
    profile_configs {
        name: "customers/customer_id/profiles/profile_id"
        features {
            insights {
                exclude_ingestion_pipeline_from_agent_stats: true
            }
        }
    }
`, &nonpubconfigpb.CustomerConfig{})))
mockConfigClient.EXPECT().Get(gomock.Any()).Return(mockCustomerConfig, nil)
```

### Test Conventions

- ES golden file naming: `{RPCName}_{TestCaseDesc}_es_request.json`
- ClickHouse golden file naming: `clickhouse_{RPCName}_{TestCaseDesc}_request.sql`
- ES test suites embed `AnalyticsV2TestSuite`; stats test suites embed `AnalyticsTestSuite`
- Use `t.Run(tc.desc, ...)` or `s.Run(...)` for table-driven subtests within a single server instance
- Use separate top-level `Test*` functions when different env flag configurations are needed (each gets its own server)
