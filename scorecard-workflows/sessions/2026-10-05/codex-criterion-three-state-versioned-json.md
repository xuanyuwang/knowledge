# Criterion three-state scope in versioned template JSON

- Date: 2026-10-05
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Related source repo: `/Users/xuanyu.wang/repos/director` (`main`, read-only)
- Product correction: Xuanyu clarified that a template always applies to at least one use case. Template scope needs only all versus a finite non-empty set; zero-applicability is needed for a **criterion** after a template-scope removal.

## Corrected scope boundary

Keep the current template-column contract: `usecase_ids = '{}'` represents all use cases, and a non-empty array represents a finite set. Do not introduce a template-level NONE state or reinterpret SQL NULL for this feature. `NULL` remains a legacy compatibility value to assess separately; it is not the new criterion sentinel.

Versioned template JSON can add an explicit criterion NONE state alongside ALL and INCLUDED/SOME. Example: template `{A,B}`, criterion `{A}`; remove `A` → new template `{B}`, criterion NONE. The criterion remains in the new revision but is inapplicable to B; historical scorecards keep their pinned old revision. A later template addition follows the already agreed transition rule: newly added use case membership is added to every criterion, so a NONE criterion could become `{C}` when C is added.

The exact V3 JSON representation remains to be chosen. One compact encoding is `null` = none, `[]` = all, and non-empty IDs = included subset on a criterion field. A tagged scope object is another choice. If JSON `null` is chosen, the V3 parser must distinguish **missing field** from explicit `null`; a plain Go `*[]string` does not. V3 writes should require an explicit valid scope or a deliberately defined default, and reject omissions that would silently widen a previously narrowed criterion. The existing containment and per-use-case coverage validations still apply. Chapters have no independent scope; criteria nested under branches do.

## Version and updater boundary

`shared/scoring/scorecard_template_parser.go` currently treats missing JSON `version` as V1, supports V1 and V2, and rejects V3. `ScorecardTemplateStructureV2` has `Version int`. Director converts V1 `criteria` to V2 `items` when editing; there is no sequential backend updater chain today.

A backend V2→V3 normalizer can upgrade historical criteria that lack a scope field. Under Xuanyu's prior compatibility choice, each missing historical criterion scope inherits **that stored revision's** template scope. Therefore the normalizer needs both the JSON and the same revision row's `usecase_ids`; JSON alone cannot materialize a finite included list. It should return a latest-schema in-memory model without changing historical rows, while new Create/Commit writes persist V3 JSON with explicit criterion scope. The same normalizer should be used by evaluation, scoring, Get/list conversion, and other backend JSON readers. A frontend editor must also understand the V3 contract before V3 writes begin.

The version field is inside template JSON; template `usecase_ids` remains outside and retains one global meaning. This avoids the version-conditioned SQL template filters explored in the superseded template-level three-state proposal. SQL that needs to query **criterion** applicability directly must still handle mixed persisted V1/V2/V3 JSON or consume a normalized, revision-keyed projection. Application code can normalize after loading a row, but SQL filtering before loading cannot call that Go updater.

## Open implementation choices

- Choose the V3 criterion scope encoding and explicit missing-field policy.
- Preserve the earlier ALL meaning: an empty criterion list means all profile use cases. A criterion applying throughout a finite template therefore repeats that template's explicit IDs to satisfy containment.
- Decide whether a zero-applicability criterion may persist in an active template even though no score row is produced; the template's per-use-case criterion-coverage invariant remains in force.
- Inventory direct criterion-definition SQL queries before choosing JSON extraction versus a derived projection.
