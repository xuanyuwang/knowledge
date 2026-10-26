# Submitted editor UpdateScorecard 403 investigation

**Date:** 2026-07-22
**Source repos:** `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/director`
**Scorecard:** `customers/cresta/profiles/walter-dev/scorecards/019f8b88-ef5c-729b-89c8-c22440b77d48`
**Requester:** `customers/cresta/users/d634c90a8748bcb2`

## Symptom

The Director form remains editable with `disableEditingOnSubmittedScorecards` enabled, but `UpdateScorecard` returns `PERMISSION_DENIED` although the latest template permissions list the requester under `submittedScorecardEditors.users`.

## Findings

1. Backend 403 means submitted-editor evaluation completed with `allowed=false`. Parser/config failures are wrapped as internal errors rather than permission denied.
2. The observed `EvaluateScorecardsPermissions` response returned `allowed=true` for `customers/cresta/users/d634c90a8748bcb2`. This proves the flag-enabled latest-revision path successfully parsed the permissions, expanded the configured users, and matched the requester.
3. Proactive evaluation and update enforcement load different template revisions:
   - `EvaluateScorecardsPermissions` deliberately calls `getScorecardTemplateFromDB(..., LatestRevisionName, ...)`.
   - `UpdateScorecard` calls `dao.GetScorecardTemplates` with the scorecard's persisted `TemplateRevision`.
   - `ResetScorecard` also uses the scorecard's persisted `TemplateRevision`.
4. Given the same authenticated user, the observed true proactive decision followed by an update 403 is explained by the latest-versus-pinned permission input. The feature flag itself is not the cause.
5. Director has a separate fail-open UI edge: `useSubmittedScorecardEditPermission` treats only `allowed === false` as denied. It did not cause this occurrence because the query returned a successful true decision, but it can mask future query errors.
6. The backend membership decision is exact equality between `requester.Name` and a name in `parsedFilter.UserNames`.

## Verification

Compare:

- the scorecard row's `template_id` and `template_revision`;
- `scorecard_templates.permissions` for that pinned revision;
- `scorecard_templates.permissions` for the latest revision;
- the browser network response from `EvaluateScorecardsPermissions`.

The evaluation response is `allowed=true` while update returns 403. Compare the pinned and latest permission JSON to capture the final row-level proof and identify which historical allowlist denied the update.

## Recommended fix

Use the latest template revision consistently for submitted-editor authorization in update and reset paths, while continuing to use the scorecard's pinned revision for score calculation/template structure. Also consider failing the Director form closed when proactive permission evaluation errors.
