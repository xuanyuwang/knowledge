# lesson-module-statistics-reporting: Lesson & module level stats

**Status:** active
**Primary domain:** `training-simulator`
**Primary subdomain:** `reporting`
**Official ticket:** none yet (file under Training Simulator → Post-Launch Enhancements)
**Last updated:** 2026-08-13

## Objective and Impact

- **Objective:** Define and prepare implementation of lesson- and module-level statistics reporting (UI drawers and aggregates; CSV remains an uncommitted design exploration) on top of existing session-level `RetrieveTrainingSimulatorTaskStats`.
- **Customer/system impact:** Lets coaches diagnose content quality (weak modules/criteria) across assignments; fulfills “module/lesson level coming soon” GTM messaging.
- **Role:** investigated requirements and authored the backend-first/frontend-second engineering plan

## Scope

**In scope**

- Requirements synthesis from Figma, Linear project deferrals, Slack product answers, and current proto/impl
- Project brief in `deliverables/lesson-module-statistics-reporting.md`
- Open questions for product/design before eng spike

**Non-goals**

- Implementation in this work item
- Production KPI uplift linking (GONG-4179)
- CSV implementation until product confirms commitment, row grain, and semantics

## Source Context

- **Repos:** `cresta-proto` (read), `go-servers` / `director` (referenced via domain map)
- **Worktrees:** none for this discovery
- **Branches:** n/a
- **PRs/commits:** n/a

## Current Understanding

Session-level reporting is shipped. August Figma defines Session / Module / Lesson reporting sidecards; product has publicly said module/lesson reporting is coming soon. No dedicated Linear implementation ticket was found. Figma explores CSV exports, but no authoritative commitment, spec, or release date was found.

The backend and frontend plans are now defined in separate deliverables. The backend reads DirectorTask assignments plus revision-pinned task runs, joins the normalized conversation/quiz outcome tables, aggregates latest attempts by content, and exposes separate lesson/module stats RPCs. Accurate historical adoption/completion still requires CONVI-7263 assignment snapshots first; current DirectorTask content stores only stable lesson names.

## Findings and Decisions

- Treat merged `RetrieveTrainingSimulatorTaskStats` as the session-grain baseline; design-doc `GetTrainingSimulatorStats` is historical.
- Prefer new lesson/module grains rooted in `TrainingSimulatorTaskRun` rather than ClickHouse for v1 unless scale requires it.
- Preserve current semantics: latest module attempt is official, lesson score is the simple average of required module scores, all modules must pass, and N/A is excluded.
- Track CSV as an open product decision rather than an implementation requirement.
- Do not extend `TaskStats`; add `RetrieveTrainingSimulatorLessonStats` and `RetrieveTrainingSimulatorModuleStats`.
- Add assignment snapshots to public and persisted DirectorTask content protos; no relational migration is needed for the snapshot itself.
- `go-servers/main` moved conversation evaluation outcomes to `training_simulator_conversation_scores` and quiz outcomes to `quiz_scores`; the reporting reader must normalize both plus the pre-migration fallback columns.
- The existing task-run composite index is likely sufficient for a task-scoped query; do not add lesson/module indexes until an `EXPLAIN ANALYZE` spike proves the need.
- Content-level aggregate RPCs should not copy the existing `AGENT` role. Start with the admin/QA-admin Lesson Configuration surface; manager access requires a separate reporting route and explicit manageable-user authorization.
- Director can reuse the current filter state, table placeholders, `FullDrawer`, and reporting patterns, but must batch by content name rather than issue per-row requests.

## Blockers and Dependencies

- Product answers needed for Insights vs Lesson Configuration placement, release scope, date-range semantics, and the “Active lessons” metric on the lesson drawer.
- Full document bodies were not exposed through the plugin search path; indexed Glean results were vetted and linked, with reduced confidence where metadata/content was partial.
- Figma MCP hit View-seat rate limit mid-session after primary screenshots were captured.
- CONVI-7263 is Todo; without it, never-started agents cannot be tied to historical lesson/module revisions after content edits.

## Validation and Rollout

- n/a until tickets exist

## Next Actions

1. Review the backend product/security gates: task statuses, date semantics, quiz pass rule, roles, and manager scope.
2. Implement or schedule CONVI-7263 assignment snapshots and task-run revision pinning.
3. Spike the normalized task-run/conversation-score/quiz-score query above 1,000 runs and capture `EXPLAIN ANALYZE`.
4. Create Linear epic + proto/snapshot, BE aggregation, FE integration, and QA tickets.
5. After the BE contract is approved, implement the Director hooks/count columns and lesson/module drawers described in the FE design.

## Timeline

- 2026-08-11 — Collected Figma/Linear/Slack/proto context; wrote project brief. Evidence: `deliverables/lesson-module-statistics-reporting.md`, `sessions/2026-08-11/cursor-lesson-module-stats-docs.md`, `log/2026-08-11.md`.
- 2026-08-11 — Corrected the brief after three Glean plugin enterprise-search passes: added vetted PRD/design/customer/implementation sources, clarified current metric semantics, and downgraded CSV from requirement to uncommitted exploration.
- 2026-08-11 — Defined the backend read model, aggregation grains, assignment-snapshot prerequisite, query/index strategy, and additive proto/RPC proposal after inspecting authoritative August 10 `origin/main` refs.
- 2026-08-13 — Revalidated the plan against current `go-servers`, `cresta-proto`, and `director` main checkouts; corrected the backend for normalized conversation/quiz score storage and produced a concrete FE integration design. Evidence: `deliverables/lesson-module-statistics-eng-design.md`, `deliverables/lesson-module-statistics-fe-design.md`, `sessions/2026-08-13/codex-lesson-module-stats-design.md`.
