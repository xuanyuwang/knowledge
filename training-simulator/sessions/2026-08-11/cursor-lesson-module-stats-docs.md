# Session Note - 2026-08-11 - Cursor - Lesson/module stats docs

**Started:** 2026-08-11 ~08:25 EDT
**Tool:** Cursor
**Project:** `training-simulator`
**Goal:** Collect docs on lesson/module statistics reporting and generate a project requirements brief

## Source Context

- **Primary repo:** `knowledge` (durable write) + `cresta-proto` (read)
- **Repo path:** `/Users/xuanyu.wang/repos/knowledge`
- **Worktree path:** main checkout (no worktree for knowledge)
- **Branch:** main (knowledge)
- **Ticket / PR:** none yet — work item `lesson-module-statistics-reporting`

## Inputs Reviewed

- Figma: `B5tJUlNnKbbjVfxH44nqNl` node `13108:21741` (Session / Module / Lesson reporting section + drawers + CSV annotation)
- Linear: Training Simulator project description/milestones; CONVI-7020, CONVI-7044, CONVI-7105, CONVI-7157; GONG-783, GONG-4179
- Slack: Woolworths reporting answer (Krystal, 2026-08-05); partnerships FAQ (2026-06-18); launch enablement thread (2026-06-23)
- Local: `stats.proto`, `RetrieveTrainingSimulatorTaskStats` request/response; domain `subdomains/reporting/README.md`; 2026-08-09 domain seed session
- Glean plugin enterprise searches: product/design docs, customer/GTM requirements, and implementation evidence; indexed results linked to PRD, engineering designs, design-review threads, customer asks, tickets, and PRs
- Access limitation: direct full-document reads were not exposed through the search path; Google Doc/Coda web fetch remained login-gated

## Actions Summary

- Authenticated Figma MCP; pulled design context + screenshots for section and drawers
- Searched Linear/Slack for reporting / lesson / module stats signals
- Ran three focused Glean plugin research passes and vetted results for relevance, freshness, and authority
- Inspected current stats proto surface
- Inspected `origin/main` (2026-08-10 refs) for task stats, task runs, DirectorTask content, revisioned lesson/module DB models, schema/indexes, and assignment snapshot ticket CONVI-7263
- Wrote and corrected the deliverable brief, work item, and daily log

## Findings

- Session-level stats shipped; lesson/module are designed and product-described as “coming soon,” but release phase and implementation tickets are unconfirmed
- Figma explores CSV columns across grains, but no authoritative CSV commitment/spec/date was found
- Module drawer centers on criterion pass fractions; lesson drawer centers on per-module pass rates
- Official Linear deferral text matches this work: aggregated agent- or lesson-level analytics
- Current official scoring uses latest module attempt; lesson score is a simple module average; all required modules must pass; N/A is excluded
- Repeated customer asks emphasize missed-behavior frequency, retries/improvement, longitudinal diagnostics, and content-quality identification
- Outcome facts are sufficient in `training_simulator_task_runs`, including revision IDs and criterion JSON; accurate never-started/adoption denominators require assignment snapshots.
- Current task stats loses zero-run assignments, ignores `direct_team_only`, uses task rather than run time filtering, caps task-run reads at 1,000, and reloads current content revisions.
- Director FE confirms missing relationship counts with `sessionsAssigned: 0` and `activeLessons: 0`; new stats messages include lesson session count and module active-lesson count.

## Decisions Made

- Keep durable artifacts under existing domain `training-simulator` (not a new top-level project)
- Publish requirements as `deliverables/lesson-module-statistics-reporting.md`
- Do not invent a CONVI id until product files tickets
- Treat CSV as a separate product decision, not committed v1 scope
- Propose two content-grain RPCs instead of extending session-grain `TaskStats`.
- Require CONVI-7263-compatible lesson/module/scenario snapshots in DirectorTask content before historical completion reporting.

## Follow-ups

- Verify full PRD/Design bodies through authenticated document access
- Product/design Q&A on release scope, metric semantics, and CSV commitment
- Review the backend contract; then file snapshot, proto, BE aggregation/index, FE, and QA tickets

## Links

- Deliverable: `deliverables/lesson-module-statistics-reporting.md`
- Figma: https://www.figma.com/design/B5tJUlNnKbbjVfxH44nqNl/Training-Simulator--Coaching-Simulator-?node-id=13108-21741
- Design doc (eng): https://docs.google.com/document/d/1GCeE9XCAVcgetOhYWPvqZJ3hp3qeVhK5YMk4rBkWmd4/edit
- PRD: https://coda.io/d/_ddKtmYQWQVC/Coaching-Training-Simulator-PRD_suAmSoxf
- P1 engineering design: https://docs.google.com/document/d/1luOZUCJ3FfzyrijTH1PY5heF2TTIKbCX5-bUNHx5hSw/edit
- Woolworths Slack: https://cresta.enterprise.slack.com/archives/C09H52T5FK7/p1785906804472019
- Reporting/scoring clarification: https://cresta.enterprise.slack.com/archives/C0AEE1T2U1X/p1781029644559089
- Assignment snapshots: https://linear.app/cresta/issue/CONVI-7263/update-training-simulator-session-to-take-snapshot-of-revisions-from
