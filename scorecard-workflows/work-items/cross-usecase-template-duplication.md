# Cross-use-case template duplication preserves portable semantics

**Primary domain:** scorecard-workflows
**Primary subdomain:** template-authoring-and-versioning
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-02-06
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

CONVI-6116 reused CreateScorecardTemplate through a frontend warning/select flow rather than adding a backend API. The selected destination is in the same profile, excludes the current use case and CARE_EFFICIENCY, and preserves template type. Archived templates cannot use the action.

The copy retains structure, scoring, and role permissions, resets audience to all agents, clears QA task configuration and AutoQA/Opera links, and changes use-case scope. Outcome references are selectively retained only when available in the destination; unavailable outcomes become a new empty outcome criterion. Target-filtered metadata is therefore part of correctness, not merely picker UI. The implementation summary records this behavior; the preserved test plan is a plan rather than proof of deployment.

## Source context and limits

Director template builder, DuplicateScorecardTemplateModal.tsx, useOutcomeMetadata.ts; implementation summary for CONVI-6116.

Inactivity evidence: Implementation summary has this date; log/2025-02-11.md conflicts with the 2026 implementation and is not a reliable latest date. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

If resumed, validate destination metadata and reset behavior against the current builder, including delayed metadata loading, unavailable outcomes, permissions, task clearing, and audience. Do not copy source-use-case automation links blindly.

## Preserved evidence

The original folder is retained intact at [duplicate-template-across-usecase legacy evidence](../sessions/2026-10-25/duplicate-template-across-usecase-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [duplicate-template-implementation-summary.md](../sessions/2026-10-25/duplicate-template-across-usecase-legacy-evidence/duplicate-template-implementation-summary.md)
- [investigation.md](../sessions/2026-10-25/duplicate-template-across-usecase-legacy-evidence/investigation.md)
- [pm-review.md](../sessions/2026-10-25/duplicate-template-across-usecase-legacy-evidence/pm-review.md)
- [test-plan.md](../sessions/2026-10-25/duplicate-template-across-usecase-legacy-evidence/test-plan.md)
