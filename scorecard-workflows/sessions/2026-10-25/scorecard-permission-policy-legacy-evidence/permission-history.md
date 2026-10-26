# Permission Evolution History

Created: Jun 9, 2026

## Scope

This document records how each scorecard permission evolved: why it was introduced, how behavior changed, and what that implies for the centralized scorecard permission policy design.

Sources used:

- Code history in `cresta-proto`, `go-servers`, and `director`.
- Glean search results from Coda, Google Docs, Linear, Slack, GitHub PRs, and help docs.
- Existing project investigation in `investigation.md`.

Analytics eligibility is intentionally out of scope for this project. Publish and view history mention analytics only when the historical feature discussion depended on agent-facing visibility, but the proposed permission policy should not expose analytics eligibility in its first API.

## Timeline

| Date | Permission / behavior | Change | Main reason |
|---|---|---|---|
| Jan 2024 | `template_editors`, `scorecard_viewers`, `scorecard_graders` | Added `ScorecardTemplate.Permissions` with three role lists. | Customers needed flexible control over who can see/use scorecards and a way to protect data integrity. |
| Dec 2024 | `scorecard_appealers` | Added appeal role list to template permissions and Director template builder. | Scorecard appeal workflow needed configurable roles for appeal requests. |
| Jan 2025 | Appeal request/resolve submit behavior | Backend implemented submit support for appeal scorecards. | Appeal became a scorecard workflow, not only a UI affordance. |
| Feb 2025 | Appeal default roles | Director changed empty appeal config to default to Manager and Manager 2nd instead of effectively allowing everyone. | Agents should not be able to appeal when permissions are not explicitly set up. |
| Mar-Apr 2025 | Shared edit permission helper | `ValidateRequesterPermission` was removed and create/update/submit moved toward shared `HasScorecardEditPermission`. | Reduce duplicated scorecard type permission logic. |
| Apr 2026 | Group calibration response special cases | Create requires task audience; update/submit require creator. | Managers can participate in group calibration, but only within assigned group calibration context. |
| Apr-May 2026 | View permission fixes | AI Coach template/criterion lists and notifications were aligned with `scorecard_viewers`. | Restricted scorecards were leaking into UI surfaces or notifications after viewer changes. |
| May 2026 | `scorecard_publisher_roles` | Added publish permission and `PublishScorecard` flow. | Submission was not enough as a final gate; reviewers needed control over when agents see scorecards. |
| May 2026 | `submitted_scorecard_editors` | Added user/team/group allowlist for editing submitted normal scorecards. | Submitted scorecards needed stronger edit locking without changing grader semantics. |

## `template_editors`

### Introduced

`template_editors` was introduced in `cresta-proto` PR `#3820`, commit `ee5c745058` on Jan 23, 2024, as part of the first `ScorecardTemplate.Permissions` message.

The original proto comment defined it as "roles that have admin permission (editing sections, etc.)" and documented two UI modes:

- "Cresta Admin only" maps to `[SUPER_ADMIN]`.
- "Cresta and customer user" maps to `[SUPER_ADMIN, QA_ADMIN]`.

### Why It Was Introduced

The original "Scorecard Template Permission" design doc says the feature came from customer feedback asking for more flexibility around who can see and use scorecards. The admin permission was scoped to Performance Config: if a user can see a template in that admin context, they can edit it; if not, the template effectively does not exist to them.

The design intentionally put permissions on the template because, in both admin and closed-conversation contexts, scorecards derive from templates.

### Behavior Evolution

The first implementation used role lists rather than a boolean even though the initial UI only had two template admin modes. PR discussion noted that a boolean could satisfy the immediate need, but a role list would be easier to extend and would use the same processing model as the view/grade permissions.

Current Director behavior still treats `template_editors` differently from the other scorecard permissions:

- SUPER_ADMIN / Cresta admin gets all permissions.
- Customer-side template edit access is effectively represented by whether `QA_ADMIN` is present.
- If customer template edit access is disabled, Director returns no scorecard-template permissions for non-Cresta users.

### Design Implication

`template_editors` is a template-admin permission, not a scorecard runtime capability. The scorecard policy layer should not overload it as `GRADE`, `SUBMIT`, `APPEAL`, or `PUBLISH`. It can remain a separate template capability if template-level permission evaluation is added later.

## `scorecard_viewers`

### Introduced

`scorecard_viewers` was introduced in the same Jan 2024 proto change, `ee5c745058`, with the proto comment "roles that have view permission (seeing it from filters, etc.)."

### Why It Was Introduced

The original design doc defined view in the Closed Conversation context: if a requester does not have view permission, the scorecard should be absent from filters, dropdowns, and similar surfaces. The doc also said scorecard and template can be used interchangeably in that context because each scorecard is derived from a template.

The user-facing help doc later described "Who can view this scorecard" as roles that can view scored results in Performance Insights and Closed Conversations. It also distinguishes view-only users from graders: they can see graded scorecards but should see a message that they cannot grade.

### Behavior Evolution

Current backend helper `HasScorecardViewPermission` describes this as notification permission: if a role can view a scorecard, it should have permission to be notified. Its default roles are:

- ADMIN
- QA_ADMIN
- QA_SPECIALIST
- MANAGER
- MANAGER_2ND
- AGENT

If `scorecard_viewers` is non-empty, it replaces the default role list. SUPER_ADMIN is always allowed.

Important behavior changes:

- Notification recipients now check the scorecard's own template revision. This matters because changing template permissions creates a new template revision; old scorecards may still point at an older revision.
- A 2026 bug investigation found the notification code was already correctly using `HasScorecardViewPermission` for new scorecards, but old scorecards could still notify agents because they were tied to the old template revision.
- An Apr 2026 Director fix aligned AI Coach criterion/template dropdowns with `canViewScorecard`, because supervisors without view access could still select restricted scorecards in AI Coach.
- Publish added another visibility dimension: when a template requires publish, agents should not see unpublished manual scorecards even if their role is in `scorecard_viewers`.

### Current Ambiguity

`scorecard_viewers` carries at least three meanings:

- Static template/view role: whether the template/scorecard should appear in role-filtered UI surfaces.
- Notification eligibility: whether a user should receive submit/publish notifications.
- Partial read expectation: product copy says viewers can see graded scorecards, but backend `GetScorecard`/`ListScorecards` also consider requester type, ownership, submitted/published state, and auto-score state.

### Design Implication

The policy layer should separate static role eligibility from runtime visibility:

- `CAN_VIEW_BY_TEMPLATE_ROLE`
- `READ_SCORECARD`
- `RECEIVE_SUBMIT_NOTIFICATION`
- `RECEIVE_PUBLISH_NOTIFICATION`

This avoids treating `scorecard_viewers` as a complete read policy when scorecard state and requester relationship also matter.

## `scorecard_graders`

### Introduced

`scorecard_graders` was introduced in the Jan 2024 proto change, `ee5c745058`, with the proto comment "roles that have grade permission (manually change the score)."

### Why It Was Introduced

The original design doc defined grade as a Closed Conversation permission: the ability to manually change the score of a scorecard for a closed conversation. It was part of the same customer-driven push to configure who can use a scorecard and protect scorecard data integrity.

The help doc later calls the UI label "Who can use this scorecard" and says these users can grade conversations in Closed Conversations and filter by the template in Performance Insights. It also notes that scorecard editors/graders are automatically added as viewers.

### Behavior Evolution

Current backend behavior is implemented through `HasScorecardEditPermission`, where normal/original scorecards use:

- Default roles: ADMIN, QA_ADMIN, QA_SPECIALIST, MANAGER, MANAGER_2ND.
- Template override: `scorecard_graders`, if non-empty.
- SUPER_ADMIN bypass.

The behavior has evolved in a few ways:

- Initially, different scorecard APIs had their own permission checks. In Mar-Apr 2025, permission checks moved toward shared helpers, including removal of `ValidateRequesterPermission`.
- `scorecard_graders` now effectively gates normal scorecard create/update/submit in multiple paths, not only "manually change score."
- Submitted normal scorecards may be further restricted by `submitted_scorecard_editors`, which can deny users who would otherwise be allowed by `scorecard_graders`.
- Group calibration response scorecards no longer rely only on grader roles; create/update/submit add task-audience and creator constraints.

### Design Implication

The policy layer should not expose a single overloaded "edit" decision. It should split at least:

- `GRADE`
- `SUBMIT`
- `UPDATE_DRAFT`
- `UPDATE_SUBMITTED`

The first version can preserve current behavior by making `SUBMIT` reuse edit/grade rules, but the output should make that coupling explicit.

## `scorecard_appealers`

### Introduced

`scorecard_appealers` was added to `ScorecardTemplate.Permissions` in `cresta-proto` PR `#5418`, commit `b25a3e0c4a`, on Dec 9, 2024. The proto comment defines it as "roles that have appeal permission (appeal the scorecard)."

Director support followed in PR `#9799`, commit `aa8fcc0007`, on Dec 11, 2024. The template builder added "Who can appeal this scorecard" for non-process templates and added `canAppealScorecard` to the frontend permission helper.

Backend submit support for appeal scorecards landed in `go-servers` PR `#18245`, commit `48809085d4`, on Jan 10, 2025.

### Why It Was Introduced

Glean surfaced the proto PR as part of `CONVI-3886 [Scorecard Appeal FE] support appeal scorecard creations/updates/submit`. The feature needed a configurable role list for who can start the appeal workflow.

Director also added a broader feature-access permission named `conversations/scorecards/edit/appeal`, enabled for ADMIN, MANAGER, MANAGER_2ND, and AGENT. That feature-access gate is separate from template-level `scorecard_appealers`.

### Behavior Evolution

Current backend behavior uses `scorecard_appealers` only for `SCORECARD_TYPE_APPEAL_REQUEST`:

- Default appeal-request roles: MANAGER, MANAGER_2ND, ADMIN.
- Template override: `scorecard_appealers`, if non-empty.
- SUPER_ADMIN bypass.

Director behavior changed in PR `#10628`, commit `fec9722b28`, on Feb 18, 2025:

- New/default appeal permissions became Manager and Manager 2nd.
- Empty appeal config no longer meant agents could appeal by default.
- Available appeal roles still include Agent in the UI, but Agent is not part of the default selection.

Appeal resolve is a separate scorecard type:

- Default resolve roles: QA_ADMIN, ADMIN.
- No template override today.

Publish explicitly excludes appeal scorecards. The publish design documents and Slack discussion decided publish should apply only to original scorecards because appeal request and appeal resolve scorecards should not be published independently.

### Design Implication

"Appeal" should not be one boolean internally. The centralized policy should represent at least:

- `CREATE_APPEAL_REQUEST`
- `RESOLVE_APPEAL`

The first is controlled by `scorecard_appealers`; the second is currently controlled by default QA/admin roles unless a future template field is added.

## `scorecard_publisher_roles`

### Introduced

`scorecard_publisher_roles` was added to `ScorecardTemplate.Permissions` in `cresta-proto` PR `#8485`, commit `cdf79d7b15`, on May 12, 2026. The proto comment defines it as "roles that have publish permission (publish the scorecard)."

Director added the require-publish toggle and publish button in May 2026:

- PR `#18689`: require publish toggle in the scorecard access step.
- PR `#18716`: publish button in the scorecard panel.

Backend publish support landed in `go-servers` around PR `#27852`, including `PublishScorecard`.

### Why It Was Introduced

The publish design doc states the goal directly: add a publish gate for conversation scorecards so QA analysts and managers can review and finalize scores before agents see evaluation results. Before this, submission was the final gate; once submitted, a scorecard became visible to roles with view permission, including agents.

The design also needed auditability: track who published and when.

Non-goals included:

- Process scorecards.
- Auto scorecards.
- Appeal scorecards.
- Bulk publish.
- Unpublish/revoke.
- Re-publish after appeal resolution.

### Behavior Evolution

`scorecard_publisher_roles` has two meanings:

- Non-empty roles define who can publish.
- Non-empty roles also mean the template requires explicit publish.

Current `PublishScorecard` validates:

- Scorecard exists.
- Scorecard is submitted and has a submitter.
- Scorecard is not already published.
- Scorecard is an original/normal scorecard.
- Template type is conversation.
- Requester has a role in `scorecard_publisher_roles`.

Submit behavior changed with publish:

- If publish is enabled and the template does not require publish, submit can auto-publish normal scorecards.
- If the template requires publish, submit suppresses agent notification until publish.
- Published scorecards remain published even if template publisher roles later change.

Visibility behavior changed with publish:

- Agents should see only published scorecards from templates requiring publish.
- Templates that do not require publish behave as before, with submitted/manual or unmodified auto-score visibility depending on the surface.
- Auto-generated scorecards were intentionally left visible to agents in performance contexts as a core value prop.

### Design Implication

Publish is both a capability and a state transition that affects visibility. The policy layer should expose:

- `PUBLISH` capability with validation checks for not submitted, already published, not original, not conversation template, or missing publisher role.
- `REQUIRES_PUBLISH` / `VISIBLE_TO_AGENT` style visibility facts derived from template + scorecard state.

It should avoid making callers infer "requires publish" from raw role-list length.

## `submitted_scorecard_editors`

### Introduced

`submitted_scorecard_editors` was added in May 2026 for `CONVI-6862`.

The proto history shows an initial add on May 20, 2026, followed by PR `#8631`. The field evolved during review:

- It started as a role-like permission.
- It moved to a `UserTeamGroup` structure.
- Field number 6 was reserved and `submitted_scorecard_editors` became field 7.

Backend enforcement landed in `go-servers` PR `#28171`, commit `e170df990f`, on May 29, 2026.

Director added submitted editor controls in late May 2026, including a users/teams/groups picker with copy like "Who can edit this scorecard after submission."

### Why It Was Introduced

The "Disable Scorecard Editing" engineering design doc says submitted scorecards were not immutable and there was no dedicated template permission for post-submit editing. The desired v1 behavior was a submitted-scorecard lock.

The design explicitly avoided reusing `scorecard_graders` because changing `scorecard_graders` semantics would silently affect existing meaning. A separate permission answers a narrower question: even if the user can normally edit/grade, is this already-submitted scorecard editable for this operation?

### Behavior Evolution

Current backend behavior:

- Applies only to submitted normal/original scorecards.
- Falls back to existing edit permission when:
  - scorecard is unsubmitted,
  - scorecard type is out of scope,
  - the config is unset,
  - the config is empty or whitespace only.
- When configured, expands users, teams, and groups through the shared user-filter parser.
- A configured allowlist denies even users who would otherwise pass role-based edit checks. Tests explicitly cover denying admin bypass when submitted editors are configured.
- Enforcement applies to `UpdateScorecard` and `ResetScorecard`.
- First submit remains unchanged.
- Appeals and group calibration are out of scope.

### Design Implication

This is the strongest evidence that role-based template permissions are no longer enough. The policy layer input must support:

- requester user identity,
- user/team/group membership,
- resource ACL expansion,
- scorecard submitted state,
- scorecard type.

The implementation should keep these branches clear internally: "allowed by role fallback", "allowed by submitted editor allowlist", and "denied because submitted editor allowlist is configured and requester is not included." The first public policy result can still remain a boolean.

## Implicit And Special-Case Permissions

These are not fields in `ScorecardTemplate.Permissions`, but they are part of the scorecard permission domain and should be represented in the centralized policy.

### Group Calibration Response

Group calibration response permissions changed in `go-servers` commit `c1c2c709` on Apr 29, 2026.

The change added:

- Create: requester must be in the task audience.
- Update: only the creator can update.
- Submit: only the creator can submit.

The Linear context says managers can be participants in group calibration but could not submit scorecards due to permission checks. Marriott preferred allowing managers to submit in group calibration, with group calibration as the priority over 1:1 calibration.

Design implication: group calibration decisions require task audience and creator state, not only template roles.

### Agent Read Visibility

Agent read visibility is controlled by requester relationship and scorecard state:

- Agent-only users can only access their own scorecards.
- With publish enabled, unpublished manual scorecards from templates requiring publish are hidden from agents.
- Unmodified auto-scored scorecards can remain visible.
- To avoid leaking existence, some denied agent reads are returned as NotFound.

Design implication: visibility decisions need enough internal structure to map a denied agent read to either "deny" or "not found compatible" at API boundaries, even if the public permission API only returns booleans.

### Score Comment Visibility

Score comments have `comment_access_roles` and are scrubbed after load by `removeCommentsForUnauthorizedUsers`.

Design implication: score/comment visibility is field-level and may be partial. The first policy API can expose aggregate read-comment visibility, but a complete model eventually needs per-score or per-comment decisions.

## Summary For The New Policy Layer

The permission model evolved from static role lists into a mixed model:

- Template role lists: template editors, viewers, graders, appealers, publishers.
- User/team/group allowlists: submitted scorecard editors.
- Scorecard state gates: submitted, published, original vs appeal vs calibration, auto/manual scoring.
- Relationship gates: requester is agent, creator, submitter, manager, task audience member.
- Surface-specific visibility: API read, filters/dropdowns, notifications, field-level comments.

The centralized policy layer should therefore be a scorecard-state-aware evaluator, not a thin wrapper around `ScorecardTemplate.Permissions`. The first API should answer capabilities and visibilities only; analytics eligibility remains out of scope.
