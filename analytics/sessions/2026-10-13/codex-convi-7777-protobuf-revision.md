# CONVI-7777 dedicated protobuf checkpoint — 2026-10-13

**Primary source repo:** `/Users/xuanyu.wang/repos/cresta-proto`.
**Branch/worktree:** `xw/convi-7777-assistance-usage` at `/Users/xuanyu.wang/repos/cresta-proto-convi-7777-assistance-usage`.
**Base:** `438363bc3866042d701a8fc50198fe87d2b21394`; working-tree edits, no new commit.
**Primary knowledge domain/subdomain:** analytics / assistance-insights.

## Authorization and scope

The user approved implementation of the dedicated API/smaller-query direction and reuse/override of the existing ticket worktrees, explicitly requesting protobuf first for their review. This turn stops at that checkpoint. No backend/frontend edits were made. Their existing first-pass opt-in code remains superseded and incompatible with the replaced unpublished proto fields until the next implementation stage.

Settled contract: Agent distinct participation and agent-attributed use; Team sums of agent counts, including shared conversations and overlapping membership; ungrouped overall distinct conversation IDs with any eligible agent's use qualifying once. Conversation start time supplies both population selection and buckets. UI scope: column beside Convos powered by AA % and trend support in the existing Assistance Used chart. V1/P2 feature scope unchanged.

## Construction

1. Added an implementation plan and replaced only the prior ticket's unpublished proto edits, restoring ConversationStats source and its modified generated message file to HEAD.
2. Added `RetrieveConversationAssistanceUsageStats`, HTTP route, request/response, and Assistance-page role/permission annotations. The request contains standard Attribute/TimeRange/Metadata/user-filter controls, one optional Agent/Team grouping, and optional frequency for period-only versus DAILY/WEEKLY buckets. No end-time switch is exposed.
3. Added `conversation_assistance_usage_stats.proto` for period and bucket counts. Every percentage can use matching denominator/assisted counts. Top-level overall counts are explicitly distinct independent of row grouping; Team results can differ from them. Zero denominator means unavailable percentage. Required-query failure cannot produce a successful partial response.
4. Registered the new proto in the analytics Bazel target and generated only analytics service and new stats Go files from the successful target build, including grpc/gateway/validation/resource-name/SQL outputs. Applied the repository's cresta_lite build-tag convention to gateway/SQL/validation outputs. Non-Go generation/publication is deferred to integration.
5. Updated the existing construction Markdown and HTML report around the dedicated API checkpoint; retained the first-pass Markdown construction as explicitly superseded history. Updated work item, README, and daily movement.

## Verification

- `buf build --path cresta/v1/analytics/analytics_service.proto --path cresta/v1/analytics/conversation_assistance_usage_stats.proto`: passed.
- Scoped `buf lint`: passed; existing DEFAULT lint-category deprecation warning only.
- Scoped `buf breaking --against '.git#ref=HEAD'`: passed.
- `bazel --bazelrc=local.bazelrc build //cresta/v1/analytics:analytics_go_proto`: passed, including generated Go compilation.
- `git diff --check`: passed in proto and knowledge.
- HTML embedded JS syntax and all four step/snapshot/change/navigation controls: passed in a simulated DOM. This is a static functional check, not visual rendering.
- CUA local-file browser open was rejected by the browser URL policy (file protocol not allowed). No alternate browser route or workaround was attempted; visual inspection remains unverified.
- Opened the new stats proto in Codex (tool returned queued).

No product runtime tests, deployment/customer/history/performance claims, customer data queries, AWS/Okta/Azure/SSH credential/profile selection or use, external mutation, commit, push, or PR.

## Next

User reviews the RPC/request/response and stats messages. Then replace the old backend/frontend pass with shared cohort extraction, small per-feature query functions, server-side set operations/aggregation, and generated-client UI integration. Enforce supported grouping/frequency and test semantic boundaries in that backend stage. Benchmark before query consolidation.
