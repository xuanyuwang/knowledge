# CONVI-7952: Marriott Insights 429 from oversized auth ACL response

**Status:** diagnosed; implementation and production validation pending
**Primary domain:** analytics
**Primary subdomain:** shared-analytics-platform
**Official ticket:** [CONVI-7952](https://linear.app/cresta/issue/CONVI-7952)
**Last updated:** 2026-11-06

## Objective and impact

Investigate Marriott QA Score Stats and Closed Conversations failures. Diagnosed a deterministic auth response-size failure during ACL expansion; existing ticket rate-limit hypothesis needs correction. Role: diagnosed.

## Current understanding

Read-only auth RPC reproduction against the active users' combined management roots returns ResourceExhausted because the response is 4,826,132 bytes and the default client receive limit is 4,194,304 bytes. Both users manage 2,426 users across 121 groups. The resource ACL helper asks public ListUsers for up to 5,000 full user resources per page and the injected Insights auth client retains the default service config. Root role override skips this expansion, explaining why support cannot reproduce it under a root session.

## Access and identities

Vault CLI installed; cresta-cli --auth=cresta read-only access verified to Marriott auth and app databases after the user obtained Marriott production access. No additional access currently needed. Active Mary is 90a3669c9bd81d5; inactive duplicate metub088 is 516d52dcacfe287f. Active Heather is 58bc185181ea031.

## Validation and recommendation

- Unchanged 4 MiB client / 5,000-user page: reproduces ResourceExhausted.
- Local 8 MiB receive cap / same request: all 2,426 users returned.
- Existing 4 MiB cap / 1,000-user pages: all 2,426 unique users returned over three pages.
- Recommend smaller ACL pages with complete-pagination regression coverage; an auth-client-specific receive-cap increase is another mitigation. A slim ID-only ACL endpoint avoids the unnecessary full resources longer term.
- No source/config edits, production writes or deployment. Operator-token direct RPC checks reproduce the management payload; real customer sessions and UI recovery remain unverified. Ticket was not edited or posted to Slack.

## Lightweight user API assessment

A live operator-token comparison returned the exact same 2,426 Marriott user IDs through ListUsersForAnalytics, in one page under an explicit 4 MiB receive cap. Use includeInactiveUsers=true, agentOnly=false, no profileIds, and the same transitive manager scope. This supports replacing the shared ACL user lookups to address the reproduced response-size failure. The current CONVI-7871 group-only plan would not fix it; the user agreed to add this user migration to that ticket.

Preserve direct peer versus transitive managed membership, optional selected users, all pages, caller access rules, and caller inclusion. Review internal server authentication, hardcoded user-type exclusions, and five-minute cache behavior. Current operator comparison proves Marriott scope ID equality, not customer-session or general authorization parity. LiteUser still carries memberships and has no universal size guarantee.

## Source context and next actions

Primary source /Users/xuanyu.wang/repos/go-servers main, 35c364ff698e86e4d8eef77f4504eb85dc76c5cc. No new worktree or branch. Supporting config and Flux checkouts read only.

1. Extend CONVI-7871 with the shared ACL managed/peer ListUsersForAnalytics migration; implement with caller-scope and inactive-user parity. Smaller public pages remain an immediate alternative.
2. Verify managed/peer ACL sets across all pages; preserve filtering and caller scope.
3. Deploy and verify Mary/Heather QA Score Stats and Closed Conversations under their real identities.
4. Correct the ticket's quota/size-limit diagnosis when authorized to update Linear.

## Timeline

- 2026-11-06 — Set up Vault-based customer access, verified active/duplicate account identity, traced the shared ACL dependency, reproduced oversized ListUsers and validated both local mitigations. [Detailed evidence](../sessions/2026-11-06/codex-convi-7952-marriott-429.md).

- 2026-11-06 — Verified identical Marriott user IDs with the lightweight API under 4 MiB; updated [CONVI-7871](https://linear.app/cresta/issue/CONVI-7871) to include the ACL managed/peer user migration and verified the saved description. Implementation and customer-session validation pending.
