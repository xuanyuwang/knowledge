# Latest template permission evaluation investigation

**Date:** 2026-07-22  
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`  
**Working context:** `/Users/xuanyu.wang/repos/go-servers-convi-7206` on `convi-7206-gate-submitted-scorecard-lock-behind-ff`  
**Ticket:** [CONVI-7350](https://linear.app/cresta/issue/CONVI-7350/refer-the-latest-revision-of-template-for-permission-evaluation)

## Reported behavior

For scorecard `customers/cresta/profiles/walter-dev/scorecards/019f8b88-ef5c-729b-89c8-c22440b77d48`, the authenticated requester was included in the template's submitted-scorecard editor configuration.

The frontend permission request returned:

```json
{
  "results": [
    {
      "scorecardName": "customers/cresta/profiles/walter-dev/scorecards/019f8b88-ef5c-729b-89c8-c22440b77d48",
      "permissions": [
        {
          "permission": "SCORECARD_PERMISSION_MODIFY_SUBMITTED_LOCKED_SCORECARD",
          "allowed": true
        }
      ]
    }
  ]
}
```

Director therefore did not block editing, but `UpdateScorecard` returned gRPC `PERMISSION_DENIED` / HTTP 403.

## Evidence chain

### Director

- `director/packages/director-app/src/components/scoring/scorecard-form/ScorecardForm.tsx`
  - Uses `useSubmittedScorecardEditPermission`.
  - Applies its `readOnly` result to the form.
- `director/packages/director-app/src/hooks/coaching/useSubmittedScorecardEditPermission.ts`
  - Enables evaluation when the feature flag is on and the scorecard is submitted.
  - Treats the explicit `allowed: true` response as editable.
- `director/packages/director-app/src/hooks/coaching/useEvaluateScorecardPermissions.ts`
  - Sends `currentUser.name` as `requesterUserName`.

This rules out a purely local frontend allowlist calculation: Director's editable state was backed by the backend permission RPC.

### Proactive backend path

- `apiserver/internal/coaching/action_evaluate_scorecards_permissions.go`
  - `getEvaluateScorecardsPermissionsInputs` groups scorecards by template ID.
  - It fetches templates through `getScorecardTemplateFromDB` with `LatestRevisionName`.
  - It evaluates permissions using `ScorecardPermissionEvaluator.EvaluateForTemplate`.

This path evaluates the latest template permissions.

### Write-enforcement path

- `apiserver/internal/coaching/action_update_scorecard.go`
  - `getExistingScorecardAndTemplate` loads the existing scorecard.
  - It loads the associated template with `existingScorecard.TemplateRevision`.
  - `checkPermissionOfUpdateScorecard` passes that pinned template into `hasSubmittedScorecardEditPermission`.
- `apiserver/internal/coaching/action_reset_scorecard.go`
  - Loads the template with `existingScorecard.TemplateRevision`.
  - Passes that pinned template into the same permission helper.
- `apiserver/internal/coaching/submitted_scorecard_permissions.go`
  - `hasSubmittedScorecardEditPermission` currently accepts a template from the caller.
  - It forwards that template to the evaluator.
- `apiserver/internal/coaching/scorecards/permission_evaluator.go`
  - Expands `submittedScorecardEditors`.
  - Performs exact requester membership using `slices.Contains(parsedFilter.UserNames, requester.GetName())`.

These paths evaluate permissions from the scorecard's historical pinned revision.

### Data-model evidence

- `apiserver/sql-schema/gen/model/scorecards.go`
  - Persists both `TemplateID` and `TemplateRevision`.
- `apiserver/sql-schema/gen/model/scorecard_templates.go`
  - Stores `Permissions` on each template revision.

Permissions can therefore differ by revision while a scorecard remains pinned to an earlier revision.

## Root cause

The system has two authorization revision policies:

```text
Director
  -> EvaluateScorecardsPermissions
  -> getScorecardTemplateFromDB(LatestRevisionName)
  -> latest submittedScorecardEditors
  -> allowed: true

UpdateScorecard / ResetScorecard
  -> existing scorecard.TemplateRevision
  -> dao.GetScorecardTemplates(pinned revision)
  -> historical submittedScorecardEditors
  -> denied
```

The identity and evaluator are not inherently different. The template object supplied to the evaluator is different.

## Proposed implementation

Update `hasSubmittedScorecardEditPermission` rather than creating another helper:

1. Remove the `scorecardTemplate *dbmodel.ScorecardTemplates` parameter.
2. Load the template inside the helper with:

   ```go
   s.getScorecardTemplateFromDB(
       ctx,
       s.appsDB.DB(ctx),
       scorecard.Customer,
       scorecard.Profile,
       scorecard.TemplateID,
       LatestRevisionName,
       nil,
   )
   ```

3. Pass the resulting latest template to `ScorecardPermissionEvaluator`.
4. Update `UpdateScorecard` and `ResetScorecard` to call the helper without a template.
5. Preserve their pinned-template loads where required for non-authorization behavior; remove them only if no longer otherwise needed.

## Design rationale

- The helper becomes the source of truth for both revision selection and submitted-scorecard authorization.
- Write handlers do not invoke another gRPC handler.
- The proactive RPC and enforcement use the same current-policy semantics.
- Historical scorecard content remains isolated from current authorization policy.

## Regression scenarios

- Requester denied in pinned `r1` and allowed in latest `r2`: preview, update, and reset all allow.
- Requester allowed in pinned `r1` and denied in latest `r2`: preview, update, and reset all deny.
- Same permission in both revisions: existing behavior remains unchanged.

## Separate follow-up

Director currently marks the form denied only when `allowed === false`. An errored permission query may leave `allowed` undefined and the form editable. This is a fail-open UI issue, but it did not cause the reported mismatch because the recorded RPC response was explicitly `allowed: true`.
