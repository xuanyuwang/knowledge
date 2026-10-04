# CONVI-7784: Evaluate scorecard write race conditions

**Status:** active (Linear Backlog)
**Primary domain:** `scorecard-data-sync`
**Primary subdomain:** none
**Official ticket:** [CONVI-7784](https://linear.app/cresta/issue/CONVI-7784/systematically-evaluate-scorecard-write-race-conditions-and)
**Last updated:** 2026-10-01

## Objective and Impact

- **Objective:** Map scorecard write paths, assess concurrency and ordering risks, reproduce high-risk interleavings, and prioritize fixes.
- **Customer/system impact:** Two RCG scorecards had parent totals inconsistent with persisted criteria. One confirmed PostgreSQL mismatch propagated to ClickHouse, affecting the displayed score; exact writer interleavings remain unproven.
- **Role:** coordinated the backlog investigation scope.

## Scope

**In scope**

- Director and backend mutation paths, PostgreSQL parent/criterion consistency, and downstream projection ordering.
- Deterministic concurrency coverage and a prioritized mitigation plan.

**Non-goals**

- Attribute the September 11 CEL Retreat incident to a particular request order without traces.
- Treat the separately reported post-repair chart value as explained without its URL, filters, and serving path.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/director`; knowledge record in `/Users/xuanyu.wang/repos/knowledge`.
- **Worktrees/branches:** None created or inspected for this ticket; planning used existing incident records and Linear.
- **PRs/commits:** [go-servers PR #32447](https://github.com/cresta/go-servers/pull/32447), merged 2026-09-24; prior lost-update ticket [CONVI-6076](https://linear.app/cresta/issue/CONVI-6076).

## Current Understanding

RCG CEL Retreat scorecard `01a091f4-8df3-72f7-b976-d5b75a99abad` stored a 50% parent while four equally weighted persisted chapters calculate to 75%; ClickHouse mirrored the mismatch. A subsequent user edit made the PostgreSQL parent and latest ClickHouse rows 75%. The earlier submit and update timestamps were 752 microseconds apart, which supports but does not prove a race. PR #32447 addresses a likely submit/autosave mechanism but does not establish safety for update/update or other writers.

## Findings and Decisions

- Filed one broad evaluation ticket rather than prescribing an implementation before the writer-path and race matrix are verified.
- Ticket completion requires a source-linked writer/field map, risk matrix, deterministic reproduction for highest-risk paths, and ranked implementation plan with follow-up tickets.

## Blockers and Dependencies

- Request traces may be unavailable for incident-level attribution; the evaluation can proceed with current code and deterministic tests.
- The report/chart still showing 50% after repair remains a separate serving-path investigation in [the CEL Retreat work item](rcg-cel-retreat-score-mismatch.md).

## Validation and Rollout

- Linear issue was created with status Backlog and linked to CONVI-6076.
- No product code, tests, deployment, or scorecard data changed.

## Next Actions

1. Inventory current write paths and their field/transaction boundaries.
2. Build and exercise the concurrency matrix; classify reproduced failures separately from possible risks.
3. Recommend layered fixes and create focused implementation tickets.

## Timeline

- 2026-10-01 — Created [CONVI-7784](https://linear.app/cresta/issue/CONVI-7784/systematically-evaluate-scorecard-write-race-conditions-and) in Backlog, grounded in the [CEL Retreat investigation](../sessions/2026-09-28/codex-rcg-cel-retreat-score-mismatch.md). See [session](../sessions/2026-10-01/codex-scorecard-race-risk-ticket.md) and [daily log](../log/2026-10-01.md).
