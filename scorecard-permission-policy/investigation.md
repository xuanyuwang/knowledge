# Investigation

Created: Jun 5, 2026

## Source Map

Primary code paths reviewed:

- `go-servers/apiserver/internal/coaching/action_create_scorecard.go`
- `go-servers/apiserver/internal/coaching/action_update_scorecard.go`
- `go-servers/apiserver/internal/coaching/action_submit_scorecard.go`
- `go-servers/apiserver/internal/coaching/action_publish_scorecard.go`
- `go-servers/apiserver/internal/coaching/action_get_scorecard.go`
- `go-servers/apiserver/internal/coaching/action_list_scorecards.go`
- `go-servers/apiserver/internal/coaching/submitted_scorecard_permissions.go`
- `go-servers/apiserver/internal/coaching/util.go`
- `go-servers/apiserver/internal/coaching/common_scorecard.go`
- `go-servers/shared/clickhouse/conversations/scorecard_score.go`
- `go-servers/shared/clickhouse/conversations/conversation.go`
- `cresta-proto/cresta/v1/coaching/scorecard_template.proto`
- `cresta-proto/cresta/v1/coaching/scorecard.proto`

Analytics code was reviewed only as background context. Analytics eligibility is out of scope for this project.

## Template Permission Shape

`ScorecardTemplate.Permissions` currently contains:

| Field | Current intent |
|---|---|
| `template_editors` | Roles that can edit scorecard templates. |
| `scorecard_viewers` | Roles that can view scorecard-related notifications / visibility-sensitive surfaces. |
| `scorecard_graders` | Roles that can manually create/update/submit normal scorecards. |
| `scorecard_appealers` | Roles that can create appeal request scorecards. |
| `scorecard_publisher_roles` | Roles that can publish scorecards; non-empty also means the template requires explicit publish. |
| `submitted_scorecard_editors` | Users/teams/groups allowed to update a submitted normal scorecard. |

The DB stores permissions as a JSON-serialized DB proto and converts to the service proto through `convertNullStringToServiceScorecardTemplatePermissions`.

## Template Structure Policy Shape

`ScorecardTemplateStructureV2` is the scoring form shape. It is still relevant to the policy layer because it has field-level behavior, even though it does not hold role permissions:

| Structure field / method | Current policy relevance |
|---|---|
| `displayCommentField` | Template/criterion-level comment UI behavior. |
| `CommentSettings.RequiredForValues` | Submission/grade validation may require comments for selected values. |
| `CommentSettings.RequiredOnOverride` | Submission/grade validation may require comments when overriding Auto QA. |
| `ExcludeOutcomeInsights` | Analytics-specific; noted for context only and out of scope for the permission policy. |
| `ExcludeFromQAScores` | Analytics/scoring-specific; noted for context only and out of scope for the permission policy. |
| `AutoQA` / triggers | Drives AI-scored/unmodified auto-score behavior relevant to agent visibility. |
| `perMessage` | A scorecard/criterion may be scoped to messages instead of the whole conversation. |
| `branches` | Active criteria can depend on parent answers, affecting score/comment visibility. |

The policy layer should treat these as structure-level policies, distinct from role-based `ScorecardTemplate.Permissions`.

## Scorecard State Factors

Current checks use these scorecard fields as policy inputs:

| Factor | Where it matters |
|---|---|
| `scorecard_type` | Selects permission role set and workflow semantics. Normal, appeal request, appeal resolve, group calibration answer key/response are treated differently. |
| `submitted_at` / `submitter_user_id` | Required before publish; used for submitted-editor restrictions. |
| `published_at` / `publisher_user_id` | Required for agent visibility when `enableScorecardPublish` is enabled; blocks duplicate publish. |
| `ai_scored_at` + `manually_scored` | Agent visibility permits unmodified auto-scored scorecards even if not submitted/published. |
| `agent_user_id` | Agent-only get/list/acknowledge access is scoped to the agent's own scorecards. |
| `creator_user_id` | Group calibration responses can only be updated/submitted by creator; notifications use creator. |
| `calibrated_scorecard_id` | Calibration scorecards are excluded from many list/write paths. |
| `reference_scorecard_id` | Appeal workflow relationship: original -> replica -> request -> resolve. |
| `task_ids` | Group calibration response creation uses task/audience constraints. |
| `template_id` / `template_revision` | Pulls the template policy and scoring structure used by evaluation. |
| `template type` | Publish currently only supports conversation templates. Process templates have different write behavior. |

## Requester Factors

Current checks use:

| Factor | Current source |
|---|---|
| Authenticated principal | `authEntityProvider.GetAuthEntity`. |
| User resource name / ID | `GetClaimUserResourceName`, `GetClaimUserID`, `ToAPIUser`. |
| Roles | `user.User.Roles`, `PrincipalEntity.HasAnyRole`, and helper `userUtils.HasAnyRole`. |
| Server / super admin bypass | `PrincipalEntity.IsServerOrSuperAdmin`; some helper functions also hard-code SUPER_ADMIN as allowed. |
| Agent-only classification | `PrincipalEntity.IsAgentOnly`. |
| QA-approved roles | `hasQAApprovedRole` gives broad list/filter access for QA/admin/manager roles. |
| Resource ACL | Used by user filtering and submitted-editor expansion through user-filter parser. |
| User/team/group membership | Used for submitted-scorecard editors, task audiences, template audiences, and group filters. |

## Current Capability Behavior

### Grade / Create / Update / Submit

`HasScorecardEditPermission` is the central helper for create/update/submit, but its meaning varies by scorecard type:

| Scorecard type | Default roles | Template override |
|---|---|---|
| Normal / unspecified | ADMIN, QA_ADMIN, QA_SPECIALIST, MANAGER, MANAGER_2ND | `scorecard_graders`, if non-empty. |
| Appeal request | MANAGER, MANAGER_2ND, ADMIN | `scorecard_appealers`, if non-empty. |
| Appeal resolve | QA_ADMIN, ADMIN | No template override. |
| Group calibration answer key | QA_ADMIN, ADMIN | No template override. |
| Group calibration response | QA_ADMIN, QA_SPECIALIST, ADMIN, MANAGER, MANAGER_2ND | No template override; creator and task-audience checks are added elsewhere. |

Important deviations:

- `SubmitScorecard` uses `HasScorecardEditPermission` directly, so submit currently reuses grade/edit permission.
- `UpdateScorecard` adds `submitted_scorecard_editors` only for submitted normal scorecards.
- Group calibration response update/submit also requires requester to be the creator.
- Group calibration response create also requires requester to be in the task audience.

### Appeal

Appeals are modeled as scorecards:

- Appeal request creation checks `HasScorecardEditPermission(..., SCORECARD_TYPE_APPEAL_REQUEST)`.
- Appeal resolve creation checks `HasScorecardEditPermission(..., SCORECARD_TYPE_APPEAL_RESOLVE)`.
- Appeal resolve submit mutates the original scorecard/scores as part of submit.

There is no single "appeal" decision today; request and resolve are separate scorecard types with separate default roles.

### Publish

`PublishScorecard` checks all of:

- Scorecard exists.
- Scorecard has `submitted_at` and `submitter_user_id`.
- Scorecard is not already published.
- Scorecard is an original normal scorecard.
- Template is a conversation template.
- Requester has any role in `scorecard_publisher_roles`.

`scorecard_publisher_roles` has two meanings:

- Non-empty list defines who can publish.
- Non-empty list means the template requires explicit publish.

When `enableScorecardPublish` is enabled, `SubmitScorecard` auto-publishes normal scorecards for templates with no publisher roles.

## Current Visibility Behavior

### Scorecard API Read Visibility

`GetScorecard` fetches first, then calls `validateScorecardAccess`.

Rules:

- Server/super-admin bypass.
- Agent-only requesters can only access scorecards where `agent_user_id` matches their user ID.
- If `enableScorecardPublish` is enabled, agent-only requesters can see the scorecard only if it is published or if it is unmodified auto-scored (`ai_scored_at` set and `manually_scored` false/null).
- To avoid leaking existence, unpublished manual scorecards return NotFound to agents.

`ListScorecards` uses similar filtering for agent-only users:

- Restricts agent list to the authenticated user's ID unless QA-approved/server/super-admin.
- When `enableScorecardPublish` is enabled, agent-only visibility is `published_at IS NOT NULL OR (ai_scored_at IS NOT NULL AND NOT COALESCE(manually_scored, FALSE))`.
- When the flag is disabled, agent-only visibility is `submitted_at IS NOT NULL OR unmodified auto-scored`.

### Notification Visibility

`HasScorecardViewPermission` is currently described as notification permission. It defaults to ADMIN, QA_ADMIN, QA_SPECIALIST, MANAGER, MANAGER_2ND, AGENT, with optional override by `scorecard_viewers`. SUPER_ADMIN is always allowed.

Submit/publish notifications use `scorecard_viewers` and `scorecard_publisher_roles` in ways that do not exactly match API read visibility:

- Submit notification suppresses agent recipients when explicit publish is required.
- If publish is not required and publish flag is enabled, submit notification may be converted into a published notification.
- Publish notification includes the agent only if `HasScorecardViewPermission(agent, template)` is true.

### Score Detail / Comment Visibility

Score comments have per-score `comment_access_roles`. `removeCommentsForUnauthorizedUsers` removes comments from the response when the requester lacks those roles. This is separate from scorecard visibility and should remain a field-level visibility decision in the policy result.

## Analytics Context: Out of Scope

Analytics eligibility is intentionally out of scope for this scorecard permission policy project. The following findings are retained only to document related behavior that should not be included in the first policy API.

Manual QA stats and scorecard detail queries in `RetrieveManualQAStats` count scorecards with:

- `manually_scored = TRUE`
- matching customer/profile/template
- `calibrated_scorecard_id IS NULL`
- `scorecard_type IS NULL OR scorecard_type = 0`
- `submitted_at` within the requested range
- optional usecase/task/agent/submission-source/submitter filters

`RetrieveScorecardCriteriaStats` counts auto-score criteria only from submitted, non-calibration, original scorecards.

ClickHouse scorecard rows store `scorecard_submit_time`, `scorecard_publish_time`, `manually_scored`, and other scorecard state. Conversation ClickHouse indexing reads only original scorecards. Current analytics primarily use submit-time eligibility, not publish-time eligibility.

## Gaps / Risks

- "Edit", "grade", "submit", and "appeal" share helper functions but are semantically different capabilities.
- Template `scorecard_viewers` is notification-oriented in some paths but resembles read visibility in product semantics.
- Publish is currently both a capability and a visibility state transition.
- Agent visibility is feature-flag-sensitive and not expressed in template policy.
- Analytics eligibility is query-embedded and explicitly out of scope for this project.
- Field-level score/comment visibility is handled after data load, so callers must remember to scrub.
- Template-only permission evaluation is inherently partial because scorecard state affects every major output except static role lists.
