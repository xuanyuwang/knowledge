# Scorecard Sync Phone Story Session - 2026-08-31

## Context

- **Objective:** Turn the CONVI-5565 PostgreSQL-to-ClickHouse scorecard consistency project into a precise, value-oriented phone interview narrative.
- **Primary domain:** `train-for-staff` (system/career synthesis), grounded in the canonical `scorecard-data-sync` domain.
- **Primary source repo:** `/Users/xuanyu.wang/repos/knowledge`
- **Branch/worktree:** `main` in `/Users/xuanyu.wang/repos/knowledge`
- **Timezone:** America/Toronto

## Inputs Reviewed

- `train-for-staff/staff-project.md`, `resume-snippets.md`, `resume.typ`, and `senior-to-staff.md`.
- `convi-5565-scorecard-ch-pg-sync/README.md`, `investigation.md`, and the March 11–12 production-verification logs.
- `scorecard-data-sync/README.md`, `deliverables/architecture-and-invariants.md`, and `deliverables/failure-modes-and-case-index.md`.
- `blog/2026-03-13-debugging-dual-database-sync.md`.

## Synthesis

- Reframed the project from “fixed a PG/CH race” to end-to-end ownership of an ambiguous customer-facing analytics correctness problem.
- Made the two independent failure mechanisms explicit: stale async projection ordering and PostgreSQL lost updates from full-struct ORM saves.
- Separated business value, technical challenge, personal contribution, operational discipline, and reusable leverage.
- Resolved an evidence ambiguity: 9,155 PG submitted scorecards were inspected, but 2,996 present in both stores were the comparable correctness population.
- Preserved the 0.87% stale-submission residual as a bounded architectural limitation instead of claiming perfect consistency.
- Expressed senior/staff evidence through demonstrated behaviors rather than a self-awarded level claim.

## Artifacts Updated

- `train-for-staff/deliverables/phone-screen-scorecard-pg-clickhouse.md`
- `train-for-staff/resume-snippets.md`
- `train-for-staff/resume.typ`
- `train-for-staff/staff-project.md`
- `train-for-staff/project.yaml`
- `train-for-staff/log/2026-08-31.md`

## Credentials Used

- None. Only local knowledge artifacts were read.

## Follow-up

- Rehearse the 30-second and two-minute versions aloud and edit phrases that do not sound natural in spoken English.
- Decide whether the next artifact should cover Group Calibration, user-filter consolidation, or ClickHouse external tables to create a balanced three-project interview portfolio.
