# CONVI-7777 — Backend Phase I, Step 0 baseline

**Status:** Baseline inspection complete; Step 1 has not started. User confirmed Phase I is backend only and corrected the current task to Step 0. No backend, proto, dependency, or Director files were edited in this step.

## Current state

- Backend worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage`, branch `xw/convi-7777-assistance-usage`, HEAD `8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498`.
- Existing uncommitted draft: two modified ConversationStats files (78 insertions, 17 deletions) and untracked `retrieve_conversation_assistance_usage_clickhouse.go`. This draft extends the old RPC with an opt-in flag, joins one combined assistance-use CTE, and adds assisted counts to existing converters. It is superseded by the dedicated API design; it is preserved untouched in Step 0.
- Exact tracked draft diff SHA256: `6d33c90d77c8886c22a0815a895d0b5824e7270c44e87155a084448280115c98`. Untracked query file SHA256: `f4162182f74f1c6ca566b7b6f365fba8090fce36a3ec1fa722535b2115336e99`.
- [Proto PR #10283](https://github.com/cresta/cresta-proto/pull/10283) is merged at reviewed head `f3ed4fabb8ec019a04ab6740df9bd7948a859092`. Its final optional details use the dedicated ten-category `AssistanceUsageFeature` enum. The earlier oneof-wrapper description is historical.
- Backend `go.mod` and `deps.bzl` both pin `cresta-proto/v2 v2.31.32`; cached generated analytics sources contain neither the dedicated RPC nor the superseded opt-in fields. A compatible generated dependency must be selected and synchronized before RPC wiring. No dependency was changed or local replace added; the current published version containing the API has not been identified.
- No dedicated RPC override exists in this worktree. `AnalyticsServiceImpl` embeds the generated unimplemented service. Source presence in the merged proto does not prove backend runtime support.

## Reusable production path

`RetrieveConversationStats` resolves user/ACL/group filters with `ParseUserFilterForAnalytics`, applies empty-selection handling with `ApplyUserFilterFromResult`, resolves bot inclusion, and parses standard filters. Its denominator starts from eligible `message_d` conversation/agent pairs and joins the `conv_with_aa` conversation selection. Without an Agent Assist filter, that selection includes filtered conversations regardless of whether assistance was offered. The message side excludes developer users and empty agent IDs and applies the source and request predicates. Both ordinary and metadata/moment query paths exist; preserving only the ordinary path is insufficient.

`buildAgentAssistFiltersSubQuery` provides conversation selection and the conversation time: the default/OFF branches choose the latest row per conversation/agent; ON combines label evidence and selects the latest conversation time with `argMax`. `conversationStatsClickhouseQueryWithMoment` additionally handles included/excluded moments and selected-or-missing metadata, with latest versus stale matching. Time used for bucketing comes from the conversation selection rather than each message row's copied time. Source policy is 0/8, plus 7 when bot inclusion is enabled. The existing parser defaults to conversation start time; the new API contract fixes this time basis.

Reuse internal filtering and selection, not the public aggregate response: the current final COUNT discards conversation membership. Keep existing ConversationStats behavior stable and avoid making the new metric depend on the superseded combined feature CTE.

## What Step 0 changes and why

Step 0 creates a pinned baseline, separates earlier draft work from accepted production logic, limits Phase I to backend counts, and makes Step 1 reviewable. Product behavior is unchanged. The prior protobuf report is preserved as `convi-7777-protobuf-construction-review.html`; the current interactive report now opens on the backend baseline.

## Proposed Step 1 contract

**Input:** Customer/profile scope, resolved eligible user selection, standard cohort attribute filters, `[start,end)` conversation-start interval, bot inclusion, and metadata stale/latest matching mode. User/ACL resolution remains an explicit prerequisite; a raw cohort reader must not be exposed as a public bypass. If extraction is limited to SQL helpers, distinguish parser/query proof from later handler-level authorization proof.

**Output:** Internal unique `(conversation_id, agent_user_id)` keys carrying canonical `conversation_start_time`. No public details serialization, feature-use query, numerator, Team aggregation, time-bucket aggregation, or Director changes. Keep raw start time for later bucket construction. Proposed names/types are not implemented yet.

**Change boundary:** Extract or share the final eligible selection from both existing query paths before COUNT. Prefer shared selection consumed by the legacy aggregate and new key reader, rather than independent copied predicates. Preserve latest/stale metadata behavior and the feature-flag-controlled query behavior already present. First preserve the superseded draft as a recoverable snapshot and remove only its ticket-specific opt-in changes; never mix them into the new accepted baseline. Do not advance to qualifying feature evidence in the same checkpoint.

**Completion evidence:** Query/argument assertions for filter composition, seeded ClickHouse key-set checks where execution is required to prove deduplication/joins, and existing ConversationStats regression checks. Compare distinct cohort IDs with ungrouped legacy period counts and distinct pairs per agent with legacy Agent counts. Do not equate the sum of Agent or bucket counts with overall distinct IDs. Include unchanged existing end-time behavior in legacy regression coverage even though the new key reader uses start time only. Record missing runtime/environment proof rather than labelling SQL shape as data parity.

## Acceptance scenarios for Step 1

All rows below are planned acceptance criteria, not executed test results.

| Scenario | Expected behavior |
| --- | --- |
| No assistance offered or used | An otherwise eligible conversation remains in the denominator. |
| Conversation without eligible messages | A conversation_d row alone does not enter the cohort. |
| Repeated messages and stored versions | Return one key per conversation and agent; use the conversation-selection start time, not one key per message/version. |
| Shared conversation | Conversation c1 handled by Alice and Bob produces (c1,Alice) and (c1,Bob). Overall denominator is 1; each agent denominator is 1; their shared team later sums to 2. |
| Scoped access and explicit users/groups | Reuse resolved eligible users and group memberships; excluded agents produce no keys. |
| Empty resolved selection | A limited caller with zero eligible users gets an empty cohort, never an unfiltered query. |
| Agent-only, peers, deactivated users | Preserve existing user-filter switches; verify at orchestration integration if Step 1 is SQL-only. |
| Source/bot/developer rules | Preserve source 0/8; source 7 only when configured; exclude developer and empty-agent message rows. |
| Start-time boundaries | Include start == lower bound and exclude start == upper bound; assistance event time cannot select the cohort. |
| Agent Assist ON/OFF | Preserve the conversation-label selection and anti-join behavior, including old and per-message label rows. |
| Metadata and moments | Preserve selected, missing, selected-or-missing, excluded, and stale/latest semantics on the metadata query path. |
| Other supported attribute predicates | Carry duration, use case, channel and other supported message predicates through the existing parser; do not silently drop unsupported input. |
| Existing ConversationStats | Existing request SQL/results remain equivalent after extraction; include both ordinary and metadata paths. |
| Dependency or query failure | Return an annotated error; never turn unavailable cohort data into a successful zero. |

## Phase I boundaries and pending decisions

Counts include period totals, optional daily/weekly buckets, and Agent/Team/overall results under the merged proto. There are ten feature categories: behavioral/checklist/KB/workflow hints, GenAI answers, Summary, Notes copy, KB search, workflow document use, and Chat Suggestions. Smart Compose, email, Silence/Hold, Reminder, and Customer Snapshot remain excluded. Existing feature-use predicates are the starting point; discrepancies are reviewed explicitly in each feature checkpoint. Feature-level parity is outside Step 0 and Step 1.

Conversation details and all Director integration are deferred. Explicitly rejecting `include_conversation_usage_details=true` in Phase I is a recommendation pending agreement, not an implemented or approved behavior. It does not block cohort extraction. Published proto version selection and the exact internal helper shape remain to be resolved during implementation. No merge or deployment recommendation is made at this baseline checkpoint.

## Verification and limits

Attempted `go test ./insights-server/internal/analyticsimpl -run '^TestRetrieveConversationStatsValueOrMissingMetadataFilter$' -count=1` in the existing ticket worktree. Compilation failed before tests ran because the superseded draft references absent proto fields/methods: `AssistanceUsageIncluded`, `GetIncludeAssistanceUsage`, and `ConversationsWithAssistanceUsedCount`. This is the existing baseline blocker, not a test failure caused by Step 0. No code was changed to repair it. Backend diff check passed, and the tracked draft diff plus untracked query file retained their recorded SHA256 hashes.

Source inspection, dependency pin comparison, merged PR status and scoped diff inspection were performed. No customer data, live database, performance benchmark, endpoint execution or deployment validation was performed. The Step 1 acceptance matrix is unexecuted. Source code and the existing uncommitted draft remain unchanged. GitHub checks used `gh` configured authentication; dependency downloads used the Go/Git tools' configured authentication. No AWS, Okta, Azure or SSH credential/profile was explicitly selected or read.

## Source trail

- [Pinned legacy handler](https://github.com/cresta/go-servers/blob/8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498/insights-server/internal/analyticsimpl/retrieve_conversation_stats.go)
- [Pinned cohort/query paths](https://github.com/cresta/go-servers/blob/8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498/insights-server/internal/analyticsimpl/retrieve_conversation_stats_clickhouse.go)
- [Pinned shared filter parser](https://github.com/cresta/go-servers/blob/8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498/insights-server/internal/analyticsimpl/common_clickhouse.go)
- [Pinned user/ACL resolution](https://github.com/cresta/go-servers/blob/8d6a2b0b3666bde1a6bcb44a1fbd3848f1dfb498/insights-server/internal/analyticsimpl/common_user_filter.go)
- [Merged contract](https://github.com/cresta/cresta-proto/blob/f3ed4fabb8ec019a04ab6740df9bd7948a859092/cresta/v1/analytics/analytics_service.proto)
- [Final feature enum and count semantics](https://github.com/cresta/cresta-proto/blob/f3ed4fabb8ec019a04ab6740df9bd7948a859092/cresta/v1/analytics/conversation_assistance_usage_stats.proto)
