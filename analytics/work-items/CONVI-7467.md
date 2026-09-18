# CONVI-7467: Oportun supervisor missing Active Days

**Status:** active
**Primary domain:** `analytics`
**Primary subdomain:** `active-days`
**Official ticket:** [CONVI-7467](https://linear.app/cresta/issue/CONVI-7467/oportun-collections-supervisor-santiago-barreiro-missing-active-days)
**Last updated:** 2026-09-15

## Objective and Impact

- **Objective:** Explain and fix why Oportun supervisor Santiago Barreiro cannot see Active Days in the Agent Leaderboard even though his roles grant Manager Access and qualifying Active Days data exists within the reported scope.
- **Customer/system impact:** One confirmed Oportun Collections supervisor across the collections voice and collections SPL voice use cases; reporting visibility only.
- **Role:** diagnosing

## Scope

**In scope**

- Director metric visibility, selectable-metric construction, user/team filter state, `RetrieveAgentStats` and companion leaderboard requests, role/capability evaluation, and customer data needed to distinguish a UI suppression bug from a scope/data issue.

**Non-goals**

- Customer-data mutation, permission changes, or a reusable troubleshooting skill before the product root cause and fix are validated.

## Source Context

- **Repos:** `director`, `go-servers`, `config`, `coaching-qm-skills`, `knowledge`
- **Current investigation checkout:** `/Users/xuanyu.wang/repos/director` (local `main`; read-only investigation only)
- **Worktrees:** none yet
- **Branches:** none yet
- **PRs/commits:** none yet

## Current Understanding

A selected Conversation Duration range is the only condition found that exactly reproduces the reported symptom: Active Agents and Active Days summary cards disappear, the ordinary Active Days table column disappears, and Active Days is removed from the metric dropdown. Returning the filter to `All minutes` restores every surface. Director persists this filter per customer/profile/use case under the `leaderboard-page` local-storage state, so two users with the same role can see different metric availability.

This is the leading customer root cause, but it still needs Santiago-specific confirmation because the ticket screenshots were captured from a reporter/admin session and show `All minutes`. Ask Santiago to capture his duration label, set it to `All minutes`, and confirm Active Days returns. Do not simply unhide the metric: Agent Stats does not consistently apply the duration predicate to all rows used by Active Days, so showing it under a duration selection can present misleading semantics.

## Findings and Decisions

- Active Days is gated to `FA.CONVERSATIONS.LIVE.BASIC` in `useGetVisibleMetricsForLeaderboard`; this visibility check is not directly data-dependent.
- The live Oportun Role Permissions matrix shows Manager, Manager 2nd, and Insights Admin all have both `Insights / Leaderboard / Manager Access` and `Conversations / Live / Basic Access` in Collections SPL. Permission differences therefore do not explain Santiago versus Alondra.
- Santiago (`2118b19a05222659`) has 11 regular-source (`conversation_source = 0`) Collections SPL conversations on 8 distinct days from 2026-07-02 through 2026-08-10. Missing source data does not explain the symptom.
- Selecting `1 - 60+` in Conversation Duration reproduced the complete symptom in the authenticated Oportun tenant; resetting to `All minutes` restored it.
- `useVisibleColumnsForLeaderboards` hides the ordinary Active Days column when duration buckets are selected. `useFilterSelectableMetricsLogic` hides Active Days, Active Agents, and AHT from the selectable metrics under the same condition. Summary statistics use the same condition.
- `useLeaderboardsFilters` persists `conversationDurationBuckets` in the `leaderboard-page` filter state, explaining a per-user split without a role or data split.
- The duration constraint is intentional and dates to Director commit `7e13269db10` / PR #5663. The linked product discussion explicitly called out fewer or misleading Active Days under duration filtering; current code retains the constraint.
- Backend inspection supports keeping the constraint: duration is applied to `conversation_d`, but the Active Days path can retain active-assistance rows through a `FULL OUTER JOIN` where the corresponding source lacks `conversation_duration_secs`.
- Treat the `coaching-qm-skills` Active Days skill as a follow-on artifact after a proper fix and validation, so it documents a proven diagnostic decision tree rather than the current hypothesis.

## Blockers and Dependencies

- Need Santiago-specific before/after confirmation of the persisted Conversation Duration state. No supported customer-user impersonation path was found.
- A product UX change needs a clear placement/copy decision and focused regression coverage; the semantic fix is not to unhide Active Days under duration filtering.

## Validation and Rollout

- Exact UI reproduction: complete in the Oportun tenant.
- Customer-specific reset verification: pending Santiago/Support.
- Product UX regression coverage and live post-fix verification: pending implementation decision.

## Next Actions

1. Ask Santiago to capture the Conversation Duration label, change it to `All minutes`, and confirm Active Days returns in the card, column, and metric dropdown.
2. Treat that reset as the immediate customer remediation.
3. If a product change is desired, add explicit UX explaining that Active Days, Active Agents, and AHT are unavailable while duration filtering is active; do not expose semantically incorrect values.
4. Validate the customer path and any UX change, then create the reusable Active Days troubleshooting skill in `coaching-qm-skills`.

## Timeline

- 2026-09-15 — Started investigation from the Linear description/comments and current indexed/local Director code; identified a data-derived dropdown suppression path requiring live confirmation. Evidence: `sessions/2026-09-15/codex-convi-7467-active-days.md`, `log/2026-09-15.md`.
- 2026-09-15 — Reproduced the full symptom with a non-default Conversation Duration filter, proved permissions and regular-source activity are present, and identified persisted browser-local filter state as the leading root cause. Santiago-specific reset confirmation remains pending.
