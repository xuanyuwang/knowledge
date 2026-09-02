# Superhuman engineering-design comment review

Date: 2026-08-23
Source repo: `/Users/xuanyu.wang/repos/knowledge`
Branch/worktree context: main checkout; no knowledge worktree

## Inputs

- Downloaded PDF: `/Users/xuanyu.wang/Downloads/Training Simulator Lesson and Module Statistics — Engineering Design.pdf`
- Six review threads exported on pages 12-13.
- Current list request protos in `cresta-proto`.
- Current Training Simulator filter and Lesson Configuration code in `director`.

## Findings and proposed resolutions

1. Reuse/extend list filters: option 2 already extends `ListTrainingLessons` and `ListTrainingModules` with an optional statistics filter. Keep catalog filters and reporting filters semantically separate.
2. Product filter scope: the page already exposes agents, teams, virtual groups, and date range. This project wires that state into Lesson Configuration; it adds no filter UI. Omit `direct_team_only` unless Product expands the page scope.
3. Missing message details: add complete definitions for `TrainingSimulatorResultSummary`, `TrainingSimulatorModuleResultSummary`, and `TrainingSimulatorCriterionStats`.
4. `result_status_missing`: rename to `has_missing_result_status` and document that it warns about legacy rows lacking persisted evaluation status; assignment/attempt counts remain valid, while score/pass/N-A results may be incomplete.
5. `active_lesson_count`: define it as the number of current `STATE_ACTIVE` lessons containing the module, independent of user/date filters.
6. `current_passing_score`: remove it from statistics. The listed `TrainingModule` already carries the current 0-100 threshold in its evaluation config, so duplicating it creates two sources of truth.

## Superhuman Docs status

The connector later became available through its Superhuman Docs URI support. Read back the full live page and all 12 active comment threads, updated the remote design, and replied to every active thread with its resolution. Threads remain active for reviewer confirmation; none was resolved unilaterally.

Remote updates include the open API decision, option-dependent frontend loading, complete shared protobuf messages, existing-filter reuse, removal of `direct_team_only`, `has_missing_result_status` semantics, active-lesson scope, and removal of the duplicated passing-score field. A final live read found no stale text claiming that option 3 was selected.

## Statistics-definition follow-up

Consolidated all calculations in the local Superhuman draft under one authoritative `Statistics definitions` section. It defines common terms; lesson/module counts, rates, scores, attempts, and retries; content counts; criterion rollups; empty-denominator behavior; legacy warnings; and invariants. It also clarifies that `agent_user_id` is derived by expanding the DirectorTask audience rather than read from a task-table column. The live page was intentionally left unchanged for this follow-up pending local review.

Expanded that section to cover all three Figma reporting levels explicitly: Session, Lesson, and Module. Session reporting now defines the DirectorTask row and per-assigned-agent calculation unit, zero-run behavior, counts, scores, pass rates, attempts, retries, overall cards, score distribution, and assigned-agent completed/overdue values.

Documented an additive migration for `RetrieveTrainingSimulatorTaskStats`: root loading in filtered DirectorTasks and expanded audiences; direct uncapped database reads; current lesson/module resolution without the first-run assumption; complete/N-A classification from persisted evaluation state; one agent entry for every assignee; shared result-summary and missing-status fields; and an explicit decision or enforcement for tasks containing multiple lessons. The legacy `score = -1` plus `not_applicable = true` convention remains only as a compatibility representation of an absent meaningful task score; genuine all-N/A counts live in the new result summary.

The per-agent compatibility contract now explicitly distinguishes incomplete, completed all-N/A, passed, and failed using existing `status`, `not_applicable`, and `passed` fields. Director currently treats every `COMPLETE && !passed` entry as failed; it must check `not_applicable` first so completed all-N/A sessions remain completed but do not enter score or pass-rate denominators.

Refined the Session section after product feedback: do not repeat session statistics already present in the design. Retain only the missing incomplete-assignee count and per-agent drawer values (status, all-attempt count, score percentage, agent identity, latest time, and View link). Most values already exist or can be derived from `AgentPerformanceEntry`; add only `attempt_count` for the drawer, while keeping `has_missing_result_status` as a separate data-correctness warning. The incomplete-agent count is derived from the complete agent list rather than duplicated on `TaskStats`.

Refined lesson/module scope to follow the Figma UI strictly. Lesson statistics now expose only session count, average score, pass rate, and per-module pass-rate chips. Module statistics expose only active-lesson count, average score, passed/failed fraction, pass rate, and criterion pass fractions. Removed assignment, started, completion, incomplete, N/A, attempt, and retry fields from these reporting definitions and proto examples. Content metadata and current thresholds continue to come from the existing lesson/module resources.

Attempted to sync the full local draft back to the existing Superhuman Docs page while preserving all comment threads. No remote mutation occurred: the Coda/Superhuman connector returned `Unauthorized`, the local Superhuman Docs app-control bridge could not start, and browser discovery found no connected browser. Resume by reconnecting the Coda/Superhuman integration or enabling a browser/app-control connection; update the existing page body rather than recreating the page so comments remain attached.

## Local updates

- `deliverables/lesson-module-statistics-eng-design.md`
- `deliverables/superhuman-api-design-update-draft.md`
- `deliverables/lesson-module-statistics-fe-design.md`
- `work-items/lesson-module-statistics-reporting.md`
- [Superhuman engineering design](https://docs.superhuman.com/d/_dE0dCcz8Bub/_subF07cy)
