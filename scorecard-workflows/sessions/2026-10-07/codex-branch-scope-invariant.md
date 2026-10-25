# Branch-child criterion scope invariant

- Date: 2026-10-07
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Input: Continue the core cross-use-case template design after deferring materialized-view queryability.

## Source trace

`shared/scoring/scorecard_template_parser.go:parseTemplateCriterionBranches` parses branch children as criteria under a parent criterion. `shared/scoring/scorecard_scores_dao.go:GetValidScorecardScores` invokes `validateChildrenRecursively` when a scored parent has branches. That routine makes a child's score valid only if the parent score is valid and the branch condition matches. The current code has no criterion use-case scope, so it does not yet enforce a parent/child scope relationship.

## Model question

For template `{A,B}`, parent criterion `{A}`, and branch child `{B}`, the child would nominally apply to B but its parent could not be evaluated there. This is unreachable configuration. Proposed invariant: every branch child's use-case scope is a subset of its parent criterion's scope, in addition to the established template containment rule. A child with NONE is allowed. A parent with NONE can only have NONE children. A template use-case addition applied to every criterion preserves this invariant; explicit edits still need validation.

The per-use-case template coverage check should not count a branch child whose parent is out of scope. Xuanyu confirmed the subset rule on 2026-10-07 and classified it as an edge validation rule. It does not block the provisional core-model assessment.
