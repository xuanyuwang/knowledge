# Design Plan

Created: Jun 5, 2026

## Design Principles

- Make the policy layer a pure domain evaluator after inputs are hydrated.
- Keep hydration separate from evaluation: callers can pass already-loaded requester/template/scorecard state, while the RPC can hydrate for clients.
- Return structured allow/deny results with reasons. Avoid forcing clients to reverse-engineer denial causes from gRPC errors.
- Preserve current behavior first, then tighten semantics in later phases.
- Batch by default; permission checks will be called from list views and notifications.
- Treat template-only evaluation as out of scope for the first API. Scorecard state is required for publish, submit, and visibility answers.
- Keep analytics eligibility out of scope. Analytics inclusion rules should remain in analytics-specific code unless a separate project takes them on.

## Domain Model

### Requester Context

Internal evaluator input:

```go
type ScorecardPolicyRequester struct {
    UserName string
    UserID string
    Roles []authpb.AuthProto_Role
    IsServer bool
    IsSuperAdmin bool
    IsAgentOnly bool
    IsAPIKey bool
    ManagedUserIDs set.Set[string]
    ManagedGroupIDs set.Set[string]
    Memberships UserMembershipSnapshot
}
```

Notes:

- `ManagedUserIDs` and `ManagedGroupIDs` should come from Resource ACL when enabled.
- `Memberships` is needed for `submitted_scorecard_editors`, template audience, and task audience checks.
- SUPER_ADMIN / server bypass should be explicit policy, not scattered helper behavior.

### Template Policy

Normalize DB/service template fields into:

```go
type ScorecardTemplatePolicy struct {
    TemplateName string
    TemplateType coachingpb.ScorecardTemplateType
    TemplateStatus coachingpb.ScorecardTemplateStatus
    Audience *coachingpb.Audience
    Permissions *coachingpb.ScorecardTemplate_Permissions
    QATaskConfig *qapb.QATaskConfig
    Structure scoring.ScorecardTemplateStructure
}
```

### Scorecard State

Normalize DB/PB scorecard fields into:

```go
type ScorecardPolicyState struct {
    ScorecardName string
    AgentUserName string
    CreatorUserName string
    SubmitterUserName string
    PublisherUserName string
    ScorecardType coachingpb.ScorecardType
    Submitted bool
    Published bool
    Acknowledged bool
    AIScored bool
    ManuallyScored bool
    Calibrated bool
    ReferenceScorecardID string
    TaskNames []string
    SubmissionSource coachingpb.ScorecardSubmissionSource
    UsecaseName string
}
```

## Policy Outputs

Use stable enum keys, not ad-hoc booleans, so clients can request only what they need and future capabilities do not require a new top-level shape.

```protobuf
enum ScorecardCapability {
  SCORECARD_CAPABILITY_UNSPECIFIED = 0;
  SCORECARD_CAPABILITY_GRADE = 1;
  SCORECARD_CAPABILITY_SUBMIT = 2;
  SCORECARD_CAPABILITY_CREATE_APPEAL = 3;
  SCORECARD_CAPABILITY_RESOLVE_APPEAL = 4;
  SCORECARD_CAPABILITY_PUBLISH = 5;
  SCORECARD_CAPABILITY_ACKNOWLEDGE = 6;
}

enum ScorecardVisibility {
  SCORECARD_VISIBILITY_UNSPECIFIED = 0;
  SCORECARD_VISIBILITY_READ_SCORECARD = 1;
  SCORECARD_VISIBILITY_READ_SCORES = 2;
  SCORECARD_VISIBILITY_READ_SCORE_COMMENTS = 3;
  SCORECARD_VISIBILITY_RECEIVE_SUBMIT_NOTIFICATION = 4;
  SCORECARD_VISIBILITY_RECEIVE_PUBLISH_NOTIFICATION = 5;
}

message ScorecardPolicyDecision {
  bool allowed = 1;
  repeated ScorecardPolicyReason reasons = 2;
}

message ScorecardPolicyReason {
  ScorecardPolicyReasonCode code = 1;
  string message = 2;
}
```

Suggested reason codes:

```protobuf
enum ScorecardPolicyReasonCode {
  SCORECARD_POLICY_REASON_CODE_UNSPECIFIED = 0;
  SCORECARD_POLICY_REASON_CODE_ALLOWED_BY_SERVER_OR_SUPER_ADMIN = 1;
  SCORECARD_POLICY_REASON_CODE_ALLOWED_BY_ROLE = 2;
  SCORECARD_POLICY_REASON_CODE_ALLOWED_BY_SUBMITTED_EDITOR = 3;
  SCORECARD_POLICY_REASON_CODE_ALLOWED_BY_SCORECARD_AGENT = 4;
  SCORECARD_POLICY_REASON_CODE_ALLOWED_BY_UNMODIFIED_AUTO_SCORE = 5;
  SCORECARD_POLICY_REASON_CODE_DENIED_ROLE_NOT_ALLOWED = 100;
  SCORECARD_POLICY_REASON_CODE_DENIED_NOT_SCORECARD_AGENT = 101;
  SCORECARD_POLICY_REASON_CODE_DENIED_NOT_SUBMITTED = 102;
  SCORECARD_POLICY_REASON_CODE_DENIED_ALREADY_PUBLISHED = 103;
  SCORECARD_POLICY_REASON_CODE_DENIED_NOT_PUBLISHED = 104;
  SCORECARD_POLICY_REASON_CODE_DENIED_NOT_ORIGINAL_SCORECARD = 105;
  SCORECARD_POLICY_REASON_CODE_DENIED_NOT_CONVERSATION_TEMPLATE = 106;
  SCORECARD_POLICY_REASON_CODE_DENIED_NOT_CREATOR = 107;
  SCORECARD_POLICY_REASON_CODE_DENIED_NOT_TASK_AUDIENCE = 108;
}
```

## RPC Design

### Request

```protobuf
message BatchEvaluateScorecardPermissionsRequest {
  string parent = 1 [
    (google.api.resource_reference).type = "cresta.v1.customer.Profile",
    (google.api.field_behavior) = REQUIRED
  ];

  repeated string scorecard_names = 2 [
    (google.api.resource_reference).type = "cresta.v1.coaching.Scorecard",
    (google.api.field_behavior) = REQUIRED
  ];

  repeated ScorecardCapability capabilities = 3;
  repeated ScorecardVisibility visibilities = 4;

  // Optional. Defaults to the authenticated user. Server callers can evaluate
  // on behalf of a user without minting a separate user token.
  string requester_user_name = 5 [
    (google.api.resource_reference).type = "cresta.v1.user.User",
    (google.api.field_behavior) = OPTIONAL
  ];

  // Optional knobs for compatibility with existing rollout behavior.
  ScorecardPermissionEvaluationOptions options = 6;
}

message ScorecardPermissionEvaluationOptions {
  // If unset, the server reads the current feature flag value.
  optional bool enable_scorecard_publish = 1;

  // Include detailed reason messages. Codes should always be returned.
  bool include_reason_messages = 2;
}
```

### Response

```protobuf
message BatchEvaluateScorecardPermissionsResponse {
  repeated ScorecardPermissionResult results = 1;
}

message ScorecardPermissionResult {
  string scorecard_name = 1;
  map<int32, ScorecardPolicyDecision> capabilities = 2;
  map<int32, ScorecardPolicyDecision> visibilities = 3;
}
```

Proto map keys are `int32` because protobuf does not allow enum keys in maps. Generated clients can wrap this into typed helpers.

### Auth Annotation

The RPC itself should allow the superset of roles that can currently call scorecard read/list/action APIs. Domain results decide actual capability.

Initial service annotation should include:

- AGENT
- MANAGER
- MANAGER_2ND
- ADMIN
- SUPER_ADMIN
- QA_ADMIN
- QA_SPECIALIST
- PROCESS_SPECIALIST

For `requester_user_name` impersonation, require server/super-admin/API-key or a dedicated internal permission.

## Evaluation Semantics

### Grade

Equivalent to current normal-scorecard edit/update capability:

- Server/super-admin allowed.
- Normal scorecard uses `scorecard_graders` if configured; otherwise default normal-scorecard edit roles.
- Submitted normal scorecard uses `submitted_scorecard_editors` when configured, otherwise falls back to normal edit roles.
- Group calibration response requires creator and task audience.
- Appeal request/resolve should not return `GRADE`; they should use appeal-specific capabilities.

### Submit

Preserve current behavior initially:

- Reuse edit permission by scorecard type.
- Group calibration response requires creator.
- Appeal resolve submit is allowed only if requester can resolve appeal.

Future tightening:

- Split submit from grade in template policy if product needs "can draft but cannot submit".

### Appeal

Split into two capabilities:

- `CREATE_APPEAL`: current appeal request creation. Uses `scorecard_appealers` if configured, otherwise current default appeal-request roles.
- `RESOLVE_APPEAL`: current appeal resolve creation/submission. Uses default resolve roles until template gets a dedicated field.

The API can expose both while UI can collapse them into one "appeal" affordance when needed.

### Publish

Allowed only when:

- Requester has a role in `scorecard_publisher_roles`.
- Scorecard is submitted.
- Scorecard is not already published.
- Scorecard is original/normal.
- Template type is conversation.

If `scorecard_publisher_roles` is empty:

- `PUBLISH` is denied because explicit publish is not required/supported for that template.
- Submit may auto-publish when `enableScorecardPublish` is enabled.

### Read Scorecard

Initial compatibility rules:

- Server/super-admin allowed.
- Non-agent-only QA/manager/admin roles are allowed by existing endpoint authorization and list filters.
- Agent-only requester must be the scorecard agent.
- With `enableScorecardPublish = true`, agent-only read requires published or unmodified auto-scored.
- With `enableScorecardPublish = false`, agent-only read requires submitted or unmodified auto-scored.

Open design issue:

- Decide whether `scorecard_viewers` should become a true read policy for all roles. Today it is primarily notification-oriented.

### Read Comments

Return a separate visibility decision per score comment in later versions. V1 can return an aggregate:

- Allowed when every returned score comment is visible to requester.
- Partially allowed should be represented by field-level scrubbing in the caller until the response supports per-score decisions.

### Analytics Eligibility

Out of scope. The scorecard permission policy will not expose analytics eligibility decisions in the first API.

## Internal Package Shape

Suggested first implementation location:

```text
go-servers/apiserver/internal/coaching/scorecardpolicy/
```

Files:

- `types.go`: normalized requester/template/scorecard inputs and result types.
- `evaluator.go`: pure capability and visibility decisions.
- `hydrate.go`: DB/user/ACL/template loading helpers for RPC/service integration.
- `reasons.go`: stable reason code mapping.
- `evaluator_test.go`: table-driven policy matrix tests.

The existing service handlers can use the internal evaluator before the RPC is exposed.

## Migration Plan

### Phase 1: Internal Evaluator

- Add `scorecardpolicy` package with table-driven tests that encode current behavior.
- Port existing helpers into evaluator semantics without changing public behavior:
  - `HasScorecardEditPermission`
  - `HasScorecardViewPermission`
  - `hasScorecardPublishPermission`
  - `templateRequiresPublish`
  - submitted-scorecard editor checks
  - agent visibility checks

### Phase 2: Replace Local Checks

Replace call sites incrementally:

- `UpdateScorecard`: use `GRADE`.
- `SubmitScorecard`: use `SUBMIT`.
- `CreateScorecard` appeal paths: use `CREATE_APPEAL` / `RESOLVE_APPEAL`.
- `PublishScorecard`: use `PUBLISH`.
- `GetScorecard` / `ListScorecards`: use `READ_SCORECARD` where practical, keeping SQL filtering for list performance.
- Notification recipients: use notification visibility decisions.

### Phase 3: Batch RPC

- Add proto in `cresta/v1/coaching/coaching_service.proto`.
- Implement batch hydration and evaluator calls.
- Add API tests covering mixed scorecard states and requester roles.
- Keep response stable and additive.

### Phase 4: Product Semantics Cleanup

Resolve whether to add explicit template policy fields for:

- Submitters, separate from graders.
- Appeal resolvers, separate from default QA/admin roles.
- True scorecard readers, separate from notification recipients.

## Testing Matrix

Minimum evaluator test cases:

| Case | Expected |
|---|---|
| Normal draft, default template, manager | grade+submit allowed. |
| Normal draft, `scorecard_graders=[QA_ADMIN]`, manager | grade+submit denied. |
| Submitted normal, no submitted editors, QA specialist | grade allowed by fallback. |
| Submitted normal, submitted editors configured without requester | grade denied. |
| Submitted normal, submitted editors include requester via group | grade allowed. |
| Appeal request, `scorecard_appealers=[INSIGHTS_VIEWER]`, insights viewer | create appeal allowed. |
| Appeal resolve, manager | resolve denied. |
| Group calibration response, non-creator | grade/submit denied. |
| Publish before submit | publish denied not submitted. |
| Publish already published | publish denied already published. |
| Publish process template | publish denied not conversation template. |
| Agent read own published scorecard | read allowed. |
| Agent read own unpublished manual scorecard with publish flag on | read denied/not found compatible. |
| Agent read own unmodified auto scorecard | read allowed. |

## Open Questions

- Should `scorecard_viewers` become canonical scorecard read permission, or remain notification/audience policy?
- Do we need separate `can_submit` template config, or is submit intentionally tied to grade/edit?
- Do appeal request and appeal resolve need separate template config?
- How should policy represent partial score/comment visibility without forcing score payload hydration?
- Should API callers be able to evaluate permissions for another user, and which auth method should gate that?
