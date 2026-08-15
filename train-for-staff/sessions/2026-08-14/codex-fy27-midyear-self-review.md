# FY27 Mid-Year Self-Review Drafting Session

**Date:** 2026-08-14

**Source repo:** `/Users/xuanyu.wang/repos/knowledge`

**Branch/worktree:** `main` at `/Users/xuanyu.wang/repos/knowledge`

## Objective

Draft Xuanyu Wang's FY27 H1 self-reflection in a senior/Staff engineer voice, answering the four prompts in the supplied HiBob PDF and grounding Operating Principle language in Cresta's canonical internal definition.

## Inputs Reviewed

- `/Users/xuanyu.wang/Downloads/Bob.pdf` (three pages, prompts visually and textually verified)
- Glean document: `Operating Principles` (`https://docs.superhuman.com/d/_dOdd8RvXUvm/_supg5MWW`)
- `train-for-staff/staff-project.md`, `senior-to-staff.md`, `resume-snippets.md`
- H1 weekly summaries and canonical project evidence for external tables, user-filter semantics, scorecard consistency, backfills/reindexing, scorecard-template stewardship, and July analytics investigations

## Decisions

- Selected three contributions that show Staff-level breadth without relying on uncompleted or overstated outcomes: scalable analytics filtering, scorecard reliability/repair, and domain stewardship.
- Final human-selected principles are `Jump In`, `Help our Customers Win`, and `Patient in Strategy, Impatient in Execution`.
- Grounded `Jump In` in moving beyond familiar scorecard API work into user-filter semantics, scored-N/A template and formula lifecycles, and ClickHouse query/storage behavior, with concrete designs, fixes, tests, and rollout outcomes in each area.
- Kept claims bounded where repository artifacts conflict or include partial populations: 2,996 PG/CH matched scorecards for the zero-mismatch result; user-filter migration not claimed as complete; 0.87% rapid Save-to-Submit residual retained.
- Framed H2 growth around converting accumulated investigation judgment into reusable team capability and linking technical strategy to measurable customer/business outcomes, with concrete manager asks.
- Defined reuse as adoption-oriented playbooks, decision trees, verification tools, worked examples, and AI-assisted skills—not documentation produced without evidence that others can apply it.
- Revised the second contribution to make `scorecard-sync-monitor` and auto-heal the central Staff-level reliability story: fleet-wide measurement, missing/stale classification, targeted repair, rollout guardrails, and convergence-oriented operations.
- Kept the auto-heal claim within the H1 boundary: implementation and staging validation are included; unrestricted production rollout and August recovery outcomes are not claimed.

## Output

- `train-for-staff/deliverables/fy27-mid-year-self-review-draft.md`

## Additional Evidence for Reliability Framing

- `weekly-summary/weekly-summary-2026-04-06-to-12.md`
- `auto-backfill-missing-scorecards/auto-heal-design.md`
- `auto-backfill-missing-scorecards/implementation-and-validation-summary.md`
- `auto-backfill-missing-scorecards/rollout-plan.md`
- `scorecard-data-sync/deliverables/monitoring-and-diagnosis.md`
- `scorecard-data-sync/deliverables/repair-and-backfill-playbook.md`
- `scorecard-data-sync/log/2026-07-22.md`

## Follow-up

- Human should adjust tone, collaborator attribution, and any confidential incident language before pasting into HiBob.
- Optionally produce a shorter version if HiBob field limits or manager preference require it.
