# Full-template Commit criterion scope precedence

- Date: 2026-10-07
- Source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Related source repos: `/Users/xuanyu.wang/repos/director` and `/Users/xuanyu.wang/repos/cresta-proto` (`main`, read-only)
- Input: Xuanyu confirmed that a criterion's final use-case scope may be explicitly narrowed after adding a template use case, as long as it remains a subset of the template's final scope.

## Decision

A full-template Commit carries the intended final criterion scopes. The frontend applies the automatic template-scope transition as an editing default, then allows explicit criterion edits before save. The backend validates the submitted complete definition, including containment and the already agreed requirement that each template use case have at least one applicable criterion. It does not overwrite an explicit criterion edit by blindly re-adding a newly added template use case.

If a separate `FieldMask` path updates only `usecase_names`, the backend starts from the previous full definition and applies the automatic add/remove transition itself before creating the next revision. That path remains a proposal.

## Next compatibility question

Historical stored criteria without a scope field inherit their own revision's template scope and are normalized on read. A new full-template Commit with a missing criterion scope is different: it may be an older client dropping an explicit subset. Decide whether to reject missing scope on new full definitions, and how to stage that rule while older clients remain deployed. The current Director builder submits a complete template, while other update paths also call Commit with a whole template.
