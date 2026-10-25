# Template JSON version and use-case scope semantics

> Superseded application: the version field is relevant to a **criterion-level** none/all/some scope in JSON, not to changing the semantics of the template-level SQL `usecase_ids` column. See [the corrected scope model](codex-criterion-three-state-versioned-json.md). The source facts and sample version counts below remain valid.

- Date: 2026-10-05
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Related source repos: `/Users/xuanyu.wang/repos/director` (`main`, read-only)
- Question: Can template schema version and a backend updater chain distinguish historical NULL/all from new NULL/none?
- Previous context: `sessions/2026-10-05/codex-template-three-state-compatibility.md` and `codex-template-null-history-tests.md`.

## Verified current state

- The JSON **already has** a `version` field. `shared/scoring/scorecard_template_parser.go:templateStructure.Version` is `*int`; missing version is treated as V1. `ParseScorecardTemplateStructure` supports V1 and V2 and rejects other values. `ScorecardTemplateStructureV2` contains `Version int` and `Items`. The V1/V2 distinction is primarily a JSON shape distinction (`criteria` versus `items`). The current parser dispatches directly; it does not run sequential V1→V2→V3 updaters.
- Director's builder converts a V1 `criteria` template to `items` and `version: 2` on editing; new templates are V2. The backend has many direct `ParseScorecardTemplateStructure(template.Template)` callers, including create/update/submit/export/scoring paths, not one DB-row normalization boundary.
- `director.scorecard_templates.usecase_ids` and `director.scorecard_template_revisions.usecase_ids` are **separate SQL columns**, outside the versioned JSON. `ScorecardTemplateProtoFromDB` converts nil and empty arrays to the same zero-length repeated `usecase_names`. JSON version alone does not allow a V3 API client to tell V3 all (`{}`) from V3 none (NULL).
- A read-only count in the user-specified `voice-sandbox-2` database grouped by `template->>'version'` found all 1,513 current rows and all 1,513 revision rows at JSON version `2`, with zero missing/version-1/other rows and zero NULL/empty use-case arrays. This is a one-database observation, not a global distribution.
- Existing SQL template filters treat `usecase_ids IS NULL OR usecase_ids = '{}'` as all, and some paths inspect arrays in Go. If old and new persisted versions coexist, direct SQL must either inspect `template->>'version'` in every relevant predicate or query a normalized projection/view. A Go-only updater cannot change rows filtered out by SQL before they reach Go.

## What a version-aware updater would require

Suppose stored V1/V2 NULL means all and stored V3 NULL means none, while `{}` means all. The normalizer must accept the **whole stored row** or at least `(original JSON version, template JSON, usecase_ids)`. It should first interpret scope using the original version, then run deterministic JSON updaters, returning a latest-version in-memory template with explicit logical scope. Merely upgrading `template.version` to 3 while retaining a NULL `usecase_ids` would silently flip historical all to none.

On a new Commit, persist the latest JSON version and canonical array value together in both template tables; historical revisions can remain physically unchanged if read paths normalize them. Old binaries reject V3 JSON, so V3 writes still need the same reader-first deployment gate. Direct SQL requires an explicit old/new branch, e.g. old-version NULL is included as all but V3 NULL is excluded as none. Missing JSON version must be treated as V1 in that predicate. A version-aware view could centralize the condition, but callers must use it consistently.

An explicit API scope mode remains necessary: within V3, both NULL and `{}` become empty `usecase_names` today. A legacy client can also drop an unknown mode on full-template Commit, so safe edit behavior is still required.

## Assessment

The updater chain is useful for future **JSON structure** changes, including criterion applicability fields and historical V1/V2 normalization. It is a costly discriminator for the semantics of an external SQL column. Under Xuanyu's working assumption that existing template rows contain no NULL, a global NULL=none reader-first rollout is simpler and avoids permanent version-aware SQL. Use JSON-version-based array semantics only if a real legacy NULL population must be preserved without rewriting rows. This is a design assessment, not a selected implementation.

The older `knowledge/template-schema-version-updater/deliverables/eng-plan.md` is a draft that proposes adding a version field; current source already has the field and V1/V2 parser dispatch. Any future updater plan should start from this verified state.
