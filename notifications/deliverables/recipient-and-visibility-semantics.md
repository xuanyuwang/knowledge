# Recipient and Visibility Semantics

## Separate the Decisions

Do not treat one template role list as the complete notification policy. Recipient eligibility can depend on:

- static template role eligibility;
- requester/recipient identity and relationship to the scorecard;
- scorecard type and lifecycle state;
- template revision used by the scorecard;
- submission vs publish transition;
- agent-facing visibility feature flags;
- workflow-specific creator/task/audience constraints.

The permission design should expose distinct capabilities such as `RECEIVE_SUBMIT_NOTIFICATION` and `RECEIVE_PUBLISH_NOTIFICATION` rather than infer them from a generic read boolean.

## `scorecard_viewers`

Current evidence describes `HasScorecardViewPermission` as notification-oriented:

- default roles include administrative, QA, manager, and agent roles;
- non-empty `scorecard_viewers` replaces the default template role list;
- SUPER_ADMIN remains allowed;
- notification checks must use the scorecard's own template revision because later permission revisions should not silently rewrite historical scorecard policy.

`scorecard_viewers` is not a complete runtime read policy. API visibility also depends on agent ownership, submitted/published state, auto/manual scoring, and feature flags.

## Submit and Publish

- When explicit publish is required, submit suppresses the agent notification until publish.
- When publish behavior is enabled but the template does not require a separate publisher, submit may transition/notify as published.
- Publish includes the agent only when runtime/template view permission allows it.
- Appeal scorecards are not independently publishable under the documented design.

These rules mean trigger state and recipient eligibility must be evaluated together.

## Group Calibration and Appeals

- Group-calibration response ownership uses creator/task-audience constraints in the workflow domain.
- Appeals have distinct request/resolve roles and should not inherit normal publish behavior blindly.
- No notification-specific group-calibration evidence was found in the migrated folder; treat that as an inventory gap rather than assuming no notifications exist.

## Ownership Boundary

- `scorecard-workflows` owns capability and visibility semantics.
- `notifications` owns how those semantics are consumed to select recipients and deliver messages.

Changes to either side should update both domains through links, not duplicated competing rules.

## Review Checklist

- [ ] Is the trigger submit, publish, appeal, calibration, or another transition?
- [ ] Which template revision supplies policy?
- [ ] Is the recipient statically eligible by role?
- [ ] Does runtime state/relationship permit visibility now?
- [ ] Is the notification suppressed in favor of a later transition?
- [ ] Are channel, retry, and dedupe behavior independent of eligibility?
- [ ] Do tests cover permission changes after scorecard creation?
