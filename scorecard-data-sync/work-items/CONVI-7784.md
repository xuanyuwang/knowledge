# CONVI-7784: Evaluate scorecard write race conditions

**Status:** investigation accepted; corrected recommendations and preferred redesign recorded (official Linear status not refreshed; last recorded as Backlog)
**Primary domain:** `scorecard-data-sync`
**Primary subdomain:** none
**Official ticket:** [CONVI-7784](https://linear.app/cresta/issue/CONVI-7784/systematically-evaluate-scorecard-write-race-conditions-and)
**Last updated:** 2026-11-04

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

As of November 4, Xuanyu considers the linked Google Doc investigation sufficient. Its seven issues and eight review comments are incorporated in the [corrected recommendations](../deliverables/scorecard-races-corrected-recommendations.md). Field masks are a useful first improvement; aggregate recomputation requires actual concurrency protection; primary reads depend on capacity; CH projection time must not become a source version. The preferred redesign uses a shared mutation function, explicit PG/CH versions, optimistic conflict detection and caller-specific recovery. This is a recorded design direction, not implementation completion. The sync-delivery redesign is still unspecified in that document, and audit history remains optional.

RCG CEL Retreat scorecard `01a091f4-8df3-72f7-b976-d5b75a99abad` stored a 50% parent while four equally weighted persisted chapters calculate to 75%; ClickHouse mirrored the mismatch. A subsequent user edit made the PostgreSQL parent and latest ClickHouse rows 75%. The earlier submit and update timestamps were 752 microseconds apart, which supports but does not prove a race. PR #32447 addresses a likely submit/autosave mechanism but does not establish safety for update/update or other writers.

## Findings and Decisions

- Filed one broad evaluation ticket rather than prescribing an implementation before the writer-path and race matrix are verified.
- The original ticket requested a writer/field map, risk matrix and deterministic reproduction. The November 4 instruction accepts the investigation as sufficient for moving to corrected implementation planning; this documentation task does not claim those tests were executed or update the official ticket status.

## Blockers and Dependencies

- Request traces may be unavailable for incident-level attribution; the evaluation can proceed with current code and deterministic tests.
- The report/chart still showing 50% after repair remains a separate serving-path investigation in [the CEL Retreat work item](rcg-cel-retreat-score-mismatch.md).

## Validation and Rollout

- Linear issue was created with status Backlog and linked to CONVI-6076.
- No product code, tests, deployment, or scorecard data changed.

## Next Actions

1. Plan concrete changes from the corrected recommendations rather than repeat the broad investigation.
2. Define expected-version/conflict behavior, aggregate transaction rules, all-writer coverage and CH table/cleanup migration. Keep durable delivery and full-conversation ES versioning as separate open designs.
3. Validate process-queue/submit-flush deployment and primary capacity when planning rollout; add deterministic regression cases for the chosen implementations. No such runtime validation is claimed here.

## Timeline

- 2026-10-01 — Created [CONVI-7784](https://linear.app/cresta/issue/CONVI-7784/systematically-evaluate-scorecard-write-race-conditions-and) in Backlog, grounded in the [CEL Retreat investigation](../sessions/2026-09-28/codex-rcg-cel-retreat-score-mismatch.md). See [session](../sessions/2026-10-01/codex-scorecard-race-risk-ticket.md) and [daily log](../log/2026-10-01.md).

- 2026-11-04 — Incorporated Bo Hua Tang’s race/sync investigation with all eight indexed Xuanyu comments; recorded the explicit-version redesign and corrected unsafe transaction/timestamp recommendations. See [reference](../deliverables/scorecard-races-corrected-recommendations.md) and [session](../sessions/2026-11-04/codex-scorecard-doc-corrections.md).
