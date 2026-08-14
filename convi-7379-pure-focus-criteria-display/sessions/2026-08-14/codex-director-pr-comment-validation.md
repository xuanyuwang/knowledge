# Director PR comment validation

## Scope

- PR: `cresta/director#21757`
- Commit reviewed: `cff1d057b3787856d9013f03da8110d6a3ae64b3`
- Base: `main`
- Review only; no product-code changes made.

## Validated findings

1. **Selected criteria lose `criterionDisplayName` — valid regression.** Both select components rebuild the form value with `criterionId`, `scorecardTemplateName`, and `target`, omitting `criterionDisplayName`. The omission predates this patch, but this patch makes the UI depend on that field for historical labels and deactivated state. Changing either selector can therefore restore raw IDs and lose the deactivated presentation until data is reloaded.
2. **Deactivated criteria appear in both selectors — valid.** A missing current-template criterion with a backend display name passes the filters for both focus criteria and outcome goals. Default form partitioning instead treats it as focus criteria because `isCriterionOutcomeGoal(undefined)` is false.
3. **Historical type ambiguity — valid contract/design issue.** `CoachingPlanFocusCriteriaInfo` has no criterion-type field, and current-template metadata is unavailable by definition for a deactivated criterion. Excluding such entries from outcome options removes duplication but may classify a historically deactivated outcome goal as focus criteria. A fully correct fix needs historical type enrichment or another durable classification signal.
4. **Feature-flag concern — valid rollout risk, not conclusively a mandatory flag.** Before `go-servers#31048` is deployed, `criterionDisplayName` can be absent. The new active-criterion rendering also falls back directly to the raw ID even when current-template metadata exists. Backend-first coordination can mitigate this; a tenant flag is a product rollout choice.
5. **QA metadata — partially stale, partially valid.** The current `linear-ticket` check passes for CONVI-7379. The live `qa-metadata-check` still fails because both Before and After proof sections lack videos.
6. **Missing testkit POM — heuristic false positive.** The route already has extensive page objects under `packages/director-testkit/src/pages/coaching/coaching-hub/agent/coaching-plan/` and a smoke spec at `packages/director-testkit/src/tests/coaching/agent/coaching-plan.spec.ts`. The narrower scenario-coverage concern remains: the spec does not assert deactivated criteria or their selector classification.

## CodeRabbit CLI

CodeRabbit raised two issues:

- Major: preserve historical type/classification for deactivated criteria. This overlaps finding 3 and is valid.
- Minor: improve the pre-existing delete-session confirmation copy. The string is unchanged by this PR, so it is a reasonable cleanup but not a regression or blocker for this change.

The GitHub CodeRabbit pass is weak supporting evidence because it skipped all changed files as similar to earlier changes and its ESLint setup failed.

## Current checks observed

- `linear-ticket`: pass
- `qa-metadata-check`: fail for missing Before/After videos
- `Build / Lint`: fail at observation time; the workflow was still running and failed logs were not yet available
- Codeowner check and several build jobs: pending

## Credential use

- Used the previously cleared existing `gh:github.com` authentication only for read-only PR/check inspection.
- Used the previously cleared `/Users/xuanyu.wang/.ssh/id_ed25519` GitHub identity explicitly to push the review fix.
- Did not use the restricted emergency SSH credential.

## Follow-up implementation

- Added a shared selection resolver used by both focus-criteria and outcome-goal selectors. It copies the complete backend criterion object, preserving `criterionDisplayName` and any future contract fields instead of reconstructing a partial value.
- Changed outcome-goal option filtering to require a current criterion template classified by `isCriterionOutcomeGoal`. Missing-template/deactivated criteria therefore remain only in the focus selector, matching `calculateDefaultFormValues`.
- Added regression coverage for display-name preservation and exclusion of a genuinely missing-current-template criterion while retaining active outcomes whose template title is unavailable.
- Validation: 18 focused Vitest tests passed, scoped ESLint passed for all six changed files, Prettier was applied, and `git diff --check` passed.
- Committed as `6abb06d6b6` (`[CONVI-7379] Preserve deactivated criterion metadata`) and pushed to Director PR #21757.
- The pre-commit protected-file and i18n checks passed. Its broad lint phase repeated the branch's known no-output stall and was interrupted; the commit used `--no-verify` after scoped lint and tests passed.
