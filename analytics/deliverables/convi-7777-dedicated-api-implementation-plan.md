# CONVI-7777 dedicated API implementation plan

**Goal:** Add conversation-level assistance-use percentages beside "Convos powered by AA %" on Agent and Team leaderboards and expose an overall trend through the existing assistance-used chart.

**Architecture:** A dedicated `RetrieveConversationAssistanceUsageStats` RPC owns the metric. Reuse internal ConversationStats eligibility logic to obtain conversation/agent/start-time keys; query feature-use evidence through small functions; intersect/union keys in the server before aggregation. Query execution starts with bounded concurrency and complete-request failure on required evidence errors.

**Spec:** [Settled discussion and source audit](../sessions/2026-10-13/codex-convi-7777-split-query-design.md), supplemented by the aggregation contract below.

## Current execution contract

On October 20, the user authorized checking prerequisites, rebasing PR #34044 if needed, detaching #33993 from stack #34002, and implementing the new API locally on top of #34044, followed by an implementation report. Backend Phase I remains counts only; conversation usage details and Director are outside this implementation. The earlier population-only response is the starting checkpoint, not the completed API.

The checked prerequisites are the generated dedicated RPC/response contract, shared eligible-pair population, reusable row queries for all seven feature families, tenant-specific conversations/events connections, existing ACL/user/group resolution and metadata timezone/week rules. #34044 supplies Summary/GenAI; current main supplies the other feature helpers. The local #34044 branch is rebased onto `384cd3fda9c429c418a860c79a7352c618a8daf9`, with head `e40609a8aaa01a034fa448d9b9870eb3cb95d12f`. The API branch is based on that head.

Implementation order: retain resolved users/groups alongside eligible rows; adapt shared feature relations to distinct conversation/agent evidence restricted by an external eligible-pair table; execute seven required queries with concurrency three and fail the request on errors; intersect evidence and aggregate overall/Agent/Team period totals and DAILY/WEEKLY buckets; verify predicates and error paths, then show the construction report. Existing feature APIs and predicates remain unchanged. Notes and Guided Workflow use their events-cluster predicates; metadata eligibility is applied once by the population, rather than re-entering legacy feature metadata branches. A details request fails explicitly with Unimplemented while details are out of scope.

## Current result

Backend Phase I counts are polished and published to #33993 at `9cd953dddc7319f5fedac1e99e9efa5cb0baf9c3`. The query revision passed the full Go93 and focused race52 checks, plus a two-node/two-shard synthetic run of all seven families. The requested two-file consolidation then passed a fresh full API Bazel run (37.2s). Both #34044 frozen suites pass after rebase. #34044 CI retry is green; CI for the latest API head must be checked separately. [Implementation report](convi-7777-api-implementation-review.html) and [execution record](../sessions/2026-10-20/codex-convi-7777-api-implementation.md). #34044 and the complete counts implementation are pushed; the API worktree is clean. No merge. Production benchmarking and details remain follow-up.

## PR stack strategy

- Step 1 is one independently reviewable PR for the shared population builder and its regression coverage.
- Step 2 packaging remains open. Prefer smaller feature-family PRs when each can introduce a usable shared query and its tests without duplicating shared scaffolding; decide exact boundaries from source and diff size. The user also considers one larger PR, so this is a recommendation rather than a settled split.
- Steps 3 and 4 may share one PR if their combined implementation remains reviewable. Decide after inspecting the concrete change size, not by step numbering alone.
- October20 authorization completes backend counts locally on #34044 and detaches #33993 from the old stack. The population-only checkpoint is superseded locally; the new implementation remains uncommitted for review.

## Contract and constraints

- Agent: distinct conversations per eligible agent; use must belong to that agent.
- Team: sum agent counts, including shared conversations within a team and agents appearing in multiple teams. Only a using agent's teams receive numerator credit.
- Ungrouped overall: distinct conversation IDs, with use by any eligible agent qualifying once.
- All selection and frequency buckets use conversation start time. Reuse metadata timezone/start-of-week rules.
- Preserve the existing message-bearing cohort, authorization/user filters, source/bot rules, and current V1 feature-use predicates. Silence/Hold, chat Smart Compose, and email remain outside V1.
- The completed API returns denominator and assisted counts together; the client computes percentage and displays N/A when denominator is zero. The user-approved population-only checkpoint returns only the distinct denominator; usage fields remain uncomputed until the evidence PRs.
- Reuse current ticket worktrees; replace ticket-specific prior work as needed, preserve unrelated edits. Protobuf PR #10283 is merged; recheck backend generated dependency after the branch refresh.

## Step 1 — Protobuf review checkpoint (completed)

- [x] Replace the unpublished ConversationStats opt-in fields with a dedicated RPC/request/response in `cresta/v1/analytics/analytics_service.proto`.
- [x] Add `cresta/v1/analytics/conversation_assistance_usage_stats.proto` for grouped period totals and frequency bucket counts; document counting grain explicitly.
- [x] Add the new source to `cresta/v1/analytics/BUILD.bazel`, build the analytics proto target, and regenerate scoped Go outputs.
- [x] Check proto lint/build and compatibility against the worktree HEAD, inspect the diff, and update construction records/report.
- [x] Stop at the user's requested protobuf review checkpoint before backend/frontend replacement.

### Optional usage details — approved and implemented in protobuf

Added opt-in usage details to protobuf [PR #10283](https://github.com/cresta/cresta-proto/pull/10283), commit `e15a2680c6c6da2f693ce28c27947125353a8b58`. Request field `include_conversation_usage_details = 10` defaults false; response field `conversation_usage_details = 4` contains one record per distinct eligible conversation and its eligible agents, including no-use entries. Each agent has deduplicated qualifying features represented by the dedicated ten-category `AssistanceUsageFeature` enum (updated in commit `f3ed4fabb8ec019a04ab6740df9bd7948a859092`). Team names and start timestamps are omitted. Details have logging disabled and must be complete, without sampling or silent truncation. Backend detail serialization remains pending; Phase I explicitly rejects details requests with Unimplemented.

Later-phase follow-up: build details from the same eligible conversation/agent keys and feature sets as counts; map qualifying stored evidence into this API's feature categories; keep empty feature lists for eligible agents without use. Verify default-off behavior, one record per conversation/agent/feature, count reconstruction, authorization/filter parity, and explicit failure when complete details cannot be returned. Start timestamps stay internal for buckets.

## Step 2 — Population and feature evidence (after protobuf review)

Replace the ConversationStats opt-in implementation with one RPC implementation/test file; put shared helpers in `utils.go` where consistent with the package. Extract the eligible raw-key selection without changing existing ConversationStats results. Wrap each feature-use query in a focused function. Keep conversation/agent keys until union/intersection and record feature-query timing/read volume for tuning.

Validate missing eligible messages, source/bot and dev-user exclusions, metadata predicates, duplicate feature evidence, zero-use cohorts, use by an ineligible agent, and transfer attribution. Query failure cancels siblings and returns an error, never a fabricated zero.

## Step 3 — Aggregation

Build period and time-bucket counts from intersected keys. Test one conversation shared by two agents on one team (Team denominator 2), two teams with only one using agent (credit only that agent's teams), overlapping membership (independent Team results), and ungrouped deduplication (denominator/numerator at most 1 per conversation). Top-level overall counts stay distinct regardless of requested row grouping.

## Step 4 — Director and release integration

Replace the temporary opt-in/type bridge with the dedicated generated client. Wire period results to the adjacent column and frequency results to the chart; preserve Powered by AA and Overall Engagement. Update pinned generated dependencies only after publication. Validate column sorting/export, 0% versus N/A, period ratios rather than averages of bucket percentages, and Agent/Team/overall responses.

## Step 5 — Performance and construction review

Measure representative date ranges, cohort sizes, metadata cases, concurrency, rows/bytes read, key transfer, and Go/ClickHouse memory. Start with smaller queries; decide whether to consolidate only from measurements. Rewrite the construction record and interactive report around the final implementation, including verified evidence and remaining deployment/history limits.

Latest review fix: `9cd953dddc7319f5fedac1e99e9efa5cb0baf9c3` forces external user tables for role/activity-restricted population reads even when the shared flag is off. Both large-directory regressions fail before the fix; all six restriction cases pass afterward (17.925s), and the full API Bazel target passes (37.3s). The inline evidence-agent-list concern remains separate.
