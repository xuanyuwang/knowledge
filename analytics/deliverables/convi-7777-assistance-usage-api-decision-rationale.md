# CONVI-7777: Why assistance usage needs a dedicated API

**Audience:** Assistance Insights engineers and reviewers  
**Date:** October 13, 2026  
**Status:** Design decision and implementation direction. The dedicated protobuf contract is prepared for review; the new backend and frontend integration are not yet complete.

## Decision in brief

What we have now: we have stats APIs for different assistance type like hints, summary, notes, etc. Those APIs tells how many each assistance type there are.

What we need: a new metric that combines all assistance types, but aggregated to conversations: how many conversations have at least on assistance used for whatever type.

The biggest engineer challenges: how to make sure the new metric is aligned with all 7 existing stats APIs in terms of
1. the way of counting conversations: they include different conversation sources, user types, etc.
2. the way of counting each assistancy type: how can we reuse existing sql/code as much as possible?

High level solution: Adding `RetrieveConversationAssistanceUsageStats` to report the stats of conversations with at least one assistance use.

The service will reuse the **internal conversation eligibility rules** behind `RetrieveConversationStats` as the denominator, will use individual assistance feature stats API's logic to count each assistance.

This does mean repeating some existing feature SQL predicates in the new implementation. The current feature RPCs expose aggregate counts, and their queries do not provide reusable functions that return the conversation/agent keys needed for a union. Changing seven public RPCs and their query/response contracts first would be a much larger migration with behavior risk for existing Assistance Insights pages. We will keep that cleanup separate and make the duplicated predicates explicit and reviewable.

The following will elaborate why we duplicate code/sql. See section "Why the existing API responses cannot be aggregated".

## Release scope: V1 and V2

Here **V2** means the potential later-phase items called **P2** in the PM scope review; it is a scope bucket, not a committed delivery date or API contract.

| Phase | Included assistance use | Decision and boundary |
|---|---|---|
| **V1: this API** | Behavioral, checklist, Knowledge Base, and Guided Workflow **hint use**; GenAI answer use; Summary; Notes copy; Knowledge Base search; Guided Workflow document use; and chat Suggestions. | Ten product categories represented by seven existing stats RPCs. Count a conversation once when any qualifying use occurs. A shown, sent, or generated feature alone does not count. Summary follows the existing RPC's use predicate for parity while its copy/edit discrepancy remains an explicit decision. |
| **V2 / P2: later phase, if required** | Chat Smart Compose and **all email assistance**: partial/full reply Suggestions, email Smart Compose, and inserted email search results. | Product scope decision. Conversation linkage exists for some records, but historical coverage, use semantics, and missing query/projection work still need V2 design and validation. None contributes to the V1 numerator. |
| **Deferred, without a V2 commitment** | Silence/Hold hints. | Excluded from the current V1 implementation. The `enableSilenceHoldHintsTab` flag is a necessary gate for any future inclusion, but the current followed aggregate does not prove that a hint was delivered for that conversation. Revisit only after a per-conversation delivered-hint/use link is verified. |
| **Excluded from this metric's current scope** | Reminder hints, Customer Snapshot, and exposure-only signals such as sent hints, shown GenAI answers, or generated summaries. | No qualifying active-use rule has been approved for these items. They are not automatically V2 features. |

Some V1 wording details remain open for PM review, particularly whether KB searches from every initiation path and Guided Workflow opens from hints or step navigation qualify. The new API must use the current feature predicates as its implementation reference until a separate decision deliberately changes an existing metric.

## What this metric must count

The denominator is the current Assistance page's message-bearing eligible population: basically, conversations with messages. The query is matching `conversation_d` and `message_d` rows for an eligible `(conversation_id, agent_user_id)`, after its user/ACL, source/bot, dev-user, time, and other applicable filters. Conversations with no assistance use stay in the denominator. Selection and DAILY/WEEKLY buckets use **conversation start time**, which the current Assistance page and audited V1 feature requests already use.

The numerator is the subset with at least one approved V1 use signal. Multiple events or assistance types in one conversation must not inflate it. Keep `(conversation_id, agent_user_id)` through selection and use attribution: an agent gets credit only for their own use. Agent rows count distinct conversations per agent. Team rows follow the existing page's sum of member-agent counts, so a shared conversation can contribute twice within a team and an agent in overlapping teams contributes to each. The ungrouped overall and top-level response totals count distinct **conversation IDs** across eligible agents, independent of the requested grouping. A Team sum is therefore not necessarily the overall total.

Existing feature SQL/code is the source of truth for each current per-feature count or use predicate, subject to the known discrepancies below. The release scope above controls which of those signals contributes to this API.

## Why the existing API responses cannot be aggregated

Basically, current API responses don't have aggregation at conversation level. So they give no knowledge of the assistance-usage-per-conversation for us to do a higher level aggregation.

For example, one conversation can contain a used Suggestion and a KB search. The two RPCs can each report `1`; adding their counts yields `2`, but the new numerator is `1` conversation. Even a `total_conversation_count` field in a feature response does not identify which conversations were used: the KB and Suggestion queries count conversations with **any indexed event**, while HintStats counts conversations with sent hints. GenAI has no comparable conversation total and groups used knowledge sessions by agent and time. Notes and Suggestions count distinct events; Guided Workflow counts sessions; Summary counts conversations under its own rule.

The public RPC handlers also combine query construction, execution, aggregation, and response formatting. There is no shared feature-use function returning eligible `(conversation_id, agent_user_id)` rows that a new RPC could call. Calling a handler internally would return the same lossy aggregate response. To make that work, we would first have to change **each relevant feature query and response** to expose keys or introduce a reusable lower-level query interface, then migrate existing callers. Returning raw conversation IDs through public RPCs would also add payload and contract surface that those pages do not currently need.

## Alignment gaps that make reuse more expensive

These are source-level differences, not measured customer impact. They matter because a new percentage needs one numerator/denominator population; it cannot assume the current feature response totals share one cohort.

| Area | What I found | Consequence for this metric |
|---|---|---|
| Conversation root | `RetrieveConversationStats` requires a matching same-agent conversation and message. Feature APIs root at sent hints, events, conversation rows, or agent/time message groups. | Their `total_conversation_count` fields are not interchangeable with the page denominator; a feature record is not automatically an eligible conversation/agent key. |
| Source and dev users | ConversationStats permits sources 0/8 and profile-enabled 7 and excludes dev users. GenAI uses 0/8; HintStats uses the profile bot rule but also permits NULL source and an `IncludeCrestaBot` override. Several other feature queries have no explicit source or dev-user predicate. | The ConversationStats **source predicate** covers source-restricted 0/8 rows and profile-enabled 7, but this does not prove a superset of whole feature populations. The new metric intersects use evidence with its own eligible cohort. |
| Agent attribution | Some event/conversation joins use conversation ID alone; GenAI's existing aggregate joins use moments to message groups by agent/time. | Aggregate results cannot reliably answer which eligible agent used assistance in a shared or transferred conversation. The new queries retain agent and conversation keys. |
| Filter lowering | User/ACL resolution is shared, and the Assistance page uses conversation start time across these requests. Other filters are applied to different tables; unsupported columns and metadata joins can change which rows survive. | Shared request fields do not guarantee matching selected records. We reuse one eligibility filter path, then apply feature predicates inside it. Value-or-missing parser work is outside this decision. |
| Aggregation | Features count distinct actions, events, sessions, or conversations; group results commonly sum per-agent aggregates. HintStats sent branches use `MAX`. | Adding feature totals or averaging bucket rates cannot produce a distinct-conversation period percentage. |
| Known predicate defects | Summary's current stored data types `1/2` include edit and miss single-note copy relative to copy-only wording. Notes and Guided Workflow metadata branches use event enum values from the wrong physical cluster. | Do not present copied SQL as proven product correctness. Preserve or explicitly document current semantics, and handle the defective metadata paths as separate parity decisions. |

The remaining population differences are **not** solved by observing that time and some source predicates align. ConversationStats is an reasonable base. We can have following tickets to update existing APIs to use this conversation base.

## Options considered

| Option | What it requires | Decision |
|---|---|---|
| **A. New API calls the seven existing RPCs internally** | Their current responses still lack conversation/agent membership. Make each query and response expose that membership, or extract internal key-returning functions and migrate the RPCs first. Also reconcile population/filter and attribution differences. | **Defer.** Reusing aggregate responses is mathematically insufficient; creating reusable internals across seven APIs is a broad refactor of existing behavior. |
| **B. Frontend calls the existing RPCs and combines responses** | The same per-conversation data and API changes as A, plus transfer of those keys to the browser and client-side deduplication/attribution. | **Do not choose.** Current responses cannot produce the union. Adding key-heavy public responses, multiple page requests, and frontend set logic moves consistency and performance concerns into Director without removing the backend work. |
| **C. Dedicated API with one cohort and focused feature queries** | Reuse or extract ConversationStats' internal eligibility selection. Reproduce only the V1 qualifying-use predicates in small query functions; return conversation/agent keys internally, intersect and deduplicate, then aggregate. Instrument and measure the new query path. | **Choose for this ticket.** It provides one coherent contract and keeps changes to existing feature RPCs out of the critical path. The cost is duplicated predicate logic and drift risk until later extraction. |

The ideal longer-term shape is shared, key-returning internal feature-query functions used by both the existing feature RPCs and this API. It is a separate migration because the present handlers do not expose that layer and their current result semantics have unresolved discrepancies. We should not silently change seven existing dashboard metrics while adding one new metric.

## New API and implementation path

The prepared protobuf contract is `AnalyticsService.RetrieveConversationAssistanceUsageStats` in `cresta/v1/analytics/analytics_service.proto`, with result and bucket messages in `cresta/v1/analytics/conversation_assistance_usage_stats.proto`. Its HTTP route is:

```text
POST /v1/{parent=customers/*/profiles/*}/conversationAssistanceUsageStats:retrieve
```

The request accepts the normal attribute and time filters, optional DAILY/WEEKLY frequency, and at most one grouping: overall, Agent, or Group/Team. The response carries period total and assisted counts, grouped results, and optional start-time buckets. Director computes the percentage; a zero denominator displays N/A. The dedicated proto and generated Go outputs have been prepared and validated locally, but the backend handler and Director integration are still planned.

The intended server flow is:

```text
request filters + user/ACL resolution
       ↓
eligible (conversation ID, agent ID, start-time bucket) keys
       ↓
V1 use queries: hints | GenAI | Summary | Notes | KB search | workflow | Suggestions
       ↓
union/deduplicate use keys → intersect with eligible keys
       ↓
Agent / Team / overall period and bucket counts → one response
```

Feature queries will be small, separately instrumented functions using the existing use predicates. Start with bounded concurrency; cancel sibling work and fail the request if a required query fails, rather than returning an incomplete count as 0. Limit evidence to the requested start-time range. Keep IDs internal, and choose an efficient intersection strategy after measuring key volume. Multiple separate reads may not share one ClickHouse snapshot, so exact cross-query freshness remains a validation concern.

**Performance is a measurement task, not an assumed advantage of smaller SQL.** Separate queries can repeat scans of the same `conversation_event_d` table; a single combined query might later be better. Measure representative date ranges, cohort size, metadata cases, latency (including p95), rows/bytes read, Go/ClickHouse peak memory, transferred keys, and concurrent load. Consolidate by physical source or use database-side intersection only if the measurements justify it.

## Follow-up tickets to propose

1. **Align the base conversation population across assistance RPCs.** Inventory each RPC's conversation/message root, source/bot/dev-user and agent rules, and filter outcomes. Quantify mismatched eligible conversation/agent keys on representative customer data before choosing which existing metrics to change. Preserve current metric behavior until reviewed.
2. **Resolve other semantic mismatches.** Notes and Guided Workflow metadata event-enum defects; GenAI agent/time attribution; hint sent-branch aggregation; and table-specific filter behavior. Each change needs a before/after contract and customer impact check.
3. **Extract reusable internal key-returning query functions.** After semantics are settled, return typed `(conversation_id, agent_user_id, start_time, feature)` rows or an equivalent database-side relation from shared helpers. Migrate existing feature RPCs deliberately and let future union metrics reuse those helpers. Keep public aggregate responses stable unless a concrete consumer needs new fields.

These are proposed follow-ups, not tickets already created. They are valuable cleanup, but coupling them to this metric would turn one new reporting API into a multi-RPC behavior migration. No production data or latency measurement has yet established the size of the population gaps or the optimal query layout.
