# PR #31048 open-comment evaluation

## Scope

Read-only evaluation of unresolved review comments on `cresta/go-servers#31048` at head `6ebc7a750f`. No product-code, PR, or comment state was changed.

## Open inline threads

GitHub GraphQL reports exactly two unresolved review threads, both current (not outdated):

1. `criterionRefToDisplayName` mutates a caller-supplied map. The reviewer's suggestion is reasonable: its only caller creates an empty map, so returning a newly built map would make ownership and the function name clearer. This is a readability refactor, not a correctness issue. Keep the requested-reference filter in `resolveCriterionDisplayNames`; the helper discovers every criterion in the fetched templates, while the public helper promises results for requested refs.
2. `convertCoachingPlanToPB` remains relevant. It is called by create/update coaching-plan paths and tests, and intentionally delegates with `nil` display names because the accepted design changes read resolution but not writes. Recommend replying with those call sites and retaining the compatibility wrapper, or renaming it if the distinction remains confusing.

## Automated comments

- Cursor's suggestion to remove `defer cleanup()` is incorrect for this suite. `TestSuite.TearDownTest` clears mocks only; it does not delete scorecard-template rows. `CreateModels` returns the deletion closure, so the defer prevents state leaking into later tests. Given/When/Then markers are optional consistency/style improvements.
- Older CodeRabbit summaries target superseded revision-aware-write commits and are not unresolved inline review threads on the current head.
- Fresh CodeRabbit CLI review found one separate major issue: malformed stored IDs (for example `template/`) now cause ref collection to return an error and fail the whole read, whereas conversion previously skipped malformed entries. The write converters can also serialize an empty criterion ID. Suggested follow-up: validate new inputs and skip/log malformed legacy stored entries during display-name enrichment so valid criteria still render.

## Recommendation

Address the map-return comment as an optional cleanup. The compatibility wrapper has live callers, so either explain and retain it or remove the indirection by making those callers pass the optional map explicitly. Independently fix the malformed-stored-ID compatibility risk before merge or explicitly establish that the database invariant rules such rows out.

## Local follow-up

At the user's request, implemented the two human-review cleanups locally in `/Users/xuanyu.wang/repos/go-servers-convi-7379` without committing or updating GitHub:

- Replaced the map-mutating `criterionRefToDisplayName` with `criterionRefsToDisplayNames`, which constructs and returns its map.
- Removed the compatibility `convertCoachingPlanToPBWithDisplayNames` wrapper arrangement. `convertCoachingPlanToPB` now accepts the optional display-name map directly, and all callers explicitly pass resolved names or `nil`.

Validation passed: `TestTransformers`, `TestBatchUpsertCoachingPlan`, `git diff --check`, and CodeRabbit review of the uncommitted diff (0 issues).

Committed the cleanup as `f3a4fc7396` (`[CONVI-7379] Simplify focus criterion display name conversion`) after `bazel run //:gazelle` completed successfully without generating additional changes.

Clarification: the malformed-ID issue was emitted by the local `coderabbit review --agent -t committed -c .coderabbit.yaml` run, not posted as a GitHub review comment, so it has no comment URL.

## Malformed-ID policy follow-up

The user selected strict exposure rather than fail-open skipping. The local uncommitted follow-up preserves `parseFocusCriteriaRefs` returning an error for any malformed stored ID and adds regression coverage proving a mixed list such as `template/valid`, `template/` returns the malformed-ID error instead of partial refs. Both write converters now reject an empty criterion ID with `InvalidArgument`, preventing new `template/` rows. `TestTransformers` passes and CodeRabbit raised 0 issues on the uncommitted diff.

Committed the strict malformed-ID handling as `9bf8c91b28` (`[CONVI-7379] Reject malformed focus criterion IDs`) and pushed it with `f3a4fc7396` to PR #31048. Replied to both human review threads with the implemented map-return and wrapper-removal changes; threads were left unresolved for reviewer confirmation.
