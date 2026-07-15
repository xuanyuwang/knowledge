# Appeals

## Purpose

Own appeal request and resolution workflows, their relationship to the original scorecard, and how final values and comments appear downstream.

## Semantics and Invariants

- Appeal request and appeal resolution are distinct scorecard types and permission contexts.
- The original evaluation, requested change, resolution, and final effective result must remain distinguishable.
- An approved appeal reason may be the correct exported criterion comment when it explains the final effective value.
- Appeal visibility and action permission must be evaluated separately.

## Architecture and Source Map

- **Frontend:** appeal request/resolve scorecard experiences
- **Backend:** coaching appeal actions, related scorecards, permissions, and export assembly
- **Storage:** original and appeal scorecards/scores plus comments and relationships

## Operational Knowledge

- Trace both the original and appeal scorecards when the UI/export disagrees with the final decision.
- Confirm whether a consumer wants historical original data or the effective appealed result.

## Legacy Sources and Cases

- `export-appeal-comments/`
- `scorecard-template/deliverables/workflow-map.md`
- `scorecard-permission-policy/`

## Open Questions

- Define the canonical effective-value resolver for UI, exports, analytics, and notifications.
- Document partial approval and repeated-appeal behavior.
