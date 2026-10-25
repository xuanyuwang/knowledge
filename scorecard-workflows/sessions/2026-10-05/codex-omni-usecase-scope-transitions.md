# Omni-use-case template and criterion scope transitions

- Date: 2026-10-05
- Source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Product input: Xuanyu's clarification in the current design discussion.
- Canonical working draft: `deliverables/omni-usecase-scorecard-eng-plan-draft.md`

## Confirmed product rule

Criterion applicability remains contained by template applicability. When a use case is added to a template, it is added to every criterion, including criteria with an explicitly defined subset. When a use case is removed from a template, it is removed from every criterion that included it.

For finite scopes, with old template set `T`, new template set `T'`, and old criterion set `C`, the transition is `C' = (C ∩ T') ∪ (T' − T)`. Thus template `{A, B}` and criterion `{A}` become criterion `{A, C}` when `C` is added to the template; removing `A` afterward produces `{C}`. This describes effective membership, not a chosen storage representation.

An exclusion set relative to the template could represent the same behavior for finite scopes, provided exclusions for removed template use cases are discarded. Included-ID arrays would require updating every criterion on each template membership change. Neither representation is selected yet.

## Next design question

Xuanyu confirmed that an already evaluated scorecard remains pinned to its template revision, remains available, and retains its scores when a later revision removes its use case. Removal affects future evaluations only.

Source verification: `director.scorecards` stores `template_revision`; `apiserver/internal/coaching/action_get_scorecard.go:GetScorecard` loads the template by that revision for criterion display names. In contrast, `action_evaluate_scorecards_permissions.go:getEvaluateScorecardsPermissionsInputs` loads `LatestRevisionName` for permission evaluation, and `action_evaluate_scorecards_permissions_test.go:TestEvaluateScorecardsPermissionsUsesLatestTemplateRevision` tests it. Xuanyu confirmed that template permissions from the latest revision remain in effect. This is a compatibility constraint; the next core question concerns the authoritative criterion data model.
