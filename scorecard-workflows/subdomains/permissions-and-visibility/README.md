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

- `scorecard-permission-policy/`
- `convi-6862-disable-editing-on-submitted-scorecard/`
- `convi-7237/`

## Open Questions

- Complete the centralized capability/visibility evaluator beyond submitted editing.
- Define consistent enforcement for comments, exports, and notifications.
