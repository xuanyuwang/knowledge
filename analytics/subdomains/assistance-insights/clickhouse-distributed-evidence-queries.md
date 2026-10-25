# Distributed ClickHouse evidence queries

For CONVI-7777, reduce each feature to conversation/agent pairs before joining its eligible population. Keep only the deduplicated eligible-agent list in source predicates. An ordinary join outside the distributed reduction replaces the previous GLOBAL IN eligible-pair membership test. This follows the reduced-relation pattern in convs_with_aa, while preserving late evidence whose copied conversation timestamp differs from the population timestamp.

## Why this matters

Xuanyu’s [performance review on Sebastian Coteanu’s PR #26253](https://github.com/cresta/go-servers/pull/26253#issuecomment-4047593153) explains that GLOBAL materializes matching right-hand rows at the initiator and broadcasts them to every shard. That change fixed RetrieveUserOutcomeStats after a distributed self-join failed with Code 288. [PR #17161](https://github.com/cresta/go-servers/pull/17161) introduced the filtered/aggregated convs_with_aa pattern.

GLOBAL is not categorically forbidden. Its cost depends on right-hand cardinality, filtering, reduction and shard count. The original CONVI-7777 right-hand table was a client external table, so the earlier self-join’s Code 288 was not proven for this API. The concern was repeated full-cohort GLOBAL set construction/broadcast, not a confirmed production failure.

## Chosen shape and proof boundary

The seven new API queries filter source rows to eligible agents, retain each shared helper’s feature predicates, reduce to conversation/agent pairs, and use a normal INNER JOIN with the external eligible-pair table. Summary generation/copy still joins all three internal keys, including copied start time. Exact pair intersection remains necessary: a selected agent’s use in another conversation must not qualify, and another agent’s use must not give credit to the selected agent.

A two-node/two-shard ClickHouse 23.8.16.40 fixture executed all seven generated queries with distributed_product_mode=deny and prefer_localhost_replica=0. KB evidence from either shard qualified; duplicate evidence collapsed; outside conversations and wrong-agent evidence did not qualify. Summary generation on one shard matched a copy on the other. EXPLAIN PLAN, using an initiator Memory table with the external table’s schema, placed aggregation above ReadFromRemote and the population join above aggregation. Actual SELECT queries used the native external table. Captured remote SQL contained source agent filters and no population-table reference. These observations establish the tested execution shape, not a production performance guarantee or a measurement of all native-protocol bytes.

## Remaining cost and review rules

- Seven queries still upload the population from the API process to the initiator, with at most three reads concurrently. The change removes explicit GLOBAL membership; it does not remove client upload or initiator join memory.
- The new source agent filter admits historical evidence outside the selected population. More reduced feature pairs can reach the initiator than with early full-pair membership. Measure this tradeoff on representative customer data.
- Feature timestamps cannot safely receive the request interval without a separate contract change: population start time owns selection, and copied feature timestamps can be stale.
- A CTE name alone does not force materialization or initiator execution. Verify the actual plan and cross-shard behavior for the deployed ClickHouse version/settings before claiming a performance gain.
- Before rollout, benchmark latency, scanned rows/bytes, network volume and peak memory across representative cohorts, agent counts and shard counts.

[Investigation and implementation record](../../sessions/2026-10-21/codex-convi-7777-global-in-precedent.md) · [Reproducible synthetic fixture and plans](../../sessions/2026-10-21/convi-7777-query-shape-evidence/).

## External agent transport and timestamp interpretation

The evidence adapter always sends deduplicated eligible agents through a query-owned external table. SQL uses agent_user_id IN (SELECT user_id FROM assistance_usage_agents); the full population remains in the outer ordinary join. An 8,001-agent regression failed with Max query size exceeded before this fix and passed afterward. A fresh two-shard fixture with 8,001 external agents executed all seven queries under distributed_product_mode=deny; remote SQL references the agent table, not the eligible-pair table. The earlier [local benchmark](../../../large-user-id-clickhouse/design-review.md) supports avoiding a size threshold, but does not establish production latency for these seven queries.

KB evidence conversation_start_time copies Chats.StartedAt, while CreateTime records event creation. Delayed ingestion alone does not cause a fourteen-day difference. An older event copy can remain if the conversation start is corrected and a later indexing operation skips existing event IDs. The October 6 / October 20 example is synthetic; actual correction magnitude and reindexing guarantees remain unverified. Do not treat arbitrary synthetic timestamp mismatches as established production behavior. See the [additional-review assessment](../../sessions/2026-10-21/codex-convi-7777-evidence-review-5604982557.md).
