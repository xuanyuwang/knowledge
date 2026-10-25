# CONVI-7777 — Optional conversation-use details

**Primary source repo:** `/Users/xuanyu.wang/repos/cresta-proto`.
**Branch/worktree:** `xw/convi-7777-assistance-usage` at `/Users/xuanyu.wang/repos/cresta-proto-convi-7777-assistance-usage`.
**Context:** Open protobuf PR #10283. User approved the refined draft and requested implementation in the existing protobuf PR.

## Request

Expose exact eligible conversation IDs and which assistance features were used when an optional request bool is true, to make API verification easier without enlarging normal production responses.

## Approved contract

- Request: `bool include_conversation_usage_details = 10`, default false.
- Response: `repeated ConversationAssistanceUsageDetail conversation_usage_details = 4`, omitted unless requested. Flat response-level records are independent of grouping, avoiding duplication across groups and buckets.
- One detail per eligible conversation ID: conversation ID and all eligible participating agents. Include zero-use conversations so the denominator can be verified.
- Agent detail: agent user ID and a deduplicated list of qualifying-use features. Team names are omitted per user feedback. Empty feature list means this eligible agent had no qualifying use.
- Current contract: use a dedicated `AssistanceUsageFeature` enum for only the ten supported categories. The earlier hint/event/moment oneof was superseded by user feedback because its value boundaries were too broad. See [enum revision](codex-convi-7777-feature-enum.md).
- Derive details and counts from the same filtered cohort and evidence, with the same authorization. Preserve agent attribution rather than only attaching a union of features to the conversation.
- Overall denominator = detail count; overall numerator = details with any agent's nonempty feature list. Agent rows count their detail participations; Team rows sum member agents. Start timestamps are omitted per user feedback; an investigator can retrieve them using conversation IDs when checking time buckets. They remain internal inputs to aggregation.
- No raw payload/transcript or additional event identifiers are proposed. Mark details logging DISABLED in proto.
- Diagnostic mode needs an explicit size/completeness policy: no silent truncation or sampling. Initial recommendation is a bounded opt-in diagnostic response that errors explicitly if a documented limit is exceeded; pagination can be designed if broad-range inspection is required. Normal aggregate requests should not incur the serialization/transfer cost of details.

## Historical source enum audit (wrapper superseded)

- `cresta/v1/analytics/common.proto:770`: Attribute.HintType distinguishes behavior, KB, GW, and checklist hints.
- `cresta/v1/conversation/event.proto:236`: ConversationEvent.EventType includes used Suggestions, KB search/document, Summary, and Notes.
- `cresta/v1/moment/moment.proto:212`: Moment.Type.KA_GEN_AI_REPLY_USED is the GenAI detected-use signal.
- Action.Type models offered/generated actions, not the complete use-category taxonomy. Dashboarding AgentAssistanceForChart has a partial feature oneof, not a complete reusable feature enum.

## Implementation steps

1. Fast-forwarded the existing worktree to remote formatting commit `d35f5ae8de`, preserving the prior PR change.
2. Added opt-in usage details to protobuf [PR #10283](https://github.com/cresta/cresta-proto/pull/10283), commit `e15a2680c6c6da2f693ce28c27947125353a8b58`. Request field `include_conversation_usage_details = 10` defaults false; response field `conversation_usage_details = 4` contains one record per distinct eligible conversation and its eligible agents, including no-use entries. Each agent has deduplicated qualifying features represented by existing hint/event/moment enums. Team names and start timestamps are omitted. Details have logging disabled and must be complete, without sampling or silent truncation. Backend serialization and enforcement remain pending.
3. Scoped Buf build, lint, compatibility against prior PR head `d35f5ae8de021894fbd613c51ebd3c1ddcc39d7a`, Bazel `analytics_go_proto` compilation, clang-format 22.1.8 on the new stats proto, and `git diff --check` passed. Generated outputs were validated locally and excluded from the commit per the PR template.
4. Updated the PR description, implementation plan, construction record, and interactive report.

## State

Protobuf change implemented and pushed; backend/frontend replacement remains pending. HTML JavaScript syntax and all five steps/control paths were checked using a simulated DOM; visual rendering remains unverified due to the previously recorded browser restriction. Tools used their configured authentication; no AWS, Okta, Azure, or SSH credential/profile was explicitly selected or read.

Final remote verification: PR #10283 head is `e15a2680c6c6da2f693ce28c27947125353a8b58`; source worktree is clean. The PR contains the two protobuf sources and existing Bazel registration only. Remote CI was still running at handoff.

Subsequent revision: Replaced the `AssistanceUsageFeature` wrapper with a dedicated enum in [PR #10283](https://github.com/cresta/cresta-proto/pull/10283), commit `f3ed4fabb8ec019a04ab6740df9bd7948a859092`. `used_features` remains field 2 and now contains deduplicated enum values. The enum exposes only ten scoped categories: behavioral/checklist/KB/workflow hints, GenAI answers, Summary, Notes, KB search, workflow document use, and Chat Suggestions. The three chat suggestion origins map to one category. Regular hints, Smart Compose, Silence/Hold, and email have no enum entries. The zero value is unspecified and must never be returned as qualifying use. See [enum session](codex-convi-7777-feature-enum.md).
