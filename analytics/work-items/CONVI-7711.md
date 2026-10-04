# CONVI-7711: Sunbit Performance Insights shows no data for Sean Hudson

**Technical status:** investigated; failing-session attribution pending  
**Official status:** Done, verified through Linear MCP on 2026-09-18; failing-session attribution remains unverified by this investigation  
**Primary domain:** `analytics`  
**Primary subdomain:** `performance-insights`  
**Official ticket:** [CONVI-7711](https://linear.app/cresta/issue/CONVI-7711/sunbit-performance-insights-returns-no-data-for-one-user-despite)  
**Last updated:** 2026-09-18

## Objective and scope

Diagnose Sean's empty Digital Support / Resolutions Performance Insights views. Verify PostgreSQL and ClickHouse before attributing the issue to the CONVI-7674 process-scorecard time-filter regression. Read-only database investigation; user subsequently authorized posting an evidence comment. No product changes, database repairs, or impersonation.

## Current understanding

**Strongly consistent with CONVI-7674; the same mechanism is reproduced on Sunbit data, but Sean's request payload is still missing.** Both templates are PROCESS and every relevant parent scorecard has an empty conversation ID. In the September 1–14 UTC control, PostgreSQL and ClickHouse have identical scorecard IDs, agents, interaction timestamps, and parent scores: 131 Digital Support, 99 Resolutions. A conversation-ended-time inner join changes those counts to zero/zero.

Actual criterion data is present: 124 Digital Support and 93 Resolutions latest parent versions have positive-weight, finite, non-N/A criteria in `score_d`. The 124 matches a Support-reported count, but exact screenshot audience, template IDs, timezone and capture time have not been verified.

A separate discrepancy exists: 7 Digital Support plus 6 Resolutions scorecards are submitted in PG but still have an older unsubmitted latest version in CH. This is not complete PG/CH parity; it also does not explain template-wide zero. Preserve this as a separate synchronization follow-up.

## Source context

- Primary repo `/Users/xuanyu.wang/repos/go-servers`, existing `main` at `b2bc3849854d124ed659058f8a8de6813262ddc6`; read-only.
- Related Director source and live GitHub release-tree inspection.
- PostgreSQL `sunbit-us-east-1`, CH `sunbit_us_east_1`, `us-east-1-prod`.
- Digital Support Template: `019cffb8-041b-7748-837b-ebe698d36c08@bccc9280`.
- Resolutions: `019a87fd-2f52-75ef-b4e5-e5d7f1e50111@a05d998e`.

## Validation and rollout

[Director PR #22772](https://github.com/cresta/director/pull/22772) merged September 15 as `f740a0fa253a6c304516502ac933387f09a1eb2c`. The live September 17 release branch tip `817023bda48a506d16e4929fceb886ee02cc12c2` lacks the fix by both ancestry and effective code inspection. Actual Sunbit deployed build remains unverified.

The dedicated Linear MCP was unavailable in this session. Ticket content came through Glean MCP; browser use was stopped on user instruction. Role/hierarchy claims in the ticket have not been independently checked as effective API permissions.

## Next actions

1. Obtain Sean's failing request or validate his same state with PR #22772 / a build containing it. Check `conversationTimeRangeField`, `timeRangeFilterTarget`, exact template, dates and audience.
2. If ENDED_AT is absent or the fixed build still fails, inspect effective ACL/user filtering.
3. Follow up separately on the 13 stale submission projections; exact IDs and timestamps are in the session note. No reindex performed.

## Timeline and evidence

- 2026-09-18 — Compared PG/CH, reproduced 131/99 → 0/0 with the ended-time join, verified criterion populations, identified 13 stale submission versions, and refreshed release exclusion. [Session evidence](../sessions/2026-09-18/codex-convi-7711-sunbit-no-data.md), [SQL](../sessions/2026-09-18/convi-7711-ended-time-reproduction.sql).

- 2026-09-18 — Posted and read-back verified the [SQL/result evidence comment](https://linear.app/cresta/issue/CONVI-7711/sunbit-performance-insights-returns-no-data-for-one-user-despite#comment-4203ea56) through authenticated Linear MCP. Existing Done status was preserved.
