# CONVI-7777 GLOBAL IN precedent investigation

- Date: 2026-10-21 (America/Toronto).
- Domain/subdomain: analytics / assistance-insights.
- User requested locating the historical convs_with_aa PR and their comment on Sebastian Coteanu’s endpoint; this turn is investigation, not authorization to rewrite/push another query implementation.
- Source worktree remains clean at `f383b394c48e8eb45505d6c7ff5b275816141674`.

## Exact comment found

[Performance Review: Join Pattern Comparison](https://github.com/cresta/go-servers/pull/26253#issuecomment-4047593153), authored by `xuanyuwang` on March12,2026, on Sebastian’s [CONVI-6403 / PR26253](https://github.com/cresta/go-servers/pull/26253), “Fix user outcome stats distributed table join.” Retrieved the comment directly through GitHub API and verified author/timestamp/body. The original endpoint implementation was [PR25875](https://github.com/cresta/go-servers/pull/25875), RetrieveUserOutcomeStats.

The comment compares AgentStats’ convs/convs_with_aa CTE-to-CTE join with a raw distributed table GLOBAL LEFT JOIN against a broad, unaggregated weight CTE. It identifies initiator materialization plus broadcast to every shard, a raw distributed left side, raw right-side rows, a self join and weaker right-side filters. It recommends moving both sides into filtered CTEs with an ordinary outer join. The merged PR body confirms the production error was Code288/DISTRIBUTED_IN_JOIN_SUBQUERY_DENIED and the final solution used both CTEs. Current retrieve_user_outcome_stats_query.go retains outcomes_main/outcomes_weight_filtered and an ordinary LEFT JOIN.

The AgentStats convs_with_aa introduction is [CONVI-3659 / PR17161](https://github.com/cresta/go-servers/pull/17161), merged October19,2024, commit `61e80f285c2542faf57a83cf3d79f5e8957e5b5c`, authored by Xuanyu. Both CTEs aggregate by requested output keys before joining. PR17161’s body separately reports a historical Oportun/all-agents/180-day comparison of22s versus8s; those measurements are not evidence for the new API’s runtime. The name correction followed in PR17216.

## Application to the new API

The published evidence adapter line33 applies `(conversation_id, agent_user_id) GLOBAL IN (SELECT conversation_id, agent_user_id FROM assistance_usage_population)` inside feature sources. Lines95-115 construct/send a fresh external table containing the entire eligible-pair population for each family query. There are seven family queries, concurrency3; feature tables lack request timestamp partition bounds.

This is not the same raw distributed self-join: the right-hand relation here is a client-supplied external table. The historical comment does not establish that this exact query will raise Code288, and GLOBAL is not a blanket forbidden keyword. It does establish the relevant broadcast/materialization concern; large eligible sets and repeated source reads require investigation. Single-node/local correctness and race tests do not validate multi-shard network/memory/latency cost.

Recommended next design check: compare with the prefiltered/reduced CTE-to-CTE pattern, retain conversation/agent keys until union, and verify actual distributed execution rather than treating CTE naming as a materialization guarantee. Do not fix this by mechanically removing GLOBAL or applying feature-row timestamp filters, because those may change distributed membership or the approved stale-timestamp semantics. No production benchmark or EXPLAIN was run. No product files, GitHub comments or branches were modified. Git/gh used normal authentication; no credential entry was selected/read.

## Approved query revision

The user subsequently authorized updating the queries, pushing PR #33993, and recording the concern in Analytics. The new adapter supplies a deduplicated eligible-agent list as ordinary query parameters, reduces each qualifying feature relation to conversation/agent pairs, and joins that reduced relation with the external population using a normal INNER JOIN. The shared feature helpers and legacy endpoints are unchanged. Summary preserves its internal three-key match; population intersection uses two keys. Each concurrent query still owns an independent external table.

This is a deliberate tradeoff: removing explicit GLOBAL cohort membership avoids that SQL broadcast pattern, but each selected agent’s historical feature pairs may reach the initiator before the exact population intersection. No timestamp bound was introduced. Client-to-initiator uploads, initiator join memory, native-protocol network volume and production latency remain unmeasured; no performance gain is claimed.

Validation: the no-GLOBAL SQL guard failed on the published implementation, then passed after the change. Full API Go suite passed 93 named nodes in 36.065s; focused race passed 52 named nodes; an additional real-ClickHouse test verifies two eligible agents, duplicate population membership, an excluded third agent and an outside conversation. The final Bazel result and pushed commit are recorded below.

A separate disposable two-node/two-shard ClickHouse 23.8.16.40 fixture executed all seven generated queries with distributed_product_mode=deny and prefer_localhost_replica=0. It verified evidence on either shard, duplicate reduction, exact population intersection and Summary generation/copy split across shards. Remote SQL has no eligible-population reference. Plans place the population join above distributed reduction. The old CLI refuses external tables for EXPLAIN, so plans used a same-schema initiator Memory table; actual SELECT executions used external tables. The fixture’s first setup attempt failed because a macOS /tmp bind mount did not provide the cluster config inside Colima; copying the config and reloading it resolved setup. No source change was needed for either diagnostic limitation. Containers and their network were removed afterward.

[Durable query guidance](../../subdomains/assistance-insights/clickhouse-distributed-evidence-queries.md) and [fixture, SQL, plans, result rows and test summaries](convi-7777-query-shape-evidence/). Git/gh used normal authentication; no AWS, Okta, Azure or SSH credential entry was selected or read.

## Follow-up file consolidation

The first query-revision push completed at a10aecb539183a7d8a9770e1d30301b75e141204 before the user’s follow-up arrived. The user requested the conventional two-file API layout before further publication. Merged conversation_assistance_usage_aggregation.go into action_retrieve_conversation_assistance_usage_stats.go and renamed conversation_assistance_usage_evidence_clickhouse.go to action_retrieve_conversation_assistance_usage_stats_clickhouse.go. A byte comparison confirmed that all non-import implementation text is unchanged and the renamed ClickHouse file is identical. Gazelle regenerated BUILD.bazel. This is recorded as a separate follow-up commit rather than rewriting the already-pushed query revision.

Final consolidation commit: `0e6919128928f99c5601f1582e03fb9b630694df`. Full API Bazel passed after consolidation in 37.2s (50.797s command elapsed). Gazelle, gofmt, exact body-preservation check and git diff --check pass. PR #33993 retains base #34044; no merge.

Publication verified: GitHub PR #33993 head is `0e6919128928f99c5601f1582e03fb9b630694df`, base remains `xw/convi-7777-summary-genai-refactor`, PR is open, and the local API worktree is clean. At the final check, only the automatic-label triage check was reported and queued; no latest-head CI success is claimed. PR body and construction report now describe the query revision and two-file layout.

## Future query-count optimization candidate

Xuanyu identified frontend/backend coordination as future work to reduce total queries. Recorded a Linear ticket candidate in the canonical CONVI-7777 work item, covering the population-plus-seven-family baseline, end-to-end page request/query tracing, compatible response/shared-computation options and measurable query-count/latency/read-volume/memory validation. This is a candidate only; no Linear issue was created and the current PR is unchanged.
