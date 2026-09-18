# CONVI-7681: RCG Director analytics latency

**Status:** Todo, investigation pending  
**Priority:** Urgent  
**Primary domain:** `analytics`  
**Primary subdomains:** `performance-insights`, `conversation-volume`, `shared-analytics-platform`  
**Official ticket:** [CONVI-7681](https://linear.app/cresta/issue/CONVI-7681/rcg-director-analytics-takes-30-50-seconds-and-blocks-live-training)  
**Last updated:** 2026-09-11

## Objective and Impact

- Identify and mitigate intermittent 30-50+ second RCG Director analytics loads that blocked most of an hour-long live training and affected every leader on the call.
- Confirmed surfaces include Performance Insights, Assistance Insights, Opera rules, and Opera Analyzer.

## Scope

- `RetrieveQAScoreStats` tail latency, conversation-volume latency, ClickHouse/query cost and waits, concurrent insert/backfill impact, and bounded cache mitigation.

## Current Understanding

Andra's HAR captured four parallel seven-day `RetrieveQAScoreStats` calls that all returned HTTP 200 but took 47.380-50.212 seconds. Server wait accounted for essentially all elapsed time; receive time was below 1.1 ms. The calls covered current/prior time-range, agent-tier, and agent groupings, with response bodies from 944 bytes to 167,157 bytes. The recording separately shows conversation volume remaining blocked after other tables load and small views taking unexpectedly long.

The Slack thread identifies two hypotheses—elevated RCG insert volume after a reindex/backfill and RCG's removal from the analytics cache—but neither is established as causal. Prior observability likewise proved long-tail latency without proving an RCG request burst or ClickHouse's exact fraction of end-to-end time.

## Next Actions

1. Correlate the captured requests with backend and ClickHouse telemetry.
2. Compare fast and slow executions by query shape, rows/bytes read, memory, execution time, queue/resource waits, and concurrent writes.
3. Isolate the conversation-volume path from the QA-score groupings.
4. Evaluate a bounded temporary cache mitigation only with an explicit stale-data tradeoff.
5. Validate the durable fix on cold and warm loads across representative RCG cohorts.

## Timeline

- 2026-09-11 — Created urgent Linear ticket from Slack, HAR, and recording evidence. Details: `sessions/2026-09-11/codex-rcg-director-latency-ticket.md`.
- 2026-09-11 — Revised the Linear description to contain latency impact, evidence, hypotheses, investigation, and exit criteria only.
