# Omni-use-case template validity and validation placement

- Date: 2026-10-05
- Primary source repo: `/Users/xuanyu.wang/repos/director` (`main`, read-only)
- Related source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Design input: Xuanyu's agreement in the current engineering-plan discussion.

## Agreed invariant

A conversation template cannot apply to a use case if no criterion is applicable to that use case. Inapplicable criteria do not create score rows. Xuanyu prefers frontend validation first for authoring feedback.

## Source seams and engineering consequence

- Director `packages/director-app/src/features/admin/coaching/template-builder/TemplateBuilderForm.tsx` already calls `frontendFormValidation` in `handleSubmitForm`; it currently validates template items and shows an error toast. This is a save-time seam for the new per-use-case coverage check. A more immediate field-level warning is a UI design detail for later.
- Director `useSaveScorecardTemplate.ts` sends `template` and `usecaseNames` to Create or Commit.
- Go `apiserver/internal/coaching/action_create_scorecard_template.go` and `action_commit_scorecard_template.go` both call `newDBScorecardTemplateFromProto` in `transformers.go`. That converter parses the template and template use cases before a DB write. It is the shared backend seam for authoritative validation.
- Both validations must traverse nested and branch criteria and interpret empty/all and finite scopes consistently. Backend validation is required even when the UI checks first; otherwise API clients and scripts can persist a revision with an unusable use case.

## Next question

Xuanyu confirmed that any applicable criterion counts, including sentence/date/user criteria that are excluded from QA-score calculation. The invariant guarantees that an evaluation has a criterion, not that it can compute an aggregate QA score.

Next core question: keep the canonical `GetScorecardTemplate`/list representation as the full shared template revision, and apply use-case filtering in evaluation/consumer-specific read models, or filter the canonical template response itself? Current `GetScorecardTemplate` reads the full revision and `ScorecardTemplateProtoFromDB` converts its stored structure to the API object.
