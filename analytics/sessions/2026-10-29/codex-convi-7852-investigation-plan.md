# CONVI-7852 ticket review and investigation plan

Date: 2026-10-29 (America/Toronto)
Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
Branch/worktree context: `main`, main checkout; no product edits or source investigation.

## Request and evidence

User requested reading CONVI-7852 and summarizing its problem and proposed investigation. Read the current issue with relations and its one comment through Linear MCP. Read workspace knowledge workflow, analytics project metadata, README, and Leaderboard reference to choose the durable home. A lightweight memory search yielded no ticket-specific evidence used in the answer.

## Outcome

Recorded reported symptoms, Sarah Young's consolidated-membership reproduction, customer expectations, five-region impact, historical hypothesis, and explicit evidence limits in [the work item](../../work-items/CONVI-7852.md). The planned investigation traces customer memberships and hierarchy through API responses to the Team cell, checks current selection rules and history, separates labels from analytics/access effects, and resolves ambiguous role combinations before recommending a fix.

No claim of a confirmed root cause. Historical Slack, Zendesk screenshots, customer data, and product source remain unverified. No external mutations. No AWS, Okta, Azure, or SSH credential selected or used. Existing unrelated knowledge changes preserved.

## Follow-up

The user subsequently authorized starting the investigation. It was executed read-only as recorded below.

## Source and production investigation

- Traced AgentLeaderboardPage's Agent+Team request through RetrieveConversationStats, ParseUserFilterForAnalytics, buildUserGroupMappings, the ListUsersForAnalytics projection/proto, Director transformAttributes, row.team, and CSV export. Current source returns sorted direct-team IDs; Director picks the first; membership role information is omitted upstream. Fetched origin/main and checked relevant diffs to rule out a newer role-aware implementation.
- Read the December 2025 org_management_eng thread through Glean. It confirms multiple teams are allowed; May 27, 2026 commit a2f2905b5c4 / #28217 added stable sorting, not role priority.
- Used the customer-auth-DB skill, resolving latest config to AWS account alias us-east-1-prod, auth cluster auth-us-east-1-prod, database advent-health, profile us-east-1. Initial slug adventhealth did not resolve; config verified the correct slug advent-health. Used cresta-cli's read-only connection and explicitly enabled session default_transaction_read_only before membership/count checks; statement timeout 15 seconds. No credential output or persistence. Session closed.
- Read all six users' direct memberships and relevant hierarchy. Sarah/Allison's sorted IDs select Manager 2nd teams; Ana's current order selects her Agent team; Gary/Stephanie have three Agent teams; Anita's expected team is the parent of her managed team. Legacy team_id/manager_id are empty in all six.
- Corrected population query (UserType DEV=3, EXTERNAL=6) returned 960 active-profile users with Agent memberships, 43 with one Agent team plus other direct teams, 33 whose first sorted team is different, and 25 with multiple Agent teams. An initial exploratory query used incorrect type exclusions; only corrected values are promoted. These are membership counts, not live affected-row counts.
- [Replay](convi-7852-team-label-replay.cjs) extracted actual Director attribute/conversation-stat transformer functions and modeled backend ID sorting. Sarah selected Christina Gusby; Allison selected Silvia Martin. Synthetic count 17 and one row were preserved; reversing groups changes the label. Passed with Node. User/group conversion identity stubs isolate selection; no product source/test edits.
- Attempted a background IAB visit to the production Agent Leaderboard. It redirected to login.cresta.com for advent-health. No authenticated session available; no credentials entered, and temporary tab closed. Live UI/API and deployed-image verification remain open.

## Deliverable and next work

[Investigation report](../../deliverables/convi-7852-investigation.md) contains exact source references, predictions, query population, effects on Agent metrics/CSV/Team rollups, replay limits, and recommended decisions. [SQL](convi-7852-membership-checks.sql) preserves reproducible read-only queries. Need authenticated live response/version confirmation and agreed multi-Agent/manager-only reporting rules before choosing an implementation. No external communication, ticket update, org-config mutation, or product changes.
