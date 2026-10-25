# CONVI-7879: Conversation assistance usage tracing

- Primary domain: analytics.
- Primary subdomain: assistance-insights.
- Ticket: https://linear.app/cresta/issue/CONVI-7879/add-conversation-level-assistance-usage-tracing-in-assistance-insights
- Related ticket: CONVI-7777.
- Official status at creation: Backlog; assigned to Xuanyu Wang.
- Backend PR: https://github.com/cresta/go-servers/pull/34417 (open, ready for review).
- Technical status: ticket created; backend and frontend worktrees created; drawer design approved; backend and frontend implemented; backend Go/Bazel and frontend tests pass; frontend typecheck, lint, extraction and unused-file checks pass.

## Objective

Explain the conversation assistance usage metric with eligible conversation IDs, participating agents and deduplicated qualifying feature categories, including no-use entries.

## Source context

- Primary repo: /Users/xuanyu.wang/repos/go-servers.
- Backend worktree: /Users/xuanyu.wang/repos/go-servers-convi-7879-conversation-tracing.
- Backend base: origin/main at 6e15680fe22c1325825d1732a1ed4e28019795f0.
- Frontend worktree: /Users/xuanyu.wang/repos/director-convi-7879-conversation-tracing.
- Frontend base: origin/main at 4ef9b281fd16b21d0b5656811cbfa9d1ad919ecb.
- Branch in each repo: xw/convi-7879-conversation-tracing.
- Native worktree tool is bound to the parent workspace metadata repository and has no source-repo argument. Used git worktree add to create the explicitly requested worktrees in the required /Users/xuanyu.wang/repos directory.

## Approved design

Reuse the existing protobuf opt-in contract. Tag all ten qualifying features while retaining the existing evidence predicates. Build complete details per eligible conversation/agent only when requested, including zero-use participants; fail explicitly on evidence or response-size failures rather than silently truncating. Preserve ACLs, filters and aggregate counting grain.

In Director, clicking an Agent or Team's usage percentage opens a drawer with conversation links, eligible agents, feature labels and an explicit no-use state. Load details only when opened and the existing rollout flag is enabled and resolved; preserve the page filters and selected row. Handle loading, empty, error and changed-filter states. No raw-event timeline or new event instrumentation.

## Evidence

Freshly fetched backend main still rejects include_conversation_usage_details=true and retains only conversation/agent evidence keys. Director main preserves aggregate result groups only. Existing protobuf defines the details and ten-category enum. No new proto contract is currently planned.

## Validation and next actions

Both worktrees were verified clean at creation. User approved the drawer design. Backend opt-in details are implemented and the focused Go suite passes (43.814s), including real ClickHouse category fixtures, duplicate/no-use/multi-agent/grouping tests and explicit size/error failures. The direct ACL regression passed (19.405s) and the full focused Bazel target passed (44.9s; 378.596s build). Frontend is implemented against the existing contract; 36 app tests and nine API tests pass. Independent review fixes cover audience union semantics and partial membership-page errors. Synthetic browser verification confirms usable wrapping and populated/error/empty states. Final app typecheck, app/API lint, knip, translation extraction, Biome and whitespace checks pass. Temporary harness files removed; fixture server stopped. Backend committed/pushed as 229195655a and PR #34417 created October 23; frontend remains local and uncommitted. No deployment.

1. Review backend PR #34417 and the local frontend changes.
2. Live end-to-end verification and rollout remain for a later integration/deployment step.

## Timeline

- 2026-10-22: Created CONVI-7879 in Linear, assigned to Xuanyu Wang and related to CONVI-7777; prepared two clean worktrees from freshly fetched main. See [session](../sessions/2026-10-22/codex-conversation-assistance-tracing.md).

- 2026-10-22: User approved the drawer design. Backend keeps seven queries for ordinary aggregate requests and uses ten categorized queries for tracing (splitting hint evidence only). It stores deduplicated features per pair and serializes sorted eligible conversation/agent entries; responses over 4 MiB protobuf fail with ResourceExhausted and filter-narrowing guidance. Dedicated details requests may differ from an earlier table response as records change; no shared cross-query snapshot is promised.

- 2026-10-23: User requested backend PR. Fresh Gazelle/uncached Bazel verification passed; pushed commit 229195655a and opened [#34417](https://github.com/cresta/go-servers/pull/34417) against main. [Session](../sessions/2026-10-23/codex-conversation-tracing-backend-pr.md).
