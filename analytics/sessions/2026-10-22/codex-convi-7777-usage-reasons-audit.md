# CONVI-7777: per-conversation usage reasons

- Date: October 22, 2026 (America/Toronto).
- Primary domain: analytics / assistance-insights.
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`.
- Source context: main checkout `aca4c13774`; inspected freshly fetched `origin/main` at `c88a4e57865caa58e6c110e8b2c6a519b72e7fc1` using `git show`, without changing the checkout.
- Related sources: cresta-proto freshly fetched main `0ea419a6a2939a2b518715515ead8585f9117a57`; Director worktree `/Users/xuanyu.wang/repos/director-convi-7777-assistance-usage`, branch `xw/convi-7777-assistance-usage`, clean at `cc6fdfeb09febdf4bf66739d60376b79dfea88ed`.
- User request: verify whether the designed API feature explaining why each conversation qualifies was implemented in backend/frontend. Investigation only; no product edits or publication requested.

## Result

The protobuf contract is implemented; backend and frontend behavior are not. The counts implementation works independently of this optional diagnostic contract. This audit does not rerun counts tests or verify deployment.

1. Protobuf `analytics_service.proto:2173-2177` defines opt-in `include_conversation_usage_details`; lines 2201-2208 define complete, unsampled response-level `conversation_usage_details`, including no-use conversations/agents. `conversation_assistance_usage_stats.proto:62-110` defines conversation ID, eligible agents and deduplicated `used_features` using the ten-category `AssistanceUsageFeature` enum. No raw event payloads or timestamps are promised.
2. Backend `action_retrieve_conversation_assistance_usage_stats.go:163-165` explicitly rejects true with gRPC `Unimplemented`: `conversation usage details are not supported in this phase`. Handler validation at lines 36-38 runs before population/evidence reads. The existing validation test at lines 558-574 expects exactly this rejection.
3. Backend ClickHouse adapter projects only conversation/agent keys (`action_retrieve_conversation_assistance_usage_stats_clickhouse.go:75-111`). Hints combines behavioral/checklist/KB/workflow uses into one untagged query. `assistanceUsagePair` at lines 19-22 contains no feature; the union at lines 149-151 discards query provenance. Aggregation tests membership as a boolean at Go lines 254-269. Removing the rejection and serializing the pair set cannot recover the designed feature list.
4. Director `insightsApi.ts:321-340` constructs an explicit request without `includeConversationUsageDetails`. `types.ts:21-33` offers no opt-in property. `apiTypes.ts:88-97` exposes aggregate group counts/percentage only; `transformers.ts:214-230` preserves only those groups, discarding any future top-level details. No `conversationUsageDetails`, `includeConversationUsageDetails` or `AssistanceUsageFeature` consumer was found in the frontend implementation packages.
5. Agent and Team table hooks consume group percentages. Their heading tooltip describes qualifying categories globally; it is not evidence for a particular conversation. No per-conversation reason display is wired to this API.

## Current GitHub state

Fresh `gh pr view` confirms backend [#33993](https://github.com/cresta/go-servers/pull/33993) merged October 21 at 23:07 Toronto / October 22 03:07 UTC, merge `7fdfd2b4aed1114fbc7f166c064faf325c543281`. Director preparation [#24019](https://github.com/cresta/director/pull/24019) remains open at `2878945d6eb130c1c69afadf16bba0b53ff3130c`; column [#24020](https://github.com/cresta/director/pull/24020) remains open at the inspected worktree head. These are source and PR-state observations, not deployment proof. Earlier knowledge notes saying backend merge pending are superseded for this PR by this check.

## What remains to implement

- Preserve a deduplicated category set per eligible conversation/agent from the same evidence used for counts. Tag all ten categories, including the four currently merged hint categories. Keep no-use participants and conversations for denominator reconciliation.
- Build complete details only when requested; retain authorization/population matching, logging-disabled fields, and explicit failure instead of silent truncation. Choose a documented completeness/size policy when implementing.
- Replace the unsupported-options test with successful detail scenarios: multi-feature, duplicates, multi-agent attribution, zero-use, grouping independence, count reconstruction, and complete-response failure behavior.
- For use through Director, expose the opt-in and retain details in the adapter. The original contract was diagnostic; a customer-facing drawer or per-conversation view is a separate product choice, not implied by protobuf generation. Keep normal leaderboard requests aggregate-only.

## Verification and limits

Inspected current fetched source and live GitHub PR states, including the existing rejection regression. No Go or frontend product suite was run because no product behavior was changed and the rejection is explicit in the handler. No production API/data was queried. Authentication used normal git/gh program configuration; no AWS, Okta, Azure or SSH credential/profile was explicitly selected or read.

[Standalone report](../../deliverables/convi-7777-usage-reasons-audit.html).

Report verification: jsdom exercised all three layers, both code views, Previous/Next controls and pinned GitHub source links successfully. Scoped Markdown diff whitespace check passed. Visual rendering remains unverified because the installed Playwright package has no bundled Chromium executable; no browser was installed for this audit.
