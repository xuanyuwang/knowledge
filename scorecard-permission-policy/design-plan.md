# Design Plan

Created: Jun 5, 2026
Updated: Jun 16, 2026

## First Step

The first implementation step should focus only on the submitted-scorecard edit lock scenario from [CONVI-6862 - Disable Editing on Submitted Scorecard](../convi-6862-disable-editing-on-submitted-scorecard/).

The goal is not to solve every scorecard permission scenario yet. The goal is to centralize the existing CONVI-6862 permission decision behind a small evaluator interface that can later be used by `EvaluateScorecardsPermissions` with a batch-friendly v1 permission enum.

The API should use one `ScorecardPermission` enum instead of separate capability and visibility concepts. The enum can include broader scorecard permissions now, while the first implementation can centralize the submitted-scorecard edit-lock behavior first.

## CONVI-6862 Scope

### In Scope

- Normal/original scorecards.
- Normal Closed Conversations scorecards.
- Normal process scorecards.
- Submitted-scorecard edit lock for:
  - criteria value edits,
  - criterion comments,
  - general notes edits,
  - reset scorecard.
- Template-level post-submit exceptions through `submitted_scorecard_editors` / `submittedScorecardEditors`.
- Submitted-editor membership shape:
  - users,
  - teams,
  - groups.

### Out Of Scope

- First submit of an unsubmitted scorecard.
- Appeal request scorecards.
- Appeal resolve scorecards.
- Group calibration answer key scorecards.
- Group calibration response scorecards.
- Full publish permission semantics.
- Full scorecard read visibility semantics.
- Notification visibility.
- Score/comment field visibility.
- Analytics eligibility.
- Template-only permission evaluation.
- Full non-CONVI-6862 permission implementations in `EvaluateScorecardsPermissions`.

## Design Principles

- Preserve current CONVI-6862 behavior. Centralization must not silently change who can edit submitted scorecards.
- Use existing service structs as evaluator inputs. Do not introduce normalized requester/template/scorecard structs in this first slice.
- Keep a small evaluator interface for the permission component and its tests, but do not add artificial dependency-injection layers around simple local helpers.
- Keep the permission result minimal: permission allowed or denied.
- Keep implementation small. The first slice should live near existing scorecard domain helpers; no separate `hydrate.go` or `reason.go`.
- Reuse existing helper logic where possible instead of reimplementing permission semantics from scratch.

## Naming

Avoid using `Policy` in new type/package names. In Cresta, "Policy" is already strongly associated with Opera Policy, so names like `ScorecardPolicyEvaluator` and `scorecardpolicy` are likely to create confusion.

Phase 1 implemented names:

| Concept | Implemented name | Notes |
|---|---|---|
| Go package | `apiserver/internal/coaching/scorecards` | Reuses the existing scorecards domain package instead of creating a parallel permission package. |
| Interface | `ScorecardPermissionEvaluator` | Clear contract for evaluator tests and future direct call sites. |
| Default implementation | `DefaultScorecardPermissionEvaluator` | Conventional concrete implementation name. |
| Proto decision message | `ScorecardPermissionDecision` | Matches the public API and avoids `PolicyDecision`. |
| Proto result message | `ScorecardPermissionResult` | One scorecard's evaluated permissions. |

Other reasonable names considered:

| Name | Assessment |
|---|---|
| `scorecardaccess` / `ScorecardAccessEvaluator` | Good for read/write access, but less precise once the API includes specific permissions like grade, appeal, and publish. |
| `scorecardauthorization` / `ScorecardAuthorizationEvaluator` | Accurate but heavier, and can sound like platform auth rather than scorecard-domain permission rules. |
| `scorecardpermissioncheck` / `ScorecardPermissionChecker` | Clear for a boolean check, but weaker for a batch evaluator that returns multiple permission decisions. |
| `scorecardcapability` / `ScorecardCapabilityEvaluator` | Too narrow now that capability and visibility concepts are represented by one `ScorecardPermission` enum. |

This document should use "permission evaluator" or "permission layer" for the new component. It can still use "template policies" when referring to existing template configuration as an input factor.

## Permission Semantics

The first permission evaluator answers one question:

> Can this requester modify this scorecard through a submitted-lock-protected operation?

The current behavior should be preserved:

| Condition | Result |
|---|---|
| Scorecard is not in submitted-editor scope | Fall back to existing edit permission. |
| Scorecard is unsubmitted | Fall back to existing edit permission. |
| Scorecard is submitted normal/original | Apply submitted-editor permission if configured. |
| Submitted editors are unset, empty, or whitespace-only | Fall back to existing edit permission. |
| Submitted editors are configured and requester is in resolved users/teams/groups | Allow. |
| Submitted editors are configured and requester is not in resolved users/teams/groups | Deny. |
| Scorecard is appeal or calibration type | Fall back to existing behavior; not part of this first slice. |

Current backend behavior intentionally allows an explicitly configured submitted-editor allowlist to deny users who would otherwise pass role-based edit checks. This should remain true.

## Evaluator Interface

The evaluator lives in `apiserver/internal/coaching/scorecards`. Phase 1 keeps the interface with the evaluator so tests and future call sites can depend on the contract, but the existing coaching service remains behind a thin compatibility adapter. That adapter constructs the default evaluator from existing `ServiceImpl` dependencies and converts the result back to the existing boolean shape.

No separate `EditPermissionChecker` or template-permission converter type is needed. The default evaluator directly reuses `scorecards.HasScorecardEditPermission` for fallback behavior, and DB-to-service template permission conversion is centralized in `shared/scoring`.

```go
type ScorecardPermissionEvaluator interface {
    Evaluate(
        ctx context.Context,
        requester *userpb.User,
        scorecard *dbmodel.Scorecards,
        template *dbmodel.ScorecardTemplates,
        permissions []coachingpb.ScorecardPermission,
    ) (*coachingpb.ScorecardPermissionResult, error)
}

type DefaultScorecardPermissionEvaluator struct {
    userFilterParser      userfilter.UserFilterParser
    authUserServiceClient userpb.UserServiceClient
    configServiceClient   config.Client
    resourceACLHelper     auth.ResourceACLHelper
    logger                logger.Logger
}

func (e *DefaultScorecardPermissionEvaluator) Evaluate(
    ctx context.Context,
    requester *userpb.User,
    scorecard *dbmodel.Scorecards,
    template *dbmodel.ScorecardTemplates,
    permissions []coachingpb.ScorecardPermission,
) (*coachingpb.ScorecardPermissionResult, error)
```

The first implementation fully supports `SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD`. Unsupported permission values currently return stable denied decisions in the result.

## RPC Design

The first public API should keep the general RPC shape while prioritizing the CONVI-6862 permission implementation.

```protobuf
rpc EvaluateScorecardsPermissions(EvaluateScorecardsPermissionsRequest)
    returns (EvaluateScorecardsPermissionsResponse);

enum ScorecardPermission {
  SCORECARD_PERMISSION_UNSPECIFIED = 0;
  SCORECARD_PERMISSION_VIEW = 1;
  SCORECARD_PERMISSION_EDIT = 2;
  SCORECARD_PERMISSION_GRADE = 3;
  SCORECARD_PERMISSION_APPEAL = 4;
  SCORECARD_PERMISSION_PUBLISH = 5;

  // Can modify a submitted-lock-protected scorecard through update/reset style operations.
  SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD = 6;
}

message EvaluateScorecardsPermissionsRequest {
  string parent = 1 [
    (google.api.resource_reference).type = "cresta.v1.customer.Profile",
    (google.api.field_behavior) = REQUIRED
  ];

  repeated string scorecard_names = 2 [
    (google.api.resource_reference).type = "cresta.v1.coaching.Scorecard",
    (google.api.field_behavior) = REQUIRED
  ];

  repeated ScorecardPermission permissions = 3 [
    (google.api.field_behavior) = REQUIRED
  ];

  string requester_user_name = 4 [
    (google.api.resource_reference).type = "cresta.v1.user.User",
    (google.api.field_behavior) = REQUIRED
  ];
}

message EvaluateScorecardsPermissionsResponse {
  repeated ScorecardPermissionResult results = 1;
}

message ScorecardPermissionResult {
  string scorecard_name = 1;
  repeated ScorecardPermissionDecision permissions = 2;
}

message ScorecardPermissionDecision {
  ScorecardPermission permission = 1;
  bool allowed = 2;
}
```

There is no separate visibility output. Read and visibility-related decisions should be represented as `ScorecardPermission` enum values when they move into the permission layer.

## Implementation Shape

Phase 1 implementation location:

```text
go-servers/apiserver/internal/coaching/scorecards/
```

Files:

- `permission_evaluator.go`: `ScorecardPermissionEvaluator`, `DefaultScorecardPermissionEvaluator`, submitted-editor scope helpers, submitted-editor user-filter conversion, and `DecisionAllowed`.
- `permission_evaluator_test.go`: table-driven permission tests for CONVI-6862 behavior.
- `edit_permission.go`: moved `HasScorecardEditPermission` and `DefaultEditPermissionRoles`.
- `submitted_scorecard_permissions.go`: compatibility adapter in the coaching package that calls the new evaluator for existing call sites.
- `shared/scoring/scorecard_template_permissions.go`: shared `ConvertNullStringToServiceScorecardTemplatePermissions`.

The evaluator moved the existing submitted-scorecard editor logic:

- submitted-scorecard scope check,
- configured submitted-editor detection,
- user/team/group conversion to user-filter conditions,
- membership expansion through the shared user-filter parser,
- fallback to `HasScorecardEditPermission` when submitted-editor permission does not apply.

The fallback edit-permission helper moved into the `scorecards` package because it is scorecard-domain behavior, not a generic coaching service utility. The template permissions converter moved into `shared/scoring` because it is reused by both scoring/transformer code and the scorecard permission evaluator.

Phase 1 also updates both dependency systems to the proto version that contains the new permission messages:

- `go.mod` / `go.sum`: `github.com/cresta/cresta-proto/v2 v2.3.18`
- `deps.bzl`: `com_github_cresta_cresta_proto_v2` pinned to `v2.3.18`

## Call Sites

### `UpdateScorecard`

Existing update flows still call the coaching helper. In phase 1 that helper is now a thin adapter around `ScorecardPermissionEvaluator.Evaluate(..., [SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD])` before applying submitted-lock-protected changes.

This covers:

- criteria value edits,
- criterion comments,
- general notes edits.

### `ResetScorecard`

Existing reset flows still call the coaching helper. In phase 1 that helper delegates to `ScorecardPermissionEvaluator.Evaluate(..., [SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD])` before reset work starts.

Reset is explicitly part of the CONVI-6862 submitted-lock scope.

### `SubmitScorecard`

Do not change first-submit behavior in this first step.

Submit remains governed by existing submit/edit permission behavior.

## Testing Matrix

Minimum evaluator tests:

| Case | Expected |
|---|---|
| Unsubmitted normal scorecard, requester passes existing edit permission | Allowed by fallback. |
| Submitted normal scorecard, submitted editors unset | Uses existing edit permission fallback. |
| Submitted normal scorecard, submitted editors empty | Uses existing edit permission fallback. |
| Submitted normal scorecard, submitted editors whitespace-only | Uses existing edit permission fallback. |
| Submitted normal scorecard, direct requester user configured | Allowed. |
| Submitted normal scorecard, requester included through configured team | Allowed. |
| Submitted normal scorecard, requester included through configured group | Allowed. |
| Submitted normal scorecard, editors configured but requester not included | Denied even if requester has normal edit role. |
| Missing/invalid submitted-editor resource name | Returns error consistent with current parser behavior. |
| Appeal request scorecard | Falls back to existing behavior; submitted-editor permission not applied. |
| Appeal resolve scorecard | Falls back to existing behavior; submitted-editor permission not applied. |
| Group calibration answer key scorecard | Falls back to existing behavior; submitted-editor permission not applied. |
| Group calibration response scorecard | Falls back to existing behavior; submitted-editor permission not applied. |

The phase-1 implementation keeps existing service behavior covered through the compatibility adapter and focused evaluator tests. Handler tests with a fake evaluator become useful when the RPC or service wiring starts depending on the evaluator interface directly.

## Migration Plan

### Phase 1: Extract CONVI-6862 Permission Logic

- Add `scorecards.ScorecardPermissionEvaluator`.
- Add `scorecards.DefaultScorecardPermissionEvaluator`.
- Move the submitted-scorecard editor helper logic into `apiserver/internal/coaching/scorecards`.
- Move `HasScorecardEditPermission` and `DefaultEditPermissionRoles` into `apiserver/internal/coaching/scorecards`.
- Move `convertNullStringToServiceScorecardTemplatePermissions` into `shared/scoring` as `ConvertNullStringToServiceScorecardTemplatePermissions`.
- Keep `apiserver/internal/coaching/submitted_scorecard_permissions.go` as a thin compatibility adapter for existing call sites.
- Keep behavior identical.
- Add evaluator tests.
- Update `cresta-proto` to `v2.3.18` in both `go.mod` and `deps.bzl`.

### Phase 2: Add V1 RPC And Direct Evaluator Wiring

- Add `EvaluateScorecardsPermissions` proto in `cresta/v1/coaching/coaching_service.proto`.
- Add `ScorecardPermission` in `cresta/v1/coaching/scorecard_permission.proto`.
- Implement request hydration for requester, scorecards, and templates.
- Call `ScorecardPermissionEvaluator.Evaluate` for each requested scorecard.
- Return permission decisions with inline `allowed` booleans.
- Add RPC tests for self/authorized requester evaluation and mixed allow/deny scorecard results.
- Move existing service call sites from the compatibility adapter to direct evaluator wiring only when that reduces duplication or supports the RPC wiring.

### Phase 3: Stabilize Before Expanding

Do not fully implement grade/appeal/publish/read visibility semantics in this evaluator in the first step. After CONVI-6862 is centralized and stable, implement the next permission and migrate the next scenario.

## Deferred Work

The previous broader design direction remains valid as a later phase, but it should not block this first step.

Deferred topics:

- Permission semantics for grade, appeal, publish, and acknowledge.
- Permission semantics for scorecard read, score read, comments, submit notifications, publish notifications.
- Publish state and agent visibility.
- Appeal request vs appeal resolve permission rules.
- Group calibration creator/task-audience permission rules.
- Comment-level visibility.
- Analytics eligibility remains out of scope unless a separate analytics project is opened.
