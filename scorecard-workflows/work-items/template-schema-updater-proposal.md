# Explicit template schema normalization proposal

**Primary domain:** scorecard-workflows
**Primary subdomain:** template-authoring-and-versioning
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-05-25
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

This draft separates immutable business revision from JSON schema version. It proposes deterministic, side-effect-free vN→vN+1 updaters, one explicit legacy missing-version baseline, validation after normalization, and latest-only writes. It does not propose rewriting every historical revision immediately or combining permission/evaluation revision semantics.

The proposed first implementation is frontend template-load normalization with golden old-shape fixtures; backend normalization follows for scoring and other service readers. That FE-first sequence is a historical proposal, not an established shared runtime contract. Missing-version interpretation, the actual historical version ladder, and backend write enforcement are still open. Semantic fixtures must preserve option identities, score mappings, AutoQA, N/A, and hierarchy rather than merely matching JSON shape.

## Source context and limits

Director template builder first, then go-servers normalization; proposal derived from scorecard-template concerns.md.

Inactivity evidence: Dated draft plan and log; no implementation evidence. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

Before implementation, inventory real stored schemas, decide the no-version baseline, identify canonical FE/BE load paths, and align with the newer omni-use-case template design. Do not claim the proposed updater chain already exists.

## Preserved evidence

The original folder is retained intact at [template-schema-version-updater legacy evidence](../sessions/2026-10-25/template-schema-version-updater-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/template-schema-version-updater-legacy-evidence/README.md)
- [deliverables/eng-plan.md](../sessions/2026-10-25/template-schema-version-updater-legacy-evidence/deliverables/eng-plan.md)
- [log/2026-05-25.md](../sessions/2026-10-25/template-schema-version-updater-legacy-evidence/log/2026-05-25.md)
