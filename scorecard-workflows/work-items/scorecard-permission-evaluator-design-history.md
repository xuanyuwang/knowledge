# Central permission evaluation starts with one implemented capability

**Primary domain:** scorecard-workflows
**Primary subdomain:** permissions-and-visibility
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-06-16
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

The design separates template form/scoring structure, template permission configuration, and runtime scorecard state. Permissions belong on ScorecardTemplate.permissions, not inside StructureV2. Analytics eligibility remains separately owned; notification implications are linked to Notifications.

The June 16 plan supersedes the README sketch BatchGetScorecardPermissions/ScorecardCapability with EvaluateScorecardsPermissions and one ScorecardPermission enum. It records a small evaluator in apiserver/internal/coaching/scorecards, reusing existing service models and user-filter membership expansion. The first supported permission is MODIFY_SUBMITTED_LOCKED_SCORECARD; unsupported enum values return denied decisions, so the general enum does not prove broad grade/view/appeal/publish support.

The first extraction preserves existing submitted-edit behavior and uses a thin service adapter, shared permission conversion in shared/scoring, and proto v2.3.18 in Go/Bazel. Empty/unset/whitespace editors and out-of-scope scorecard types use legacy edit permission. Configured users/teams/groups membership can deny a user who otherwise has an edit role. Later revision-selection and role-exemption changes must be read from their own dated evidence, not inferred from this initial design.

## Source context and limits

go-servers apiserver/internal/coaching/scorecards and shared/scoring; related cresta-proto and Director client work.

Inactivity evidence: Updated design-plan supersedes the draft RPC naming in README. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

Continue from the implemented permission-specific contract, not the broad original aspiration. Audit supported enum values and actual call sites before replacing a template-permission hook or adding another capability.

## Preserved evidence

The original folder is retained intact at [scorecard-permission-policy legacy evidence](../sessions/2026-10-25/scorecard-permission-policy-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/scorecard-permission-policy-legacy-evidence/README.md)
- [design-plan.md](../sessions/2026-10-25/scorecard-permission-policy-legacy-evidence/design-plan.md)
- [investigation.md](../sessions/2026-10-25/scorecard-permission-policy-legacy-evidence/investigation.md)
- [permission-history.md](../sessions/2026-10-25/scorecard-permission-policy-legacy-evidence/permission-history.md)
