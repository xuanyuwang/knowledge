# CLO Filter Performance Test Plan

**Date:** 2026-07-27
**Scope:** `AnalyticsService.RetrieveQAScoreStats` for Performance Insights
**Status:** Ready for review; no database performance queries have been run
**Source snapshots inspected:** `go-servers` `85d639ecaf75626fd060852a5ee4196df492361c`; `clickhouse-schema` `ed4d69160f72c32ad7dcebfddeb0dd32a81d97e3`

## 1. What “performance downgrade” means

A downgrade is a repeatable increase in ClickHouse work or user-visible latency for a semantically equivalent request, after controlling for customer/profile, time range, scorecard scope, grouping, filter selectivity, matched conversations, returned rows, cache state, and cluster conditions.

Measure four distinct effects rather than one ambiguous “CLO versus metadata” number:

```text
Raw metadata regression  = median(metadata_raw) / median(metadata_mv)
CLO relative cost        = median(clo_raw) / median(selectivity_matched_metadata_mv)
Mixed routing regression = median(mixed_current) / median(mixed_per_group)
End-user impact          = median(CLO_or_mixed_request) / median(no_moment_filter_request)
```

Compute the same ratios for `query_duration_ms`, `read_rows`, `read_bytes`, and `memory_usage`. Report absolute deltas as well as ratios. For latency tails, compare p90/p95 directly. Do not interpret `clo_raw / metadata_mv` as pure implementation overhead unless the two filters have comparable input rows, matched-conversation counts, output rows, and value frequency.

The primary database outcome is ClickHouse execution time and work. The primary product outcome is end-to-end RPC/browser latency. Keep them separate because API cache hits, request preprocessing, network time, retries, response decoding, and frontend fan-out are outside ClickHouse.

For the UI track, use the longest `qaScoreStats:retrieve` network request triggered by one page load or filter action as the primary, easy-to-repeat page-performance proxy:

```text
qa_score_stats_critical_path_ms = max(duration_ms for matching qaScoreStats:retrieve requests in the iteration)
```

Use the maximum, not the sum, because the page may issue several requests concurrently. Record the number of matching requests and click/navigation-to-stable-render as companion metrics. The longest request is a strong network critical-path proxy, but it is not identical to complete page load if frontend rendering or another dependency finishes later.

## 2. Evidence from the current implementation

### Request and query path

- Director builds the RPC payload in `/Users/xuanyu.wang/repos/director/packages/director-api/src/services/cresta-api/insights/insightsApi.ts:264` and calls `AnalyticsService.RetrieveQAScoreStats` at line 426.
- The frontend query hook is `/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/hooks/useQAScoreStats.ts:6`; request parameters are assembled in `useQAScoreStatsRequestParams.ts:47`.
- The backend entry point is `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go:57`. An application stats cache can return before ClickHouse at lines 163–170.
- Normal ClickHouse execution is selected at `retrieve_qa_score_stats.go:303`; template QA-score configuration can fan one RPC into multiple bounded-parallel ClickHouse queries at lines 715–809.
- `readQaScoreStatsFromClickhouse` is at `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go:439`.

### Routing and generated SQL

- Request-wide routing is selected at `retrieve_qa_score_stats_clickhouse.go:485`:

  ```go
  fetchFromMetadataView := !qaAttributeHasConversationOutcomeMomentGroup(req.FilterByAttribute)
  ```

- The detector at `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/common_clickhouse.go:1470` returns true for a single `CONVERSATION_OUTCOME` moment even if it has no value attributes. The parser only emits a CLO group when `OutcomeValueAttributes` is non-empty at lines 834–841.
- Moment parsing is `parseMomentConditionsForQAAttribute` at `common_clickhouse.go:802`. Metadata conversion is at line 1231; CLO conversion is at line 1299; string/boolean/number predicates are at lines 1339–1381.
- The query builder is `qaScoreStatsClickhouseQueryWithMetadataView` at `retrieve_qa_score_stats_clickhouse.go:247`. For each included group, lines 272–323 build:
  1. a scan/group-by for the latest annotation per conversation;
  2. a second scan applying the value predicate;
  3. a join on conversation ID and latest timestamp;
  4. an inner join to `scorecard_score_per_conversation`.
- Metadata uses `moment_annotation_mv_by_metadata_d` and `update_time`; raw routing uses `moment_annotation_d` and `create_time` (`retrieve_qa_score_stats_clickhouse.go:262–267`).
- Existing golden SQL:
  - CLO: `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/testdata/clickhouse_RetrieveQAScoreStats_FilterByConversationOutcomeMomentGroup_request.sql`
  - metadata: `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/testdata/clickhouse_RetrieveQAScoreStats_FilterByMetadataMomentGroups_request.sql`
- The generating CLO test is `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_test.go:122`; predicate coverage, including false, exact number, and numeric bin, is in `common_clickhouse_test.go:1120–1198`.

### Physical layout

- Canonical raw-table DDL is `/Users/xuanyu.wang/repos/clickhouse-schema/conversations/migrations/20230824160348_init_db.up.sql:166`. It is a `ReplicatedReplacingMergeTree(..., update_time)` with primary key `(toStartOfHour(conversation_start_time), agent_user_id, policy_id)` and no declared monthly partition at lines 217–220. The distributed table is at lines 222–223.
- The metadata MV is at lines 569–584. It is filtered to `moment_type = 19`, projects seven columns, partitions by month, and orders by hour/conversation/template. Its distributed table is at lines 586–588.
- CLO is `moment_type = 14` and reads `moment_annotation_payload`. Exact false and zero use `JSONHas` plus extraction so missing keys do not match the default values. Numeric bins currently extract the numeric JSON value once per bound.

These are structural expectations, not measured performance conclusions.

## 3. Experimental controls and workload selection

Choose at least one high-volume customer/profile in staging with production-like data. If staging is not representative, use the controlled-production gate in section 8. For every comparison pair, hold constant:

- customer/profile and cluster;
- exact half-open time interval `[start, end)` and time target (started-at or ended-at);
- score resource, scorecard templates, criteria, agent/group filters, frequency, and group-by fields;
- outcome/metadata value frequency as closely as available;
- matched distinct conversations and final `result_rows`/`result_bytes` as closely as available;
- query settings and coordinator node.

Build a workload catalog before timing:

| Dimension | Required levels |
|---|---|
| Time range | 1 day; 7 or 30 days; representative long PI range (use the product default/commonly used maximum, recorded explicitly) |
| Volume | at least one high-volume profile; optionally a median-volume profile as a sensitivity check |
| Selectivity | selective value (target approximately bottom decile of non-empty value frequencies); low-selectivity value (approximately top decile) |
| Moment groups | 0, 1, 2 mixed, and realistic worst-case CLO count observed from UI/product constraints |
| Execution state | first eligible run after randomized/interleaved idle work (“cold-ish”); warmed repetitions; never clear OS/page caches |

Before choosing values, run bounded, approved discovery queries or use existing product aggregates. Record, per candidate template/value: annotation rows, distinct conversations, missing-key count, value count, and fraction of the time-range conversation population. Prefer exact metadata/CLO pairs whose matched distinct-conversation counts differ by no more than an agreed tolerance (proposed starting tolerance: 10%). If no such pair exists, use cardinality bands and analyze latency against `read_rows` and matched conversations rather than claiming a direct overhead ratio.

Example discovery shapes (run in the already-selected profile database, with a bounded interval and approved template ID):

```sql
-- Metadata string-value frequency from the optimized source.
SELECT
    metadata_string_value AS value,
    count() AS annotation_rows,
    uniqExact(conversation_id) AS conversations
FROM moment_annotation_mv_by_metadata_d
WHERE conversation_start_time >= {start:DateTime64(6)}
  AND conversation_start_time < {end:DateTime64(6)}
  AND moment_template_id = {metadata_template_id:String}
GROUP BY value
ORDER BY conversations DESC
LIMIT 100;

-- CLO string-value frequency and missing-key population.
SELECT
    JSONHas(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') AS key_exists,
    JSONExtractString(moment_annotation_payload, 'conversation_outcome_payload', 'string_value') AS value,
    count() AS annotation_rows,
    uniqExact(conversation_id) AS conversations
FROM moment_annotation_d
WHERE conversation_start_time >= {start:DateTime64(6)}
  AND conversation_start_time < {end:DateTime64(6)}
  AND moment_type = 14
  AND moment_template_id = {clo_template_id:String}
GROUP BY key_exists, value
ORDER BY conversations DESC
LIMIT 100;
```

Use analogous guarded queries for boolean and numeric values. If `uniqExact` is too expensive for the bounded discovery window, obtain owner approval for `uniq`/`uniqCombined` and label the count approximate; final result-equivalence checks must remain exact.

For false and zero, separately count:

- key exists and value is false/zero;
- key is missing;
- another oneof member is present.

This is both a correctness check and a selectivity control.

## 4. Core A–J matrix

Run every applicable case at each time range and selectivity level. Keep the outer score query identical. “Hypothesis” means what the code structure predicts; accept or reject it from measurements.

| ID | Query shape | Source/routing | Exact comparison | Hypothesis / purpose |
|---|---|---|---|---|
| A | No moment filter | no annotation CTE | baseline for D, H, J | Establish outer score-query and result-cardinality cost. |
| B | One metadata value filter | metadata MV, `update_time` | B vs C | Optimized metadata reference. Test string first; add typed numeric/bool when matching CLO cardinality. |
| C | Same metadata template/value forced raw | raw table, `moment_type=19`, `create_time` | C/B | Isolates raw metadata regression, subject to result-equivalence validation. |
| D | One CLO string exact-value filter | raw, `moment_type=14`, JSON string extraction | D/B (matched) and D/A | Measures real CLO path, not JSON alone. |
| E1 | One CLO boolean `true` | raw + `JSONHas`/`JSONExtractBool` | E1/B (matched), E1/A | Typed boolean cost and correctness. |
| E2 | One CLO boolean `false` | same | E2/E1 when cardinality-matched; E2/A | Ensures missing keys do not inflate false matches. |
| F1 | One CLO numeric exact non-zero | raw + `JSONHas`/`JSONExtractFloat` | F1/B (matched), F1/A | Exact numeric cost. |
| F2 | One CLO numeric exact zero | same | F2/F1 when matched; F2/A | Ensures missing keys do not inflate zero matches. |
| G | One CLO numeric bin | raw + two JSON extractions | G/F with matched cardinality; G/A | Tests repeated extraction and bound selectivity. |
| H | Metadata + CLO, current request-wide raw routing | both groups raw; two scans/group | H/I and H/A | Quantifies metadata regression plus extra group/join under current behavior. |
| I | Same mixed request, safely simulated per-group | metadata CTE on MV/`update_time`; CLO CTE raw/`create_time` | H/I | Direct estimate of per-group routing benefit. Require equivalent result set before latency comparison. |
| J | Multiple CLO groups at realistic maximum | raw; two scans and one outer join per group | J/D and scaling by group count | Tests repeated scans, joins, shrinking intersections, memory, and timeout risk. |

For B/C, use the identical metadata template and value. C is generated by taking B’s SQL and changing only the metadata CTE source to `moment_annotation_d`, latest timestamp to `create_time`, and adding `moment_type = 19`, matching current raw conversion. Do not edit the scorecard portion.

For H/I, start from H’s fully bound SQL. In I change only metadata-group CTEs to `moment_annotation_mv_by_metadata_d`, use `update_time`, and remove `moment_type = 19` because the MV does not expose it. Leave CLO groups on raw/`create_time`. Compare a stable result fingerprint (row count plus ordered hash or exported result diff) before treating H/I as a performance pair. A mismatch is a semantic finding—likely create/update-time or ReplacingMergeTree state—not a benchmark result to average away.

### Repetition and ordering

For each workload cell:

1. Run `EXPLAIN indexes = 1` once for the fully bound query; save the complete output.
2. Run one unrecorded warm-up. If the first run is intentionally retained as “cold-ish,” label it and do not combine it with warmed statistics.
3. Run at least 7 measured repetitions per shape for an initial screen. Use at least 20 measured repetitions when reporting p90/p95 or making a tail-latency decision; increase further if variance or autocorrelation remains high.
4. Generate a seeded random order within blocks (for example, one occurrence each of B/C/D/A per block), and interleave paired shapes rather than running all of one shape consecutively.
5. Pause or abort if cluster health/load guardrails are crossed. Do not increase concurrency to mimic UI fan-out until single-query shapes are safe.

Report median, p90, p95 (only when sample size supports it), standard deviation, coefficient of variation, min/max, and bootstrap confidence intervals for paired median ratios when practical. Preserve raw runs; do not silently remove outliers. Annotate retries, exceptions, deploys, merges, or workload incidents and provide sensitivity results with and without invalidated runs.

## 5. Component-decomposition experiments

Run these after A–J, on the same catalog, to identify the dominant cost. Each step must return either `count()`/a compact fingerprint or the same conversation-ID cardinality so client transfer does not dominate.

| Experiment | Controlled change | Diagnostic signal |
|---|---|---|
| Raw layout | B vs C, same metadata rows/value | Large C/B `read_rows`, `read_bytes`, selected marks/parts indicates table projection/layout/pruning dominates. |
| JSON predicate | CLO raw time+type+template scan with and without value predicate, using `count()` | Similar reads but higher CPU/time/ProfileEvents with JSON points to extraction cost. Never compare different returned payload sizes. |
| Exact vs bin | F vs G with matched-conversation count | Extra CPU/time without extra reads suggests repeated numeric extraction/bound evaluation. |
| Latest scan | execute/measure t1 alone, t2 alone, then t1–t2 join | Separates group-by/latest cost, value scan cost, and self-join/distinct cost. |
| Repeated groups | D, then 2…N CLO groups using matched-frequency templates | Near-linear reads with group count indicates repeated scans; super-linear memory/time suggests joins/intersection or contention. |
| Outer score join | annotation filter CTE ending in count/fingerprint vs full RetrieveQAScoreStats query | Delta identifies score table/join/aggregation and result-cardinality contribution. |
| Result cardinality | same query shape with selective vs common values; regress duration on `read_rows`, matched IDs, and `result_rows` | Distinguishes scan cost from join/aggregation/output growth. |
| Shard balance | aggregate child query-log metrics by host | One host dominating duration/read rows suggests shard skew or slow replica. |

Also capture `EXPLAIN PIPELINE` or `EXPLAIN PLAN` in staging if allowed; these are secondary evidence. `EXPLAIN indexes = 1` is mandatory because it shows parts/granules and index pruning. Do not infer JSON CPU directly from duration alone under changing cluster load.

## 6. Obtaining representative payloads and generated SQL

### From the UI

1. In staging Performance Insights, open browser developer tools, Network, and filter for `RetrieveQAScoreStats`.
2. Select a stable page/widget and record all requests triggered by the page before adding a moment filter. Performance Insights can issue several scorecard/template/grouping requests; do not assume one UI action equals one ClickHouse query.
3. Export/copy the request payload for A, then apply B/D/H/J filters and copy each payload and response timing. Redact authorization headers/cookies; keep customer/profile IDs only in the access-controlled results artifact.
4. Record the browser request ID/trace ID, request start/end, TTFB, transferred bytes, status, and whether a hard reload or React Query reuse occurred.
5. Confirm the payload’s `filterByAttribute.momentGroups` contains the expected moment type and `metadataValueAttributes` or `outcomeValueAttributes`. Preserve `filterByTimeRange`, `conversationTimeRangeField`, score resource, grouping, and scorecard scope.
6. Correlate the request ID with backend logs. A ClickHouse query ID generated by `QueryWithRetry` embeds the request ID when available (`/Users/xuanyu.wang/repos/go-servers/shared/clickhouse/shared/common.go:256–274`). If no ClickHouse query appears, label the UI observation as an application/cache hit and exclude it from DB execution statistics.

### Generated SQL

Use three sources, in this order:

1. `system.query_log.query` for the exact staging execution, keyed by query/initial query ID. This best preserves the executed SQL and settings.
2. Backend logs: the request is logged at debug level in `retrieve_qa_score_stats.go:229` and the SQL template at `retrieve_qa_score_stats_clickhouse.go:542`. Never paste credentials or headers.
3. The existing pure query-generation tests and golden fixtures listed in section 2. Run the focused test locally to validate the query shape; create a disposable, unmerged test case only if representative bound values cannot be recovered safely.

Archive for each shape: sanitized protobuf/JSON payload, fully bound SQL, query hash/normalized hash, source commit, settings, and expected result fingerprint. Parameterize customer/profile, timestamps, template IDs, and values in the benchmark runner; do not hand-edit ten unrelated query copies.

## 7. Measurement and query-log collection

Assign an explicit query ID such as:

```text
clo-perf-<run-date>-<shape>-<range>-<selectivity>-<block>-<rep>
```

For API/UI runs use the generated query ID and map it back to the logical run ID. Retries have an attempt suffix; keep each attempt and mark the logical request as retried.

After query-log flush latency has elapsed, collect coordinator and shard records. `system.query_log` is node-local; use `clusterAllReplicas('conversations', system.query_log)` (or the versioned-log `merge` form used by the repository sample-query guide) when authorized. Filter to `type = 'QueryFinish'` for completed attempts, but separately retain exception rows.

Example read-only collection SQL (verify column availability on the deployed ClickHouse version first):

```sql
SELECT
    hostname() AS host,
    event_time,
    type,
    query_id,
    initial_query_id,
    is_initial_query,
    query_duration_ms,
    read_rows,
    read_bytes,
    result_rows,
    result_bytes,
    memory_usage,
    ProfileEvents['SelectedParts'] AS selected_parts,
    ProfileEvents['SelectedRanges'] AS selected_ranges,
    ProfileEvents['SelectedMarks'] AS selected_marks,
    ProfileEvents['SelectedRows'] AS selected_rows,
    ProfileEvents['UserTimeMicroseconds'] AS user_cpu_us,
    ProfileEvents['SystemTimeMicroseconds'] AS system_cpu_us,
    exception_code,
    exception,
    query
FROM clusterAllReplicas('conversations', system.query_log)
WHERE event_time >= {window_start:DateTime}
  AND (query_id = {query_id:String} OR initial_query_id = {query_id:String})
ORDER BY event_time, host, query_id
SETTINGS skip_unavailable_shards = 1;
```

If parameter syntax is unavailable in the chosen client, bind values through the client rather than interpolating untrusted text. For direct distributed queries, report both:

- coordinator elapsed time (user-facing ClickHouse duration);
- summed shard work (`read_rows`, `read_bytes`, CPU) and max child duration by host.

Do not sum coordinator and child durations as elapsed time. Record `EXPLAIN indexes = 1` output separately; selected partitions and granules are plan evidence, while `SelectedParts`/`SelectedMarks` are runtime ProfileEvents when available. Record all query settings that could affect caching, joins, memory, or distributed execution. If supported and approved, disable only the ClickHouse query-result cache per benchmark session; do not clear filesystem/mark/uncompressed caches.

Client timing should measure wall-clock from dispatch through full row consumption. The backend’s current latency log starts immediately before `QueryWithRetry` and ends after row scanning (`retrieve_qa_score_stats_clickhouse.go:551–586`), so it includes ClickHouse round-trip and result consumption. Compute:

```text
API overhead      = API/RPC wall time - backend ClickHouse-span wall time
client/network gap = direct-client wall time - coordinator query_duration_ms
```

Treat negative/small gaps as clock/instrumentation noise; do not combine clocks from unsynchronized hosts.

## 8. Safe execution procedure

### Phase 0 — review and access

- Approve the workload catalog, maximum time range, repetitions, timeout, concurrency (start at 1), and abort thresholds with ClickHouse/analytics owners.
- Resolve the exact backend and schema revisions deployed in the test environment and regenerate/revalidate the SQL shapes. The inspected local checkouts were behind their remotes, so their hashes are evidence snapshots rather than proof of deployed state.
- Confirm read-only credentials, allowed cluster/database, `system.query_log` access, deployed schema/version, and whether query-result cache settings may be changed per session.
- Confirm no selected credential or profile is labeled BREAK GLASS. Do not read or use any such credential.

### Phase 1 — local/static validation

- Run only existing query-generation unit tests; no production data is needed.
- Validate that A–J payload mutations affect only the intended moment groups.
- Parse/format every SQL variant and verify table names, `moment_type`, time column, and JSON predicates.

### Phase 2 — staging

- Use production-like or anonymized volume. First run `EXPLAIN indexes = 1`; reject any accidental unbounded/full-cluster shape.
- Execute short/selective cases first at concurrency 1 with a conservative per-query timeout.
- Review reads, memory, duration, and cluster health before widening time range or group count.
- Complete A–J and decomposition tests, then run UI tests against the same payload catalog.

### Phase 3 — controlled production only if needed

Production is justified only when staging cannot reproduce the relevant volume/layout or the staging result is too close to a decision threshold. Obtain explicit owner approval for the exact customer/profile, SQL hashes, windows, repetitions, schedule, timeout, and abort thresholds.

- Use read-only access and one query at a time in a low-traffic window.
- Begin with `EXPLAIN indexes = 1`, 1-day selective A/B/D, then stop for review.
- Expand one dimension at a time. Never clear caches, run `FINAL`, force merges, mutate settings cluster-wide, or create artificial parallel UI load.
- Abort on timeout/exception, unexpected scanned rows/bytes, memory pressure, replica/shard degradation, or owner-defined utilization threshold.
- Do not use `SYSTEM FLUSH LOGS` without approval; poll for normal query-log delivery.

## 9. UI end-to-end test track

UI tests validate product impact but cannot isolate database components by themselves.

For A, B, D, H, and J:

1. Use the same staging profile, widget/page, date range, scorecard scope, and group-by.
2. Start capture immediately before the page load/filter action. Filter the network waterfall to the exact request name `qaScoreStats:retrieve`; record every match and the maximum completed-request duration as `qa_score_stats_critical_path_ms`.
3. Run one hard-navigation/warm-up and at least 7 measured page/filter interactions in seeded, interleaved order. Browser “disable cache” affects HTTP cache, not necessarily React Query or the backend stats cache.
4. Record click-to-stable-render, each matching request’s duration/TTFB, total bytes, matching-request count, `qa_score_stats_critical_path_ms`, and whether ClickHouse was actually reached.
5. Correlate all RPCs to query-log entries. Report both per-query metrics and page-level critical-path latency. When requests run concurrently, page latency is not their sum.
6. Test the realistic worst-case filter count and a common user path, not only synthetic maximums.

An iteration with zero matching requests is not a measured database-backed page load; label it as a frontend/application cache observation and exclude it from the primary network distribution. If the browser automation surface exposes Resource Timing entries for the request, collect them programmatically after the page settles. Otherwise export/copy the filtered browser network entries; do not substitute visual stopwatch timing.

Use UI results for `CLO or mixed / no-moment-filter` end-user impact. Use direct SQL for B/C, H/I, and decomposition because the UI cannot currently select routing implementations independently.

## 10. Results-recording template

One row per physical ClickHouse attempt; use a separate logical-request table to group retries/fan-out.

```csv
run_id,logical_request_id,query_id,initial_query_id,attempt,shape,source_commit,environment,cluster,coordinator_host,customer_profile_alias,time_start,time_end,range_days,time_field,selectivity_class,filter_template_alias,filter_value_alias,filter_value_type,estimated_value_frequency,matched_conversations,number_of_moment_groups,table_source,routing_mode,warm_state,warmup_or_measured,random_block,execution_order,matching_qa_request_count,qa_score_stats_critical_path_ms,click_to_stable_render_ms,client_wall_ms,api_wall_ms,backend_ch_span_ms,query_duration_ms,read_rows,read_bytes,result_rows,result_bytes,memory_usage,selected_partitions,selected_parts,selected_ranges,selected_marks,user_cpu_us,system_cpu_us,cache_settings,retry_count,exception_code,exception,normalized_query_hash,result_fingerprint,notes
```

Summary table:

| Pair | Range/selectivity | n | median ms | p90 | p95 | CV | median read rows/bytes | median memory | slowdown ratio | paired CI | result equivalent? |
|---|---|---:|---:|---:|---:|---:|---:|---:|---:|---:|---|
| C / B | | | | | | | | | | | |
| D / B matched | | | | | | | | | | | |
| H / I | | | | | | | | | | | |
| D / A | | | | | | | | | | | |
| H / A | | | | | | | | | | | |

Attach the request payload, SQL, explain output, raw query-log export, cluster-health annotation, and analysis notebook/script version to each benchmark batch.

## 11. Confounders and interpretation rules

- **Different moment populations/distributions:** compare matched cardinality bands and input rows; never call raw metadata/CLO latency difference pure JSON overhead without decomposition.
- **Query/result caches:** label first/warm runs; exclude application-cache hits from DB samples; record settings. Never clear shared caches.
- **Concurrent load:** interleave pairs, annotate cluster metrics/incidents, and rerun high-variance blocks.
- **ReplacingMergeTree versions:** raw and MV may expose duplicate/current versions differently until merges. Avoid `FINAL`; compare result fingerprints and inspect duplicate-version counts separately.
- **`create_time` vs `update_time`:** current raw routing uses `create_time`; MV uses `update_time`. H/I and B/C can differ semantically when annotations are updated. Treat mismatch as a correctness/design issue.
- **Missing vs false/zero:** keep `JSONHas` guards and verify missing-key counts. Numeric bins do not currently use a presence guard; validate that bin boundaries cannot accidentally include extraction default zero.
- **Distributed behavior:** gather all replicas/shards, distinguish coordinator latency from summed work, and report max-host skew.
- **Shard imbalance/replica choice:** retain host-level metrics and query settings; do not hide a slow shard inside cluster totals.
- **Fan-out:** `RespectTemplateQaScoreConfig` can create multiple parallel ClickHouse queries. Report RPC-level critical path, query count, max concurrency, and total cluster work separately.
- **Result cardinality:** larger intermediate conversation sets increase outer join/aggregation cost even with identical annotation scan work.
- **Time semantics/partition pruning:** test started-at and ended-at only if both are real PI use cases; the raw key and MV partition are aligned to conversation start time.

## 12. Acceptance criteria and decision framework

Before execution, owners must approve the product latency/error SLO and resource guardrails. The following are proposed decision rules, not claims about current performance:

### Keep the raw table

Choose this when all representative warmed CLO and mixed cases meet the approved p95/end-user SLO with headroom, no timeout/error/memory concern appears, slowdown is small and stable across long/high-volume cases, and decomposition shows no material avoidable resource amplification. Continue query-log monitoring after rollout.

### Implement per-group routing only

Choose this when H/I shows a reproducible, material mixed-routing improvement, results are semantically equivalent, metadata raw regression C/B is the dominant mixed penalty, and CLO-only D/E/F/G/J still meets SLO/resource guardrails. Also fix the router so only a valid CLO group with value attributes affects routing.

### Implement per-group routing plus a typed CLO MV

Choose this when per-group routing fixes metadata regression but CLO-only or multi-CLO cases still breach the approved latency/resource guardrail, or when decomposition consistently attributes the dominant cost to raw-table reads/layout and JSON extraction. Evidence should reproduce in multiple blocks and at least two time ranges, including the representative long/high-volume case.

A CLO MV is justified if at least one approved hard guardrail is breached (timeout/error, p95 product SLO, memory/read budget) or if owners accept a predeclared relative threshold with meaningful absolute impact. A proposed starting decision threshold is a reproducible ≥2× median latency or read amplification versus the selectivity-matched reference plus an absolute user-visible or capacity impact; owners must ratify or replace this before testing. Do not build an MV solely because one noisy CLO/metadata pair is slower.

The MV design should use typed string/number/bool columns plus an explicit value-type/presence discriminator, preserve false and zero, define update/version semantics, partition for PI time ranges, and order for time/template filtering. Benchmark a prototype/backfill separately before rollout; this plan only establishes the need.

## 13. Open questions and access requirements

1. Which staging profile has production-representative `moment_annotation` volume and CLO coverage for string, boolean (including false), numeric zero, and numeric bins?
2. What is the representative long PI range, current user-facing p95 SLO, RPC timeout, and acceptable ClickHouse read/memory budget?
3. What is the maximum CLO group count exposed by the UI, and what count is realistic in telemetry?
4. Can the tester read `system.query_log` across all `conversations` nodes and view required ProfileEvents? What ClickHouse version/columns are deployed?
5. Are API stats caching and ClickHouse query-result cache enabled for the selected profile, and can they be observed or disabled per staging session?
6. Can backend request/query IDs be correlated in staging logs, and are debug request logs available without exposing sensitive data?
7. Does the selected request enable `RespectTemplateQaScoreConfig`; if so, how many template-duration groups and parallel ClickHouse queries does it create?
8. Are there known annotation update patterns where `create_time` and `update_time` differ? What should “latest CLO” mean?
9. Can per-group routing be evaluated in a disposable staging branch, or should it be simulated only with read-only SQL?
10. Who approves controlled production queries and defines live cluster abort thresholds if staging is insufficient?

No production measurement should begin until questions 1–4 and the safety guardrails are resolved.
