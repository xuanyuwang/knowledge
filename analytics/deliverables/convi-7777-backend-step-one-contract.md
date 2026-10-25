# CONVI-7777 — Step 1: Share the eligible conversation population

**Status:** Implemented on `xw/convi-7777-step-1-population` in `/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage`, against base `40faeac75f2d55b06a4e3e29ac64fe782b8abd99` with cresta-proto v2.32.18. The shared builder and ConversationStats wrapper are complete; four focused Go test entry points pass (45 named nodes, zero failures, 73.107s). Gazelle generated the production source and focused test target; The generated Bazel test target passed in 44.1s with fresh test containers. Changes remain uncommitted for the user’s Step 1 review. Phase I remains backend counts only. This document preserves the approved design below; the [construction report](convi-7777-assistance-usage-construction-review.html) contains actual snapshots, the full Go diff and scenario evidence. Branch preparation is outside the numbered report; the superseded draft is preserved in stash `4dc871101b1a0c6ba4d19ffab69c2be652abb321`.

## Current state and problem

ConversationStats already selects eligible conversation/agent rows, but the selection is coupled to its final aggregation. The ordinary path intersects selected conversation rows with matching messages. The metadata path adds inclusion/exclusion joins and selected-or-missing and stale/latest rules. Returning aggregate counts loses the keys needed to intersect feature evidence for AssistanceUsage. Sharing only WHERE conditions would leave joins, deduplication, filter placement and time provenance duplicated between callers.

## Proposed change and reason

Extract one shared builder that owns the complete eligible population relation immediately before final COUNT/SUM/GROUP BY. Both ordinary and metadata cases become paths within it; absent metadata filters emit no metadata CTEs or joins. Each returned row represents one eligible `(conversation_id, agent_user_id)` pair within the selected customer/profile. Retain the existing per-pair handle time so ConversationStats does not duplicate message grouping or add a second message scan.

| Field | Meaning and consumer |
| --- | --- |
| `conversation_id` | Overall distinct-conversation counting and feature-evidence intersection. |
| `agent_user_id` | Agent attribution and later Team sums. |
| Selected conversation time | Time from the selected conversation/label row. Start time for AssistanceUsage; preserve ConversationStats' start/end selection option. The illustrative shared alias below is `conversation_time`. |
| `aht_sec` | Existing `MAX(handle_time_secs)` over matching message rows, retained for ConversationStats. AssistanceUsage need not return this field. |

The builder returns a complete SELECT plus ordered arguments and external user-filter tables. These artifacts travel together whether the query is executed or embedded as a CTE. Its inputs are independent of either RPC: resolved attribute filter, time interval, selected time field, bot policy, stale-metadata policy, and existing parser/query configuration where required. The customer/profile connection remains tenant-scoped. A context parameter and exact Go names/types will follow package conventions; the pasted interface is illustrative rather than a requirement to introduce a new abstraction layer.

User/ACL resolution remains before the builder. Callers retain the existing shared resolution and empty-selection guard. An empty authorized user population must never become an unrestricted empty user filter. The builder does not add frequency, Agent or Team aggregation.

## Eligibility pipeline owned by the builder

| Stage | Existing behavior to preserve |
| --- | --- |
| Select conversation rows | Apply time/user conditions before choosing the latest matching conversation/agent version. |
| Apply Agent Assist ON/OFF | Keep the existing conversation-label aggregation and anti-join behavior. |
| Select matching messages | Apply attribute/source/bot/developer/agent rules in their existing locations; group by conversation/agent and compute MAX handle time. |
| Intersect populations | A selected conversation/agent must have a matching message. |
| Apply metadata eligibility | Preserve inclusion, exclusion, selected-or-missing and stale/latest behavior, including conversation-ID-only metadata joins. |
| Project eligible rows | Emit conversation/agent, selected conversation time and aht_sec before caller aggregation. |

## How callers use the result

ConversationStats wraps the relation and retains its current agent/time grouping, distinct counts, handle-time sum and response conversion. Its existing grouped response and time-range behavior are regression boundaries, not redesigned in this step.

```sql
-- Explanatory design, not implemented SQL.
WITH eligible_population AS (
    -- shared builder's complete SELECT
)
SELECT
    <existing agent/time grouping>,
    COUNT(DISTINCT conversation_id),
    COUNT(DISTINCT agent_user_id),
    SUM(aht_sec)
FROM eligible_population
<existing GROUP BY>
```

AssistanceUsage projects only its required keys and start time from the same relation, selected with the start-time option:

```sql
-- Explanatory design, not implemented SQL.
SELECT conversation_id, agent_user_id,
       conversation_time AS conversation_start_time
FROM (
    -- same shared SELECT, configured for conversation start time
)
```

This establishes the internal projection/read path for the later AssistanceUsage orchestration. It does not wire the public RPC or add evidence, numerator or grouping behavior. If the internal projection can be verified without adding a production reader until its real caller exists, keep it at that boundary rather than introducing an unused framework. Only ConversationStats and AssistanceUsage are intended consumers; do not migrate other feature APIs.

## Invariants and concrete acceptance cases

These are the agreed acceptance criteria. The report distinguishes executed real-ClickHouse and response/snapshot tests from source-only checks and remaining limits.

| Invariant / case | Required result |
| --- | --- |
| Filter before deduplication | Retain latest matching version semantics; moving predicates after latest-row selection must be caught by a versioned-row fixture. |
| Selected conversation time | Read time from the selected conversation/label row, not from individual message copies. Preserve existing versioned-conversation bucket behavior. |
| Start and end modes | AssistanceUsage uses start time. ConversationStats continues to support its existing start/end mode, timezone and week rules. |
| Handle time | Messages with matching handle times 10 and 20, plus a filtered-out message at 90, produce aht_sec=20 rather than 90 or 30. |
| Message-bearing cohort | A conversation with no matching message is absent. A matching conversation without offered/used assistance remains eligible. |
| Shared conversation | c1/Alice and c1/Bob produce two rows. Later overall count is 1; each agent count is 1; shared Team count sums to 2. |
| Metadata attribution | Metadata joins still use conversation ID only; adding agent ID must not remove a previously eligible participation. |
| Metadata cases | Selected, missing, selected-or-missing, excluded and stale/latest cases retain current semantics. |
| Label branch differences | Preserve source/developer check placement across default/ON/OFF branches; do not impose a new uniform predicate. |
| Source and message rules | Preserve bot policy and message-level developer/nonempty-agent rules. |
| User filters | Keep resolved user filters and external tables with correct query argument order; limited access with no users returns early. |
| Time interval | Preserve [start,end), including lower/upper boundaries. |
| Existing aggregate behavior | Compare counts, handle times and buckets before/after extraction for ordinary and metadata cases; no extra message scan. |
| Failures | Query generation/execution errors remain errors; they cannot produce a fabricated zero result. |

## Resulting state and stop boundary

After implementation, one shared owner defines the population used by the existing aggregate and AssistanceUsage's internal projection. ConversationStats still returns the same counts, handle times and buckets. AssistanceUsage can retain keys for later evidence intersection without copying population SQL. Update the report with the actual diff, code snapshots, test results and remaining limits, then stop for user review.

Out of scope: qualifying feature queries, numerator/union, new Team/overall/bucket aggregation, public RPC wiring, optional usage details, Director, migration of other feature APIs, and speculative optimization. The existing superseded draft and dependency state are branch preparation, not a construction-report step. The five referenced handler/query/parser/user-filter/integration-test files are unchanged between inspected ticket HEAD and refreshed main; the source references therefore still describe the current production paths. Recheck state when preparing the first PR branch.

## Source trail and verification limits

The implementation executes the shared relation against real ClickHouse, including the key projection and existing aggregate wrapper. Existing versioned-conversation integration tests and ConversationStats SQL/response tests pass. Weekly buckets are source-checked, without a new weekly runtime case; production latency and load remain unmeasured. No AWS/Okta/Azure/SSH credential or profile was explicitly selected or read. See the October 16 Step 1 session for exact commands and scope.

- [Inspected ordinary and metadata query paths](https://github.com/cresta/go-servers/blob/8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498/insights-server/internal/analyticsimpl/retrieve_conversation_stats_clickhouse.go)
- [Existing versioned-conversation integration tests](https://github.com/cresta/go-servers/blob/8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498/insights-server/internal/analyticsimpl/retrieve_conversation_stats_conversation_version_integration_test.go)
- [Existing user/ACL resolution](https://github.com/cresta/go-servers/blob/8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498/insights-server/internal/analyticsimpl/common_user_filter.go)
- [Existing parser/external-table helpers](https://github.com/cresta/go-servers/blob/8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498/insights-server/internal/analyticsimpl/common_clickhouse.go)
