# CLO Filter Performance Results — NCLH Production

**Date:** 2026-07-28
**Environment:** `us-east-1-prod`, conversations ClickHouse 26.3.12.3
**Database:** `nclh_us_east_1`
**Request window:** 2026-01-30 04:00:00 through 2026-07-29 03:59:59
**CLO template:** `019601ed-4f8f-7884-98bc-8dd6043945a9`, boolean true
**Scorecard template:** `019dda54-f49a-77b2-a405-32e65391f4a3`
**Safety:** read-only, concurrency 1 for direct measurements, 120-second timeout, no cache clearing
**Status:** Preliminary production evidence; not a complete A–J benchmark

## Executive result

The supplied six-month CLO filter materially amplifies ClickHouse work. For the closest same-filter, same-result-cardinality full-query comparison, CLO increased coordinator-reported reads by approximately **10.1× rows** and **7.5× bytes**, while latency increased from **66.67 s to 85.83 s** (**1.287×**, +19.16 s). Both completed under the 120-second frontend threshold.

The page-level “longest `qaScoreStats:retrieve`” proxy increased only from 83.60 s to 85.83 s (**1.027×**, +2.23 s) because a different no-CLO score query already dominated the baseline page. Page critical path and CLO intrinsic cost must therefore be reported separately.

The direct CLO annotation CTE measurement took **17.83 s**, scanned **2.017B rows / 245.4 GB**, and issued two raw-table scans per shard. Index pruning was weak: the six-month interval selected 95.6% of local granules, and the moment-type skip index still left 90.9% of all granules. The evidence points to raw-table layout, repeated scans, and JSON payload reads as the dominant avoidable work.

## Full query evidence from `system.query_log`

The exact backend SQL recovered from query history uses `moment_annotation_d_read` twice, `MAX(create_time)` by conversation in t1, `JSONHas` plus `JSONExtractBool` in t2, a latest-timestamp self-join, and a join to `scorecard_score_per_conversation`. It also depends on an external `agent_filter` table supplied by the backend, so the exact full SQL was not blindly replayed from the CLI.

### Closest controlled full-query pair

| Metric | No CLO | CLO | CLO / no CLO |
|---|---:|---:|---:|
| Query duration | 66,667 ms | 85,830 ms | 1.287× |
| Read rows | 272,919,872 | 2,764,516,920 | 10.13× |
| Read bytes | 46,599,372,822 | 349,162,856,860 | 7.49× |
| Result rows | 98 | 98 | 1.00× |
| Memory usage | 4,890,070,618 | 1,780,781,083 | 0.36× |

The no-CLO query retains the same scorecard, use case, date range, daily group-by, voicemail `conversation` CTE, and result rows. The lower CLO memory likely reflects a smaller post-filter score aggregation, not cheaper annotation work.

### Repeated normalized-shape history

| Shape | n | Median | p90 | Range | Median rows | Median bytes | Median memory |
|---|---:|---:|---:|---:|---:|---:|---:|
| No CLO, principal matching shape | 3 | 66,667 ms | 70,894 ms | 66,147–70,894 ms | 271,691,912 | 46,379,277,008 | 4,896,641,275 |
| CLO variant A | 2 | 73,933 ms | 76,695 ms | 71,171–76,695 ms | 2,748,085,371 | 344,470,952,111 | 1,724,511,219 |
| CLO variant B | 1 | 85,830 ms | 85,830 ms | single run | 2,764,516,920 | 349,162,856,860 | 1,780,781,083 |

Variant A’s median latency ratio versus the principal no-CLO shape is 1.109×, with approximately 10.11× rows and 7.43× bytes. Variant B is the exact slowest SQL recovered from the supplied page load. Differences between CLO normalized hashes must be explained before pooling them.

## Page critical-path proxy

The longest matching top-level query in the likely no-CLO page window was 83.604 s. The longest CLO-bearing query in the later page window was 85.830 s:

```text
page proxy slowdown = 85.830 / 83.604 = 1.027×
absolute change     = +2.226 s
```

This is one non-randomized page iteration per state, separated in time and subject to concurrent cluster load. It is an observed user-impact sample, not a stable p95 estimate.

The CLO page window contained 18 top-level CLO-bearing queries across different widgets/shapes:

- median duration: 70.327 s;
- p90: 76.695 s;
- maximum: 85.830 s;
- median coordinator reads: 2.693B rows / 332.2 GB;
- median memory: 1.725 GB.

Do not sum these durations because page requests can execute concurrently.

## `EXPLAIN indexes = 1`

The distributed plan shows two `ReadFromRemote` branches joined together. A local-table explain for the value scan reported:

| Index stage | Parts | Granules |
|---|---:|---:|
| Six-month primary-key time range | 16/16 | 71,467/74,782 (95.6%) |
| `idx_moment_type` skip index | 15/16 | 67,967/74,782 (90.9% of all granules) |

The plan produced 3,005 ranges. No pruning was shown for `moment_template_id` or the JSON outcome value.

## Direct CLO annotation-component measurement

Tagged query ID: `clo-perf-20260728-clo-annotation-01`

| Metric | Result |
|---|---:|
| Client/query duration | 17.821 / 17.829 s |
| Matched latest conversations | 171,088 |
| Read rows | 2,017,300,452 |
| Read bytes | 245,367,440,892 |
| Coordinator memory | 595,074,178 |
| Exceptions | none |

Child scan decomposition:

| Component | Shard queries | Rows | Bytes | Max shard time | User CPU | System CPU |
|---|---:|---:|---:|---:|---:|---:|
| Latest scan without JSON | 3 | 1,211,295,698 | 110,403,894,572 | 8.597 s | 64.01 s | 7.10 s |
| Value scan with JSON | 3 | 806,004,754 | 134,963,546,320 | 9.126 s | 70.92 s | 9.85 s |

This confirms two large raw scans per group. The value scan reads fewer rows but more bytes and consumes more CPU because it must read and parse the payload.

## JSON micro-comparison

Both queries use the same table, six-month range, moment type, template ID, `count()` output, warmed state, and 120-second limit.

| Metric | Template/type/time only | Add boolean JSON predicate | Ratio |
|---|---:|---:|---:|
| Duration | 0.467 s | 3.400 s | 7.28× |
| Read rows | 1,211,230,600 | 1,257,258,354 | 1.04× |
| Read bytes | 60,792,760,010 | 183,312,356,108 | 3.02× |
| Shard user CPU | 35.15 s | 80.52 s | 2.29× |
| Shard system CPU | 4.40 s | 11.00 s | 2.50× |
| Selected marks | 176,138 | 184,412 | 1.05× |
| Annotation rows | 1,389,768 | 171,956 | — |

The count-only query is not the full production CTE, but it isolates payload-column and JSON-predicate cost. Duration is cache-sensitive; byte and CPU amplification are the stronger single-pair signals.

## Interpretation

### Measured conclusions

- The request completed below the 120-second threshold in observed runs, but the slowest full query had only 34.17 seconds of headroom.
- CLO caused very large resource amplification even when page critical-path latency grew less dramatically.
- The annotation component alone accounted for 17.83 s and 245.4 GB in a warmed execution.
- JSON payload filtering tripled bytes and more than doubled shard user CPU in the paired count shape.
- The two-scan/latest-join design is a major cost alongside JSON; the non-JSON latest scan alone read 110.4 GB.
- The raw-table key/index layout pruned little for this six-month template/value query.

### Structural expectations supported, but not yet fully proven

- A typed, time-partitioned CLO MV should substantially reduce raw reads and JSON CPU.
- Per-group routing does not help this CLO-only request; it remains important for mixed metadata+CLO requests but requires a separate H/I measurement.
- Table layout and repeated scans appear at least as important as JSON extraction. A prototype MV benchmark is required to quantify achievable latency.

## Limitations

- Only one customer/profile, one long range, one boolean-true CLO, and one scorecard were tested.
- Direct component queries have one measured repetition each; they are diagnostic, not p95 estimates.
- Query-history comparisons were not randomized/interleaved and may reflect changing cluster load/cache state.
- The page proxy is inferred from ClickHouse top-level queries, not captured browser network timing.
- Full-query replay was avoided because the backend supplied an external `agent_filter` table.
- Multiple normalized CLO shapes were observed and must not be pooled until their SQL/request differences are explained.
- ReplacingMergeTree versions and create-time/update-time semantics were not resolved in this run.

## Recommended next measurements

1. Repeat the page state 7 times in interleaved no-CLO/CLO order, capturing request IDs; use 20+ only for a p90/p95 decision.
2. Run 1-day and 30-day versions of the same boolean filter to establish range scaling.
3. Run the same template with boolean false and verify missing-key semantics.
4. Select a cardinality-matched metadata value and complete B/C/D.
5. Add one metadata filter and measure H/I for request-wide versus per-group routing.
6. Prototype a typed CLO MV in staging and replay the direct annotation component before approving schema work.

## Decision signal

This evidence is sufficient to justify designing and staging a typed CLO MV prototype: the observed full-query read amplification is approximately 7.5× by bytes and the direct annotation component consumes 245 GB per request. It is not yet sufficient to approve production schema deployment or claim a stable p95 improvement. Per-group routing should be evaluated independently for mixed requests.
