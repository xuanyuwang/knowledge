# CONVI-7379: Recover focus-criterion labels on reads only

Date: 2026-08-13
Status: accepted
Ticket: [CONVI-7379](https://linear.app/cresta/issue/CONVI-7379/pure-coaching-plan-11-sessions-shows-raw-focus-criteria-ids-instead-of)
PR: [go-servers #31048](https://github.com/cresta/go-servers/pull/31048)

## Decision

Keep coaching-plan and coaching-session focus criteria revision-independent in storage:

```text
templateID/criterionID
```

Do not add a scorecard-template revision to the write format. On plan/session reads, batch-load the relevant scorecard-template revisions newest-first and populate `criterionDisplayName` from the newest revision containing `(templateID, criterionID)`.

The response continues to use a wildcard template revision (`T@*`). The revision used to recover a label is not asserted as the historical revision that originally supplied the criterion.

## Why the earlier design was changed

The first implementation wrote new criteria as `T@R/C` while retaining legacy `T/C` rows. Review and caller tracing exposed compatibility problems that were broader than the original display bug:

1. **Mixed-format filtering:** `ListCoachingPlans` wildcard requests naturally matched legacy `T/C` but not revision-qualified `T@R/C`. Supporting both required normalization or expanded SQL semantics.
2. **Target compatibility:** `director.targets` stores template ID and criterion ID but no template revision. Target-originated coaching-plan queries are therefore logically revision-independent and commonly use `T@*/C`.
3. **Inconsistent frontend inputs:** one target flow sent `T@*/C`, while another reconstructed the current template name and sent `T@R-current/C`, even though both represented the same revision-independent target.
4. **Overview identity loss:** coaching-overview code joins criteria to targets using revision-independent identity and returned `T@*`, conflicting with an explicit-revision storage contract.
5. **Legacy round trips:** inferring a concrete response revision for a legacy `T/C` row did not establish provenance. Sending that inferred `T@R/C` back as a strict filter would not necessarily match the original legacy row.

Resolving those issues would require a migration policy, dual-format parser, revision-aware filtering contract, overview propagation changes, and coordinated FE query behavior. That complexity is not necessary to fix CONVI-7379.

## Selected read contract

For a stored focus criterion `T/C`:

1. Query revisions of `T` newest-first.
2. Select the first revision containing criterion `C`.
3. Return that criterion's display name as `criterionDisplayName`.
4. Return the template resource name as `T@*`.

Apply enrichment to both coaching-plan and coaching-session reads used by the 1:1 session experience. The session editor derives options from the active plan returned by `ListCoachingPlans`, while session pills come from `ListCoachingSessions`.

## Assumptions and tradeoffs

- `(templateID, criterionID)` is treated as stable logical identity across revisions.
- If a criterion was renamed, the UI shows its latest known name, not necessarily its name when the plan was created.
- Criterion IDs must not be reused for different meanings.
- This is label recovery, not historical-revision reconstruction.
- Exact revision-dependent coaching criteria remain a possible future feature, but would require an explicit storage migration and API/filter design rather than being introduced as part of this bug fix.

## Consequences

- Existing rows and target joins remain compatible.
- `ListCoachingPlans` filtering keeps its existing revision-independent behavior.
- No dual-format persistence or backfill is required.
- FE plan/session surfaces can consume `criterionDisplayName` directly and stop reconstructing labels from current templates.
- The original raw-ID issue is fixed for criteria that still exist in any historical revision.

## Validation

- Backend tests cover renamed criteria, criteria removed from the newest revision, wildcard response revision, and revision-independent plan/session writes.
- `GetCoachingPlan`, `GetCoachingSession`, `ListCoachingPlans`, `ListCoachingSessions`, `CreateCoachingSession`, and `UpdateCoachingSession` suites passed.
- Frontend focus-criteria option/tooltip tests passed (17 tests), and changed-file ESLint passed.
