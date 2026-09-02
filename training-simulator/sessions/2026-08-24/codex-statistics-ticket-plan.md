# Training Simulator statistics implementation tickets

Date: 2026-08-24
Source repo: `/Users/xuanyu.wang/repos/knowledge`
Branch/worktree context: main checkout; no knowledge worktree

## Request

Break the existing lesson/module statistics engineering design into implementation steps/tickets, beginning with easier useful work while respecting backend, frontend, data-correctness, and rollout dependencies.

## Sources reviewed

- `deliverables/superhuman-api-design-update-draft.md`
- `deliverables/lesson-module-statistics-eng-design.md`
- `deliverables/lesson-module-statistics-fe-design.md`
- `work-items/lesson-module-statistics-reporting.md`
- `README.md`

## Result

Created `deliverables/lesson-module-statistics-ticket-plan.md` with:

- 18 tickets from contract closure through staged rollout;
- hard and integration dependencies per ticket;
- relative sizes, repo/owner areas, deliverables, and acceptance criteria;
- a dependency graph, five execution waves, parallel-work guidance, and a critical path;
- Option 2 as the working API assumption, without treating the open API review as decided.

The plan intentionally starts with small contract, fixture, and query-spike work. Persistence, pure aggregation, frontend mocks, and loader work can then overlap. UI work can progress against mocks, but live rollout remains blocked on explicit evaluation status/N/A persistence, authorization, bounded direct reads, backend hardening, and staging reconciliation.

## Durable updates

- Added the ticket plan to the project artifact registry and README.
- Updated the canonical work item with the execution sequence and new timeline evidence.
- Updated the daily log.

## Next action

The 18-ticket breakdown was subsequently superseded after review because it obscured the reporting mental model. The canonical draft now uses three independently reviewable milestones:

1. complete session reporting and prove the shared assignment/latest-settled-result model;
2. add lesson reporting;
3. add module and criterion reporting.

The revised plan suggests seven coarse result-persistence and BE/FE tickets. Cross-cutting correctness, authorization, telemetry, performance measurement, reconciliation, and rollout checks are acceptance criteria within each milestone rather than separate tickets.
