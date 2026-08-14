# Session Note - 2026-08-13 - Codex - Lesson/module statistics design

**Started:** 2026-08-13
**Tool:** Codex
**Project:** `training-simulator`
**Goal:** Revalidate the reporting documents and produce a backend-first, frontend-second engineering plan

## Source Context

- **Primary durable repo:** `/Users/xuanyu.wang/repos/knowledge` (main checkout; no knowledge worktree)
- **Backend:** `/Users/xuanyu.wang/repos/go-servers`, branch `main`, commit `01a2c4ecbf` (2026-08-12)
- **Proto:** `/Users/xuanyu.wang/repos/cresta-proto`, branch `main`, commit `df2b436d03`
- **Frontend:** `/Users/xuanyu.wang/repos/director`, branch `main`, commit `4a0962688d`
- **Product code changes:** none
- **Credentials used:** none

## Inputs Reviewed

- knowledge operating model, domain model, project YAML/README, reporting subdomain, work item, requirements brief, prior backend draft, and prior discovery/design sessions
- current Training Simulator stats/service/task protos and public/persisted DirectorTask content config
- current task-stats handler/tests, task-run CRUD/list path, converters, GORM models, and Director SQL schema/indexes
- current Director API wrapper, query hook, filters, routing/access gates, Training Sessions reporting patterns, Lesson/Module tables, placeholder count columns, and feature flags

## Findings

1. The 2026-08-11 backend draft was stale against current `go-servers/main`. Conversation score/pass/criterion/agent data moved to `director.training_simulator_conversation_scores`; quiz outcomes live in `director.quiz_scores`; task-run columns are now migration fallback.
2. `ListTrainingSimulatorTaskRuns` still caps results at 1,000 and cannot be the aggregate implementation substrate.
3. The task-runs composite index already covers `(customer, profile, task, lesson, module)`, so the reporting path should query by scoped task IDs and measure before adding lesson/module indexes.
4. Quiz runs expose `quiz_score` but no persisted pass boolean. A unified reporting outcome needs an explicit quiz pass rule using the historical module threshold, and session stats should eventually share it.
5. Task-run creation currently looks up the latest lesson/module revisions. Assignment snapshots must change both DirectorTask content and task-run revision selection.
6. The existing aggregate RPC includes `AGENT`, but content-level cohort APIs are not an agent surface. Request filters alone do not authorize manager access to arbitrary users.
7. Director already has global assignee/date filters, two hard-coded reporting counts, batch content loads of 200, `FullDrawer`, and session reporting adapters. The main missing integration is threading filter state into Lesson Configuration and adding batch stats hooks/view models/drawers.
8. Managers can view Training Simulator but cannot reach Lesson Configuration. Manager reporting requires a new Insights route or a permission split; broadening authoring access is not acceptable.

## Decisions and Design Direction

- Keep separate lesson- and module-stats RPCs and leave session stats unchanged for the first release.
- Start from assignment facts to retain never-started users, then join normalized subtype outcomes and aggregate latest attempts.
- Use 0.0–1.0 scores/rates in the new API and convert scores once to Director's 0–100 view-model scale.
- Match the current Director batch size with a 1–200 name request contract; do not issue per-row calls.
- Start with QA/global-admin roles if the UI ships in Lesson Configuration. Add manager roles only with a manager-visible route and explicit manageable-user scope.
- Treat quiz pass semantics, included task statuses, date semantics, and reporting placement as review gates rather than silently guessing.
- Integrate counts/drawers behind the existing V2 flag unless rollout needs an independent reporting flag.

## Artifacts Updated

- `deliverables/lesson-module-statistics-eng-design.md` — rewritten around the current normalized storage model, assignment snapshots, authorization, query plan, tests, and rollout
- `deliverables/lesson-module-statistics-fe-design.md` — added concrete Director data flow, hooks, view models, tables, drawers, access, tests, and delivery sequence
- `deliverables/lesson-module-statistics-reporting.md` — marked implementation sections as superseded by the new designs
- `work-items/lesson-module-statistics-reporting.md` — current findings, decisions, and next actions
- `subdomains/reporting/README.md`, project `README.md`, and `project.yaml` — current domain state and artifact links

## Next Steps

1. Product/security review: reporting route/personas, task status/date semantics, quiz pass rule, all-N/A display, and manager scope.
2. Complete/schedule CONVI-7263 snapshots and task-run revision pinning.
3. Run a task-scoped PostgreSQL query-plan spike over normalized conversation/quiz outcomes above 1,000 runs.
4. File proto/snapshot, BE, FE, and QA implementation tickets.
5. Implement BE first; integrate Director only after contract and score/empty semantics are approved.
