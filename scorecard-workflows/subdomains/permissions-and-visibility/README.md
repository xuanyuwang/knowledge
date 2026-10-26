# Permissions and Visibility

## Purpose

Own the policy that determines scorecard/template capabilities and which records, details, comments, and related signals a requester may see.

## Semantics and Invariants

- Capability questions (grade, submit, appeal, publish, edit submitted) and visibility questions are distinct.
- Evaluation depends on requester identity and memberships, template permissions, scorecard type/state, creator/agent relationships, and workflow membership.
- Template editors, viewers, graders, appealers, publishers, and submitted-scorecard editors are separate audiences.
- Group-calibration response update/submit requires the response creator under current behavior.
- Appeal request and appeal resolution are distinct scorecard types/workflows.
- Notification eligibility and read visibility are related but not identical.

## Architecture and Source Map

- **Frontend:** template Access controls and runtime permission evaluation/locking
- **APIs/backend:** coaching permission checks and scorecard permission evaluation
- **Policy data:** `ScorecardTemplate.permissions` plus runtime scorecard/task context

## Operational Knowledge

- Evaluate permissions with the historical template revision and current runtime state.
- Retain reactive backend denial even when the frontend proactively disables an action.

## Legacy Sources and Cases

- [scorecard-permission-evaluator-design-history](../../work-items/scorecard-permission-evaluator-design-history.md)
- [submitted-scorecard-permission-history](../../work-items/submitted-scorecard-permission-history.md)
- [scorecard-access-advanced-layout](../../work-items/scorecard-access-advanced-layout.md)

## Open Questions

- Complete the centralized capability/visibility evaluator beyond submitted editing.
- Define consistent enforcement for comments, exports, and notifications.

See the [October 25 historical synthesis](../../deliverables/inactive-legacy-project-consolidation-2026-10-25.md) for consolidated cases, later superseding decisions, and their evidence limits.
