# Group Calibration

## Purpose

Own answer-key and reviewer-response workflows, completion semantics, permissions, reporting, and session exports.

## Semantics and Invariants

- A calibration session has an answer key and participant response scorecards with distinct ownership and mutation rules.
- Updating or submitting a response currently requires its creator.
- Completion/consistency metrics must define which responses and criteria are eligible.
- Criterion comments are already returned by `ListScorecards`; the session CSV is assembled in Director.
- Exported numeric grades must map to option labels consistently with backend scorecard exports.

## Architecture and Source Map

- **Frontend:** Director Group Calibration pages and browser-built CSV
- **APIs:** `ListDirectorTasks`, `RetrieveDirectorTaskStats`, and `ListScorecards(FULL)`
- **Backend/storage:** coaching scorecard/task APIs and PostgreSQL scores/comments

## Operational Knowledge

- Diagnose export gaps at the frontend transformer/CSV layer before adding backend fields already present in the API.
- Compare Group Calibration CSV behavior with QM/Coaching Hub export intentionally; implementations are separate.

## Legacy Sources and Cases

- `group-calibration/`
- `scorecard-template/deliverables/scorecard-export-paths.md`
- `scorecard-permission-policy/`

## Open Questions

- Close numeric-label and other export parity gaps.
- Publish completion and consistency formulas.
