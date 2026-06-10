# Scorecard Permission Policy

Authors: xuanyu.wang@cresta.ai

Status: Draft

Created: Jun 5, 2026

## Goal

Design a centralized, canonical, small domain policy layer for scorecard permissions.

Today scorecard permission checks are spread across service handlers, helper functions, and notification code. Each path answers a slightly different question with slightly different inputs. As the permission model grows, we need a single policy surface that can answer:

- **Capabilities**: grade, submit, appeal, publish.
- **Visibilities**: whether a scorecard, score details, comments, and notifications are visible to a requester.

The policy should evaluate from:

- **Requester identity**: user resource name, roles, auth principal type, user/team/group membership, ACL scope.
- **Template policies**: template type/status/audience and `ScorecardTemplate.Permissions`.
- **Scorecard state**: scorecard type, submitted/published state, manual/AI scoring state, creator/agent/submitter/publisher, calibration/appeal relationships, task membership.

## Non-goals

- Analytics eligibility is out of scope for this project. Analytics code has related scorecard inclusion rules, but those rules should stay owned by analytics until a separate analytics-specific design is started.

## Documents

- [investigation.md](investigation.md): current permission checks and decision factors found in code.
- [permission-history.md](permission-history.md): evolution history for each permission and why behavior changed.
- [design-plan.md](design-plan.md): proposed policy layer, protobuf/API sketch, semantics, and migration plan.

## Core Finding

The user-facing scorecard template JSON structure (`ScorecardTemplateStructureV2`) defines the scoring form, criteria, comments, Auto QA, and scoring-related flags. The permission configuration is not inside `ScorecardTemplateStructureV2`; it lives on `ScorecardTemplate.permissions` / `ScorecardTemplate.Permissions`.

The policy layer should therefore normalize three separate concepts before evaluating:

- Template structure policy, such as criterion-level comment and scoring behavior.
- Template permission policy, such as viewers/graders/appealers/publishers/submitted editors.
- Runtime scorecard state, such as draft/submitted/published/appeal/calibration.

## Proposed Deliverable

Add a scorecard policy evaluator in coaching service first, then expose it through a batch RPC after the internal semantics are stable.

The first public API should answer scorecard-level policy only:

```protobuf
rpc BatchEvaluateScorecardPermissions(BatchEvaluateScorecardPermissionsRequest)
    returns (BatchEvaluateScorecardPermissionsResponse);
```

Template-only permission evaluation is intentionally out of scope for the first design pass because scorecard state changes capability and visibility results.
