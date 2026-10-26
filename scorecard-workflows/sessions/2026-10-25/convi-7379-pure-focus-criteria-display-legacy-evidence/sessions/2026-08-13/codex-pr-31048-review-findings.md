# PR 31048 review findings

Date: 2026-08-13
Source repo: `/Users/xuanyu.wang/repos/go-servers`
Branch/worktree: `convi-7379-focus-criteria-display-names-be` in `/Users/xuanyu.wang/repos/go-servers-convi-7379`
PR: https://github.com/cresta/go-servers/pull/31048
Reviews:

- https://github.com/cresta/go-servers/pull/31048#pullrequestreview-4928927631
- https://github.com/cresta/go-servers/pull/31048#pullrequestreview-4928931548

## Final decision from this review

The revision-qualified write design described in the findings below was not retained. Mixed stored formats created backward-compatibility problems in ListCoachingPlans filtering, response/filter round trips, and coaching overviews. More importantly, `director.targets` has no template revision, so target identity and target-originated coaching-plan queries are inherently revision-independent.

The accepted solution keeps `templateID/criterionID` writes and existing target/filter semantics unchanged. Historical template revisions are used only on reads to recover `criterionDisplayName`, and responses retain the wildcard template revision. The earlier findings remain below as evidence explaining why the design changed. See `decisions/2026-08-13-read-path-only-focus-criterion-label-recovery.md`.

## Findings verified at commit bd1b6d6e72

1. **Wildcard focus filter misses revision-qualified stored IDs — valid, high priority.**
   `buildCoachingPlanConditions` translates `template@*/criterion` to only the legacy `template/criterion` database value. New writes with an explicit revision persist `template@revision/criterion`, so the PostgreSQL array-overlap predicate does not match them. The current tests exercise wildcard requests only against legacy rows and therefore miss the regression.

2. **Display-name lookup failures fail reads — a valid reliability tradeoff, not an unambiguous bug.**
   The resolver previously swallowed template lookup errors and returned no enrichment, but the 2026-08-12 implementation history records an explicit earlier PR-review change to annotate and propagate those errors. The frontend was also simplified on the assumption that the backend always supplies `criterionDisplayName`; failing open could reproduce the original raw-ID behavior. Keeping the lookup fail-closed therefore matches the recorded contract unless the team deliberately reprioritizes endpoint availability over complete criterion metadata.

3. **GetCoachingSession resolver error lacks context — valid minor finding.**
   The handler currently returns the resolver error directly. Under the recorded fail-closed behavior, wrap it with the coaching-session ID and operation while preserving the original error.

4. **Coaching overview drops explicit revision identity — valid, high priority.**
   `retrieveCriteriaInfo` parses an explicit revision but always emits `@*`. The no-target branch constructs a wildcard name, while the target branch returns a prebuilt wildcard `FocusCriteriaInfo`. The target lookup itself should remain revision-independent because targets do not store revision, but each returned item must be newly constructed with `ref.revisionID` and a copied target.

5. **Use nil-safe protobuf getters in focus-filter construction — valid minor hardening.**
   A nil element in the repeated message slice panics on direct field access. Nil-safe getters turn it into an ordinary invalid/empty request value that can be rejected without crashing the handler. Apply getters consistently to parsing, error rendering, and both identifier forms.

## Example scenarios

- Stored `security@rev-17/greeting`, requested filter `security@*/greeting`: current overlap candidates contain only `security/greeting`, so the plan is omitted.
- A plan is loaded, then the scorecard-template revision query times out: current code fails the Get/List operation to preserve the backend-enrichment contract. Failing open would improve availability but could make the frontend render the raw criterion ID again.
- Stored `security@rev-17/greeting` in an overview: both target and no-target branches currently return a scorecard template name ending in `security@*`, losing the historical identity needed to resolve the old criterion.
- `FocusCriteria: []*FocusCriteriaInfo{nil}`: direct `.ScorecardTemplateName` access panics; `GetScorecardTemplateName()` returns an empty string and allows normal validation.

## Recommended order

1. Fix and test wildcard matching for both legacy and explicit-revision rows.
2. Preserve explicit revision IDs in overview responses; test target and no-target branches.
3. Retain the recorded fail-closed enrichment semantics unless the availability tradeoff is deliberately reopened; wrap errors with resource context.
4. Apply protobuf getters as a small hardening cleanup.

## Director dependency on ListCoachingPlans criterion display names

Checked Director commit `d2ff182d6b` in `/Users/xuanyu.wang/repos/director-convi-7280`.

- The two focus-filtered Coaching Hub callers do not read `criterionDisplayName` from returned plans. They use template ID, criterion ID, agent identity, and target data to count agents on an org target and initialize the bulk-add table.
- `ListCoachingPlans` is also the source for the active plan on the agent coaching-plan page (`useLoadCoachingPlanDetails` calls List with agent + active + page size 1 when no historical plan ID is selected).
- That active plan flows into `FocusCriteriaSelector`, which requires `focusCriteria[].criterionDisplayName` to retain/render normal criteria and especially criteria absent from current templates. Session-card option/tooltip helpers also use backend-populated names; session rows themselves come from `ListCoachingSessions`, but the plan-level selector depends on the ListCoachingPlans response.

Conclusion: removing display-name resolution from every ListCoachingPlans response would break the paired frontend and can reproduce the raw/missing deactivated-criterion behavior. A safe optimization would be explicit per-request projection, defaulting to enriched responses for compatibility (for example a BASIC/FULL view or `exclude_criterion_display_names` flag). The known focus-filtered Coaching Hub calls could request the basic form.

## Wildcard filter SQL design

Finding 1 can be fixed in the ListCoachingPlans predicate without changing storage. Partition request criteria into concrete/exact IDs and wildcard logical IDs. Preserve array-overlap matching for legacy and concrete-revision exact values. For wildcard logical IDs, unnest `focus_criteria_ids` and compare a normalized stored value where `template@revision/criterion` becomes `template/criterion`:

```sql
EXISTS (
  SELECT 1
  FROM unnest(focus_criteria_ids) AS stored(id)
  WHERE regexp_replace(stored.id, '@[^/]+/', '/') = ANY (?::varchar[])
)
```

Combine the exact-overlap and wildcard `EXISTS` branches with OR to retain “match any requested criterion” semantics. The fixed regex contains no request data; request IDs remain bound values. Current schema inspection found no GIN index on `focus_criteria_ids`, so this does not give up an existing array-index advantage. Tests should cover wildcard versus legacy/any revision, concrete revision versus legacy/same/different revision, and multiple-filter OR behavior.

## Production ListCoachingPlans focus-filter callers

At Director commit `d2ff182d6b`, exactly two production callers populate `ListCoachingPlans.focusCriteria`:

1. `CriteriaGoalSnapshot` maps `ListTargets` results directly into the filter. `convertDBTargetToPB` emits `ScorecardTemplateName` with revision `*` because `director.targets` stores no revision. This produces wildcard `template@*` requests, one per org target in a combined OR query.
2. `AddGoalPopover` calls `useUsersAndCoachingPlansWithFocusCriterion` with `template.name`, where `template` is the current scorecard template resolved for the target. This produces a concrete current-revision request such as `template@rev-current`.

No production go-servers internal ListCoachingPlans caller supplies the focus filter; the opportunity and export callers filter by agents/plans instead. Existing ListCoachingPlans focus-filter tests cover wildcard names only.

Both Director calls conceptually operate on a revisionless target. The concrete revision in the popover comes from using the current template object for the lookup rather than forwarding the target's wildcard template name. That can omit an agent whose plan stores the same target criterion under an older explicit revision. Consider changing the popover lookup to use `target.scorecardTemplateName`/`@*`, while still preserving concrete-revision API behavior for other clients.

## Proposed filter contract

A coherent future contract is origin-sensitive at the FE and strict at the BE:

- Target-originated filter: send `T@*/C`; match legacy `T/C` and all `T@revision/C` rows.
- Plan-focus-criterion-originated filter: send `T@R/C`; match only `T@R/C`, not other revisions.

This cleanly separates revisionless target identity from revision-specific plan identity. One migration caveat remains: legacy stored `T/C` rows have unknowable original revision. The current response converter resolves such rows to a concrete revision name for display. Reusing that response as a strict concrete filter would not match the source legacy row. Options are to accept that legacy rows match wildcard queries only, migrate/backfill them, preserve transitional concrete-to-legacy matching (with false-positive risk), or expose legacy provenance separately. For semantically strict behavior, legacy-only-on-wildcard plus an explicit migration policy is the cleanest contract.

## Can the ticket be scoped to coaching sessions only?

The literal collapsed session pill can be fixed by enriching `ListCoachingSessions`, but the complete 1:1 session UX is not session-only under the paired FE design:

- `useLoadCoachingPlanDetails` calls `ListCoachingPlans` to obtain the active plan and separately calls `ListCoachingSessions`.
- `CoachingSessionCard.useSessionCriteriaOptions` builds the editor's focus/outcome options from `coachingPlanDetails.coachingPlan.focusCriteria`, not from the current session.
- At FE commit `d2ff182d6b`, deactivated plan criteria are included/labeled only when the plan criterion has BE-populated `criterionDisplayName`.
- New sessions copy the plan's focus criteria, and create/update/acknowledge responses do not all currently resolve names.

Therefore, simply removing ListCoachingPlans enrichment would fix some read-only session rendering but can reintroduce raw/missing values in the expanded editor, omit deactivated plan criteria, and leave post-write responses inconsistent. Avoiding global List enrichment is possible only with a deliberate alternative: merge enriched session criteria into editor options and enrich/refetch every session write response, or use List only to discover the active plan and follow with enriched GetCoachingPlan. The lower-risk current architecture is to retain List enrichment for plan-detail callers and add an explicit lightweight projection for filter-only List calls.

## Minimal read-only alternative

CONVI-7379 can be solved without changing the persistence format if exact historical revision identity is not a requirement:

- Continue storing `templateID/criterionID` for plan and session focus criteria.
- At read time, batch-load template revisions newest-first and use the first/latest revision containing that criterion ID to populate only `criterionDisplayName`.
- Keep the response template name revision wildcard (`template@*`); the revision used to find a label is not proven to be the revision originally associated with the plan/session and should not be presented as historical identity.
- Enrich both session reads and the plan reads used by the session editor, because the editor builds its options from plan focus criteria. Apply the behavior consistently to mutation responses or refetch after mutations.

This directly resolves the Pure examples because the removed criterion IDs still exist in historical Security template revisions. It eliminates the new storage format, dual-format parser, revision-aware List filter, overview revision propagation, and legacy round-trip issues. The tradeoff is semantic: if a stable criterion ID was renamed, the UI shows its latest known name; if IDs can be reused for different meanings, this approach is unsafe. The design should explicitly treat `(templateID, criterionID)` as stable logical identity and describe the lookup as label recovery, not historical reconstruction.

## Corresponding frontend simplification

The frontend should stop reconstructing criterion labels from the currently loaded scorecard templates wherever it is rendering a coaching plan/session focus-criterion response:

- `FocusCriteriaSelector` can remove its `useGetCriteriaIdToDisplayName(templates)` dependency, retain criteria based on `criterionDisplayName`, and pass the response field directly to the card.
- Session option and tooltip builders should use `focusCriteria[].criterionDisplayName` (falling back to the raw ID defensively) rather than recovering a name from `criterionById` or reverse-searching/splitting select-option labels.
- `CoachingPlanHistoryModal`, which also consumes `ListCoachingPlans`, should prefer the response `criterionDisplayName`; the paired FE commit did not update this remaining List consumer.

Do not delete the shared display-name hook or every lookup map globally: charts and other APIs that return only criterion IDs still need current-template name resolution. The session editor also still needs `criterionById` for current metadata such as outcome-goal/ALO classification and effectiveness behavior, and it needs `templateDisplayNameMap` for the template portion of labels. The removable map is specifically the current-template `criterion ID -> display name` map used as a substitute for response metadata.

## Implementation update

Created dedicated worktrees:

- BE: `/Users/xuanyu.wang/repos/go-servers-convi-7379`, existing branch `convi-7379-focus-criteria-display-names-be` moved out of the main checkout.
- FE: `/Users/xuanyu.wang/repos/director-convi-7379`, new branch `xw/convi-7379-focus-criteria-display-names-fe` anchored at FE commit `d2ff182d6b`.

The BE working tree now implements the minimal model: both plan and session writers persist `templateID/criterionID`; label resolution performs one newest-first query over relevant template IDs; the first revision containing a criterion supplies its display name; returned template names remain `@*`; and the revision-aware List filter changes were removed. Added coverage for renamed criteria, criteria removed from the newest revision, wildcard response revision, and revision-independent plan/session writes. Focused and endpoint-level coaching tests passed.

The FE branch consumes `criterionDisplayName` in plan/session selectors, pills, tooltips, and options while retaining current-template metadata maps for classification. `CoachingPlanHistoryModal` was additionally changed to prefer the ListCoachingPlans response name and no longer builds a current-template criterion-name lookup. Focused unit tests and ESLint passed.

The BE branch history was subsequently replaced with one commit, `6ebc7a750f` (`[CONVI-7379] Resolve focus criterion display names on read`), based on the original PR merge base. The final commit changes only `GetCoachingPlan`, `GetCoachingSession`, `ListCoachingSessions`, transformer read enrichment, and resolver tests. Write functions, mutation handlers, target/overview behavior, and List filter SQL match the base branch. The replacement was force-pushed with a lease against remote tip `cc2433342f`.
