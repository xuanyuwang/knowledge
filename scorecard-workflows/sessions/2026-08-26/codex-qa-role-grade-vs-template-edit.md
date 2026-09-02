# Session: Can customers create a QA role that grades without editing templates?

Date: 2026-08-26
Repos: director-current (+ go-servers, cresta-proto, config)
Question: Can a customer create a dedicated role for QA users that can edit/grade evaluations without editing scorecard templates/configuration?

## Verdict

**Yes**, via builtin **QM Specialist (`QA_SPECIALIST`)** or a **custom role inheriting from `QA_SPECIALIST` (or Manager)** that keeps QA evaluation page features and omits Performance Templates. Do **not** use Performance Admin (`QA_ADMIN`) if template edit must be denied.

## Terminology

| Term | Meaning |
|---|---|
| Scorecard template / configuration | Template authoring under Admin → Performance Templates |
| Evaluation / grade | Create/update/submit a scorecard instance against a conversation/process |
| Submitted locked edit | Post-submit edit when `disableEditingOnSubmittedScorecards` is on; gated by `submitted_scorecard_editors` + grade permission |
| Acknowledged edit | Agent-ack'd scorecard; UI blocks non-`QA_ADMIN` |

## Evidence map

- Template permissions shape: `cresta-proto/.../scorecard_template.proto` `Permissions`
- Grade auth: `go-servers/.../scorecards/edit_permission.go` `HasScorecardEditPermission`
- Template RPC roles: `CreateScorecardTemplate` / `CommitScorecardTemplate` → ADMIN, QA_ADMIN, SUPER_ADMIN only
- Scorecard write RPC roles: Create/Update/Submit → includes QA_SPECIALIST, not only QA_ADMIN
- Feature defaults: `permission_feature_definitions.go` — Performance Templates = ADMIN+QA_ADMIN; Agent Evaluations includes QA_SPECIALIST
- Frontend: `directorAccess.ts` FA.ADMIN.PERFORMANCE_CONFIG vs FA.CONVERSATIONS.SCORECARDS_EDIT / FA.QA.*
- Custom roles double-write inherited proto role: `syncProtoRolesFromCustomRoles`
- QA_ADMIN hardcodes: acknowledged lock in `ScorecardForm.tsx`; appeal resolve defaults; template editors toggle is SUPER_ADMIN vs SUPER_ADMIN+QA_ADMIN

## Caveats

1. Custom roles do not invent a new AuthProto role for `scorecard_graders`; graders lists use proto roles only.
2. Scorecard write RPCs are role-annotated (not feature-only); inheritance must yield a grading proto role.
3. Post-submit and acknowledged edits are separate from normal grading.
4. Appeal resolve / calibration answer key remain QA_ADMIN/ADMIN hard defaults.
