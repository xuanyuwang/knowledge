# CONVI-7238: United manager scorecard undercount

**Status:** active
**Primary domain:** analytics
**Primary subdomain:** leaderboard
**Official ticket:** [CONVI-7238](https://linear.app/cresta/issue/CONVI-7238)
**Last updated:** 2026-07-23

## Objective and Impact

- **Objective:** Reproduce the Manager Leaderboard `Scorecards evaluated` count from ClickHouse using `RetrieveQAScoreStats` semantics and explain why an expected Chat Quality Form scorecard is excluded.
- **Customer/system impact:** United sees 16 scorecards in Manager Leaderboard versus 136 in Performance Insights; at least one manually evaluated scorecard appears absent from the manager count.
- **Role:** diagnosed

## Scope

**In scope**

- Trace the `RetrieveQAScoreStats` request and ClickHouse query semantics used by Manager Leaderboard.
- Validate scorecard rows for manager Emma Alatan (`1b3b751ed82f8272`) and template `0199e3b1-232c-76bb-9f71-f63d1da2527b`.
- Recompute the expected scorecard count with production ClickHouse data.

**Non-goals**

- Change product metric definitions before the exclusion mechanism is proven.
- Implement a fix during the initial investigation.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/director`, `/Users/xuanyu.wang/repos/clickhouse-schema`
- **Worktrees:** main checkouts
- **Branches:** main
- **PRs/commits:** Historical manager leaderboard migration context: CONVI-6968 / commit `6ec3409229`

## Current Understanding

The supplied template ID `0199e3b1-232c-76bb-9f71-f63d1da2527b` is authoritatively titled `1.2 - Engaging the Customer (Auto Scored)`, not Chat Quality Form. Under the exact request scope, Emma has 51 manually submitted/manually scored scorecards: 7 Engaging the Customer scorecards with `score >= 0`, and 44 `1 - Chat Quality Form` scorecards with aggregate `score < 0`. Both `RetrieveQAScoreStats` and `RetrieveQAConversations` receive `includeNaScored: false`, so they intentionally exclude all 44 Chat Quality Form rows and return only the 7 Engaging rows. The primary undercount mechanism is therefore the N/A-score filter, not incorrect template data returned by `RetrieveQAConversations`.

## Findings and Decisions

- Manager ID under investigation: `1b3b751ed82f8272`.
- Initially supplied scorecard template ID: `0199e3b1-232c-76bb-9f71-f63d1da2527b`; PostgreSQL identifies it as `1.2 - Engaging the Customer (Auto Scored)`.
- Current Chat Quality Form template ID with matching data: `019e6964-1a14-72ea-b11d-54c2b7951e10`.
- The scorecard is expected to satisfy the manual-submission requirement but is not reflected in the displayed count.
- The supplied FE request does not contain a template filter.
- `MANUALLY_SUBMITTED` maps to `scorecard_submit_time <> 0`.
- `includeNaScored: false` maps to `score >= 0` for scorecard-level stats.
- The requested time range filters `scorecard_time`, which is conversation-start time for conversation scorecards.
- The aggregate groups by `submitter_user_id` and daily-truncated `scorecard_time`, then counts distinct `scorecard_id`.
- Production breakdown before the N/A filter: Chat Quality Form = 44 and Engaging the Customer = 7, for 51 manually submitted/manually scored scorecards.
- `includeNaScored: false` excludes all 44 Chat Quality Form rows because their latest aggregate `score` is negative.
- The seven returned details are correctly labeled at the data layer with template ID `0199e3b1...`, which maps to Engaging the Customer revision `0d37d9b0`.
- A separate frontend concern remains: `ManagerLeaderboard.tsx` assigns rather than accumulates repeated daily rows. It is not the explanation for why Chat Quality Form rows are absent from both aggregate and detail responses.
- Case study `84608747-b451fdca-43d8-448f-b35b-9d5bf8f02ac0` has two Chat Quality Form scorecards. Postgres correctly stores both overall scores as NULL because every numeric/radio criterion in revision `b678cc4d` has weight 0; ClickHouse represents that NULL as `-1`.
- One case-study scorecard (`019f3e97-a65f-74a9-8b70-f7c93f377833`) also has a separate projection defect: current submitted metadata exists in Postgres and `score_d`, but `scorecard_d` remains on an older draft version across all replicas.

## Blockers and Dependencies

- No investigation blocker remains.

## Validation and Rollout

- Production ClickHouse simulation complete.
- Fix and frontend regression test are not yet implemented.

## Next Actions

1. Decide whether Manager Leaderboard `Scorecards evaluated` should count submitted scorecards even when their aggregate score is N/A.
2. If yes, force `includeNaScored: true` for both Manager aggregate and drawer requests, then recompute the expected count after accounting for stale/missing `scorecard_d` projections.
3. Measure and repair the `scorecard_d` projection gap, starting with scorecard `019f3e97-a65f-74a9-8b70-f7c93f377833`.
4. Separately verify/fix accumulation when the aggregate response contains multiple daily rows for one manager.

## Timeline

- 2026-07-23 — Began ClickHouse-level reproduction after learning the target template is manual-only and an expected manager scorecard remains excluded. Evidence: `sessions/2026-07-23/codex-convi-7238-clickhouse.md`.
