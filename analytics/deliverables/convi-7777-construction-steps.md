# CONVI-7777 — Dedicated API construction

**Current status:** Backend Phase I Step 1 is implemented: complete eligible-population builder, ConversationStats aggregate wrapper, and real ClickHouse regression coverage. Focused Go verification passes; awaiting user review before Step 2. Changes are uncommitted on `xw/convi-7777-step-1-population`. Branch preparation is outside the numbered construction report. Historical protobuf numbering below is retained as history; the active backend report starts at the shared-builder Step 1.

**October 15 publication:** Confirmed the contract follows existing statistics API structure and opened [protobuf PR #10283](https://github.com/cresta/cresta-proto/pull/10283), commit `b58ba1a54d8b8c98c18d309d65795d103af8eb96`, based on refreshed main. The PR includes only `analytics_service.proto`, `conversation_assistance_usage_stats.proto`, and `BUILD.bazel`; CI owns generated code. Buf build/lint/compatibility against origin/main and Bazel generated-Go compilation passed after rebasing. Backend/Director replacement remains pending. [Session](../sessions/2026-10-15/codex-convi-7777-protobuf-pr.md).

## October 15 — Backend Phase I, Step 1 plan

The user supplied a shared-population-builder plan: own the complete eligibility pipeline, including metadata joins, and return one conversation/agent row carrying selected conversation time and aht_sec before final aggregation. ConversationStats wraps this relation for existing counts/handle times/buckets; AssistanceUsage projects start-time keys for later evidence intersection. SQL, arguments and external tables travel together; user/ACL resolution remains before the builder. Preserve filter-before-deduplication semantics, conversation-ID-only metadata joins, branch-specific checks and MAX handle time over matching messages. [Step 1 contract](convi-7777-backend-step-one-contract.md) · [Interactive report](convi-7777-assistance-usage-construction-review.html). Branch cleanup and trying latest origin/main are deferred until October 16, 2026, outside the construction report. No source/branch update was performed.

## October 15 — Optional verification details

Added opt-in usage details to protobuf [PR #10283](https://github.com/cresta/cresta-proto/pull/10283), commit `e15a2680c6c6da2f693ce28c27947125353a8b58`. Request field `include_conversation_usage_details = 10` defaults false; response field `conversation_usage_details = 4` contains one record per distinct eligible conversation and its eligible agents, including no-use entries. Each agent has deduplicated qualifying features represented by existing hint/event/moment enums. Team names and start timestamps are omitted. Details have logging disabled and must be complete, without sampling or silent truncation. Backend serialization and enforcement remain pending.

Scoped Buf build, lint, compatibility against prior PR head `d35f5ae8de021894fbd613c51ebd3c1ddcc39d7a`, Bazel `analytics_go_proto` compilation, clang-format 22.1.8 on the new stats proto, and `git diff --check` passed. Generated outputs were validated locally and excluded from the commit per the PR template.

Construction sequence: add the request switch, add flat conversation/agent evidence messages, reuse public enum types through a feature oneof, then validate and publish the additive contract. Normal aggregate calls leave the detail list empty. Later backend work must derive details and counts from the same intersected cohort, map stored enums explicitly, and fail when complete requested details cannot be returned.

## October 15 — Bound features to this API

Replaced the `AssistanceUsageFeature` wrapper with a dedicated enum in [PR #10283](https://github.com/cresta/cresta-proto/pull/10283), commit `f3ed4fabb8ec019a04ab6740df9bd7948a859092`. `used_features` remains field 2 and now contains deduplicated enum values. The enum exposes only ten scoped categories: behavioral/checklist/KB/workflow hints, GenAI answers, Summary, Notes, KB search, workflow document use, and Chat Suggestions. The three chat suggestion origins map to one category. Regular hints, Smart Compose, Silence/Hold, and email have no enum entries. The zero value is unspecified and must never be returned as qualifying use.

Buf build/lint, compatibility against `origin/main`, Bazel generated-Go compilation, clang-format 22.1.8, and diff checks passed. Replacing message values with enum values intentionally changes the wire/JSON representation from the earlier unreleased PR draft. Backend feature mapping remains pending. The preceding optional-details wrapper is superseded by this enum; the rest of the details contract is retained.

## Revised implementation steps

1. **Define the dedicated protobuf contract (review checkpoint).** Replace the unpublished ConversationStats extension with `RetrieveConversationAssistanceUsageStats`, its request/response, and a dedicated stats proto. Request filters follow the existing eligible message-bearing population. Selection and buckets use conversation start time. Return both denominator and assisted counts for period totals and optional DAILY/WEEKLY buckets; percentages are derived by the client, with zero denominator rendered N/A. Agent results count distinct conversations per agent; Team results sum agents, including shared conversations and overlapping membership. Overall results and response totals count distinct conversation IDs, regardless of requested row grouping. Use belongs to the eligible agent who performed it. The HTTP route is `/v1/{parent=customers/*/profiles/*}/conversationAssistanceUsageStats:retrieve`; auth follows the Assistance stats role/page precedent. Updated Bazel sources and generated scoped Go messages, gRPC, gateway, validation, resource-name, and SQL outputs.
2. **Extract the eligible base population.** Reuse ConversationStats selection in an internal function returning conversation ID, agent ID, and conversation-start-time/bucket keys. Preserve ACL, source/bot, metadata, and message eligibility.
3. **Query feature use through small functions.** Reuse existing V1 predicates, obtain conversation/agent keys per feature, and run with bounded concurrency. Intersect with the cohort and union feature keys; fail the request on required-query failure. Keep Silence/Hold, chat Smart Compose, and email outside V1.
4. **Aggregate and wire Director.** Apply the chosen Agent/Team/overall grains to period and bucket counts. Add the adjacent leaderboard column and support the existing chart trend using the dedicated generated client. Remove the prior opt-in/type bridge while preserving existing metrics.
5. **Validate and tune.** Test attribution, shared conversations, duplicate evidence, zero denominator/use, filters, and cancellation. Measure smaller-query latency/load/memory before deciding on consolidation. Update this record and report around the final integrated implementation.

**Verified for Step 1:** `buf build`, scoped `buf lint`, scoped `buf breaking --against '.git#ref=HEAD'`, `bazel --bazelrc=local.bazelrc build //cresta/v1/analytics:analytics_go_proto`, and source diff checks. These verify protobuf validity/compatibility and generated Go target compilation, not backend behavior. Non-Go client generation/publication remains part of integration. No product runtime queries, customer data, or credentials were used.

[Implementation plan](convi-7777-dedicated-api-implementation-plan.md) · [Current interactive report](convi-7777-assistance-usage-construction-review.html)

---

## Historical first pass — superseded


**Status:** In construction, October 12, 2026. These are implementation steps, not historical commits unless a commit is cited. Backend base `go-servers` `8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498`; proto base `438363bc3866042d701a8fc50198fe87d2b21394`; Director base `cd7b46d052c694ec6cd689b8f6ab9f01e9690104`.

## Purpose and settled scope

The Assistance Used leaderboard needs a percentage of its filtered conversations where any approved V1 assistance was used. The denominator matches the current `RetrieveConversationStats` message-bearing population, including its source/bot and user filters. For Team rows, the existing page sums per-agent conversation counts, so the new count uses the same agent–conversation grain. Silence/Hold, chat Smart Compose, and all email assistance are outside V1. Current per-feature SQL/code defines use predicates; Summary therefore uses the current stored 1/2 predicate until an existing-RPC semantics change is approved. A sent hint alone does not qualify.

The construction uses an opt-in `include_assistance_usage` extension of `RetrieveConversationStats`. This keeps the existing denominator, ACL, group membership, frequency, and response path together, and avoids an independently filtered denominator in a new RPC. The default remains false to avoid extra query work for existing callers. The result exposes a count at the same response, group, and time-bucket levels as the existing conversation count. A response marker, `assistance_usage_included`, distinguishes a computed zero from an older server that ignores the opt-in. Director computes the ratio from group totals and renders N/A for a zero denominator or an absent marker.

## Construction sequence

### Step 1 — Expose an opt-in count in the existing contract

**Problem:** The current API returns eligible conversation counts but no count of conversations with qualifying assistance. **Starting state:** `RetrieveConversationStatsRequest` has standard filters and `ConversationStats`/result/response contain `total_conversation_count`. **Decision:** Add a request opt-in, corresponding output counts, and a computed-response marker without changing default behavior. **Why now:** It creates a narrow contract for the backend and Director; a separate RPC could drift from the current cohort. **Change:** Proto source fields in `analytics_service.proto` and `conversation_stats.proto`:

```proto
bool include_assistance_usage = 12; // request, default false
int32 conversations_with_assistance_used_count = 8; // response/group
int32 conversations_with_assistance_used_count = 5; // frequency entry
bool assistance_usage_included = 9; // response marker
```

**Resulting state:** A caller can request the new count on the same population and distinguish a computed zero from an unsupported backend. **Next problem:** The backend must identify feature use per eligible conversation.

### Step 2 — Build feature evidence at conversation/agent grain

**Problem:** Seven feature RPCs report actions, events, sessions, or different conversation populations, so their totals cannot be summed. **Starting state:** `conversationStatsClickhouseQuery` and its metadata variant count only message-bearing eligible conversations. **Decision:** Add a conditional ClickHouse evidence CTE using current feature-use predicates for V1 hints, GenAI, Summary, Notes, KB search, workflow document use, and chat Suggestions. Restrict evidence to the requested conversation-start interval and deduplicate `(conversation_id, agent_user_id)`. **Why now:** Feature evidence needs its own meaning before it can join the cohort; its union must not copy the separate RPCs' population roots. **Change:** `buildConversationAssistanceUsageQuery` adds `valid_assistance_summaries` and `assistance_used` CTEs. Its three branches read positive moments, use events, and summary events joined to valid generated summaries. Notes and Guided Workflow use conversations-cluster event types 17 and 14. GenAI keeps its 0/8 source rule; hints keep their source/bot rule. Sent-only hints do not qualify. **Resulting state:** A bounded set of qualifying conversation/agent keys exists, with overlapping features reduced to one key. **Next problem:** Evidence must be intersected with the eligible cohort, including metadata filters.

### Step 3 — Count use inside the existing cohort and aggregate it

**Problem:** Evidence rows can be orphaned, filtered out, or repeated. **Starting state:** The ConversationStats SQL has a message/conversation cohort and optional metadata joins, but no use count. **Decision:** Left join deduplicated evidence to that cohort and count distinct eligible conversation IDs in each agent/time group. Carry the count through response conversion and the existing Team aggregation, which sums per-agent counts. Set `assistance_usage_included` on successful and authorized-empty responses. **Why now:** Joining after cohort filtering prevents feature-specific source populations from redefining the percentage; the marker prevents an unsupported backend from appearing to report 0%. **Change:** Optional SQL fragments, scan fields, and aggregation fields. Core shape:

```sql
FROM eligible_conversation_rows
LEFT JOIN assistance_used
  ON used_conversation_id = eligible.conversation_id
 AND used_agent_user_id = eligible.agent_user_id
SELECT COUNT(DISTINCT eligible.conversation_id) AS convo_count,
       COUNT(DISTINCT IF(used_conversation_id <> '', eligible.conversation_id, NULL)) AS assisted_convo_count
```

The feature evidence is bounded by conversation start time. Opt-in requests that use conversation-ended time are explicitly rejected until an end-time evidence strategy is defined; the Assistance page uses start time. **Resulting state:** Numerator and denominator share the existing page's grain and filters for supported requests; use count cannot exceed eligible count within a group. **Next problem:** The page must request and display it.

### Step 4 — Connect the Assistance Used leaderboard

**Problem:** The page only receives current conversation totals and feature action metrics. **Starting state:** `useConversationStats` fetches the page's total; leaderboard rows have no conversation-assistance percentage. **Decision:** Set the request opt-in on the Agent and Team total-conversation calls only, transform the new count and marker, and add a sortable/exportable percentage column. Compute percentage points from period totals, not daily-rate averages; show N/A for a zero denominator or unsupported backend and 0% when a supported backend computes zero use. **Why now:** The backend contract is available and existing Powered by AA/Overall Engagement columns can remain intact. **Change:** Director API request/transform, row mapping, column, and CSV header. `@cresta/web-client` 2.30.77 lacks the new generated fields, so a narrow structural type bridge is used until proto publication. Essential mapping:

```ts
useConversationStats({ ...insightsRequestParams, includeAssistanceUsage: true });
row.conversationsWithAssistanceUsedCount = response.assistanceUsageIncluded
  ? group.conversationsWithAssistanceUsedCount : undefined;
row.conversationsWithAssistanceUsedPercent = row.numOfConvos &&
  row.conversationsWithAssistanceUsedCount !== undefined
    ? (row.conversationsWithAssistanceUsedCount / row.numOfConvos) * 100
    : undefined;
```

**Resulting state:** Both tabs can display and export the new percentage. **Next problem:** Review the combined path and validate against real data and tests.

### Step 5 — Reconstruct and review the combined behavior

**Problem:** A plausible sequence does not prove the final behavior. **Starting state:** The implementation spans proto, backend, and Director. **Decision:** Create a standalone interactive HTML walkthrough with each step's problem, starting state, decision, rationale, essential change, resulting state, and next problem; inspect the final diff and record evidence limits. **Why now:** It gives the user a reviewable causal account before testing. **Change:** An HTML deliverable and source trail. **Resulting state:** The implementation is ready for semantic review. **Next problem:** Focused tests, generated-client integration, customer coverage, and latency validation remain after this requested first pass.

## Verification boundary for this pass

Per user request, defer tests while constructing these steps. Do not claim passing tests, deployed parity, historical event coverage, or query latency. Check source diffs and static structure; record compilation limits caused by unpublished proto generation. End-time opt-in requests are unsupported in this pass. No AWS, Okta, Azure, or SSH credential/profile was explicitly selected.
