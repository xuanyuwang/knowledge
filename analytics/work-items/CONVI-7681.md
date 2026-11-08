# CONVI-7681: RCG Director analytics latency

**Status:** Investigation active; pre-aggregation design narrowed
**Priority:** Urgent  
**Primary domain:** `analytics`  
**Primary subdomains:** `performance-insights`, `conversation-volume`, `shared-analytics-platform`  
**Official ticket:** [CONVI-7681](https://linear.app/cresta/issue/CONVI-7681/rcg-director-analytics-takes-30-50-seconds-and-blocks-live-training)  
**Last updated:** 2026-11-04

## Objective and Impact

- Identify and mitigate intermittent 30-50+ second RCG Director analytics loads that blocked most of an hour-long live training and affected every leader on the call.
- Confirmed surfaces include Performance Insights, Assistance Insights, Opera rules, and Opera Analyzer.

## Scope

- `RetrieveQAScoreStats` tail latency, conversation-volume latency, ClickHouse/query cost and waits, concurrent insert/backfill impact, and bounded cache mitigation.

## Current Understanding

Andra's HAR captured four parallel seven-day `RetrieveQAScoreStats` calls that all returned HTTP 200 but took 47.380-50.212 seconds. Server wait accounted for essentially all elapsed time; receive time was below 1.1 ms. The calls covered current/prior time-range, agent-tier, and agent groupings, with response bodies from 944 bytes to 167,157 bytes. The recording separately shows conversation volume remaining blocked after other tables load and small views taking unexpectedly long.

The Slack thread identifies two hypotheses—elevated RCG insert volume after a reindex/backfill and RCG's removal from the analytics cache—but neither is established as causal. Prior observability likewise proved long-tail latency without proving an RCG request burst or ClickHouse's exact fraction of end-to-end time.

November 4 read-only RCG measurements show that pre-aggregation is worth prototyping but a direct incremental materialized view is not correct. `score_d` and `scorecard_d` wrap versioned ReplacingMergeTree tables, while the API selects the latest scorecard version and joins score rows by both ID and source timestamp. An insert-trigger view would see every version and cannot retract the prior aggregate.

In the last 90 days, RCG had 58.62M physical score rows. A narrow daily agent/use-case/template/criterion key has about 5.64M distinct combinations (roughly 10.4x fewer rows), but adding submitter, manual mode, voicemail, and duration bin raises the candidate to 21.48M (roughly 2.7x fewer). These are row-cardinality estimates, not storage or latency benchmarks.

On one reachable node, 36,903 finished matching QA-stats queries in the last seven days had p50 0.39s, p95 2.01s, p99 4.35s, and max 26.27s while reading an average 36.42M rows / 1.31 GiB. This excludes canceled/failed queries and other nodes and does not invalidate the captured 47-50s page calls.

## Next Actions

1. Instrument normalized request shapes and correlate the captured requests with backend and ClickHouse query IDs, including canceled/failed queries and queue/resource waits.
2. Trace the scorecard projection writer and define an authoritative finalized or signed correction event; do not build the daily rollup directly from the current versioned tables.
3. Prototype criterion-level and scorecard-level daily rollups in a non-production database, then measure actual compressed bytes, backfill/insert cost, and old/new response parity.
4. Benchmark single-use-case and multi-use-case requests under realistic page fan-out, preserving ACL, time-basis, metadata, latest-version, N/A, autofail, and weighting semantics.
5. Keep unsupported request shapes on the raw path; evaluate a refreshable snapshot if the ingestion contract cannot change first.
6. Isolate the conversation-volume path and any temporary cache mitigation from the QA-score rollup work.

## Timeline

- 2026-09-11 — Created urgent Linear ticket from Slack, HAR, and recording evidence. Details: `sessions/2026-09-11/codex-rcg-director-latency-ticket.md`.
- 2026-09-11 — Revised the Linear description to contain latency impact, evidence, hypotheses, investigation, and exit criteria only.
- 2026-11-04 — Confirmed read-only RCG ClickHouse access, measured source/version/cardinality and recent finished-query cost, and rejected a naive insert-trigger rollup on correctness grounds. Proposed a correction-aware fact plus shared rollups, with refreshable snapshot as the interim alternative. [Plan](../deliverables/performance-insights-clickhouse-preaggregation-plan.md) and [session](../sessions/2026-11-04/codex-performance-insights-clickhouse-preaggregation.md).
