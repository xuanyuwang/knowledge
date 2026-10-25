# Omni-use-case template write contract

- Date: 2026-10-05
- Source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Related source repos: `/Users/xuanyu.wang/repos/director` and `/Users/xuanyu.wang/repos/cresta-proto` (`main`, read-only)
- Input: Xuanyu's observation that the FE submits a complete template and proposal to support use-case-only writes with a FieldMask.

## Verified paths

- Director `useSaveScorecardTemplate.ts` builds a complete `templateData`, including structure and use-case names. `director-api`'s `commitScorecardTemplate` picks title, template, audience, use-case names, QA task config, permissions, and QA score config into `CommitScorecardTemplateRequest`.
- `cresta/v1/coaching/coaching_service.proto:CommitScorecardTemplateRequest` currently requires a `ScorecardTemplate` and has no update mask.
- Backend `CommitScorecardTemplate` loads the prior revision using `req.ScorecardTemplate.UsecaseNames` as a lookup filter, then creates a new template row and revision row in a transaction. A disjoint use-case replacement may fail during that lookup even before schema validation.
- `newDBScorecardTemplateFromProto` stores the incoming template JSON after parsing and validating it. It does not normalize criterion scope today.

## Design implications

A FieldMask for `usecase_names` could make a use-case-only request explicit. The backend would still need to load the previous full revision, apply the agreed criterion-scope transition, validate the resulting complete definition, and commit a new revision atomically. A FieldMask does not resolve omitted criterion scopes in the existing full-template Commit path.

The larger schema blocker is an empty criterion subset after a removal. For template `{A, B}` and criterion `{A}`, removing `A` yields criterion `[]`; the agreed empty-list sentinel means all, not none. This can accidentally widen criterion applicability. Xuanyu proposed JSON `null` for explicit NONE at criterion level, leaving `[]` as ALL and non-empty lists as INCLUDED. A V3 scope field should be required on every criterion; historical V1/V2 omissions inherit the stored revision's template scope through version-aware normalization.

A small `encoding/json` probe confirmed that `*[]string` decodes both `{}` and `{"usecaseIds":null}` as a nil pointer, while `{"usecaseIds":[]}` is a non-nil pointer to an empty slice. Re-encoding `{}` with a non-omitempty pointer field emits `null`. Thus the parser must check field presence before Go's ordinary decoding or use a presence-bearing custom type; a plain pointer cannot implement the V3 missing-field rule alone. Once V3 presence is validated, nil may represent explicit NONE in the in-memory scope model.

Xuanyu confirmed the cardinality and transition rule: a template has at least one use case, while a criterion may have zero. A NONE criterion remains in the revision and becomes applicable again when a new use case is added to the template, because the addition applies to every criterion. Example: template `{B}`, criterion `null`; add `C` -> template `{B,C}`, criterion `{C}`. A subsequent removal of `C` returns the criterion to `null`.
