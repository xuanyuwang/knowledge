# Scorecard/template system reference and byte-size incidents

**Primary domain:** scorecard-workflows
**Primary subdomain:** template-authoring-and-versioning
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-09-22
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

The working reference distinguishes template/scorecard artifacts from lifecycle/workflow views. Persistent option values identify selections; scores carry business meaning. Scorecards pin template content revisions, while current permissions may follow a different policy. Operational persistence and analytical participation are different facts.

The June 25 empty-scorecard audit defines no-score versus analytics-empty records. Placeholder scores can be removed before persistence; live AutoQA and backfill bypass CreateScorecard and call shared scoring persistence directly. A create-API-only guard cannot enforce a universal nonempty rule. Existing valid scores can survive an empty update, process metadata can project without scores, and QA aggregation/calibration completion still requires score facts. The source leaves the product decision open between allowed drafts and invalid empty artifacts.

September splits two byte-unbounded gRPC incidents. CONVI-7663: full current-template bodies exceeded 50 MiB; RCG had 3,330 resources across 98 single-use-case sets, so a Closed Conversations-specific permission scope materially narrows the request. The persisted-scorecard Evaluate API cannot replace a template-permission hook. CONVI-7671: exporter used deprecated conversation_ids and no comment type, overfetching image-heavy AI feedback it later discarded. Final Sep 22 design sends nested conversation/type filters and reuses strip_inline_images for direct HTML-to-text conversion before serialization. A 5 MiB image test passed, but large plain text still needs byte protection.

Deep references also cover CSV parity, template lifecycle, business rules, options/N/A, and analytics attribute mappings. Projection mechanics stay in Scorecard Data Sync; analytics API interpretation belongs in Analytics with links to this evidence.

## Source context and limits

go-servers, Director, cresta-proto; empty-scorecard audit source heads eb72d7bb87a43052fffed65d3a9f518c889d8f7d / 301610c1edef5708c0e1c71d4f3bd81e8aed6888; CONVI-7671 PRs proto #9876/#9969 and go-servers #32304.

Inactivity evidence: Oct 4 commit 39db3b4 bulk-records late-September notes; Sep 22 dated work item/session/log contains the last substantive change. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

Continue in canonical [CONVI-7663](CONVI-7663.md) and [CONVI-7671](CONVI-7671.md), whose complete timelines were preserved and evidence links updated. Treat recorded PR/rollout status as historical. If resumed, verify deployed request scope, exact permission equivalence, byte limits, and oversized single-record handling.

## Preserved evidence

The original folder is retained intact at [scorecard-template legacy evidence](../sessions/2026-10-25/scorecard-template-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/scorecard-template-legacy-evidence/README.md)
- [deliverables/business-rules-catalog.md](../sessions/2026-10-25/scorecard-template-legacy-evidence/deliverables/business-rules-catalog.md)
- [deliverables/empty-scorecards-workflow-and-api-analysis.md](../sessions/2026-10-25/scorecard-template-legacy-evidence/deliverables/empty-scorecards-workflow-and-api-analysis.md)
- [deliverables/scorecard-export-paths.md](../sessions/2026-10-25/scorecard-template-legacy-evidence/deliverables/scorecard-export-paths.md)
- [work-items/CONVI-7663.md](../sessions/2026-10-25/scorecard-template-legacy-evidence/work-items/CONVI-7663.md)
- [work-items/CONVI-7671.md](../sessions/2026-10-25/scorecard-template-legacy-evidence/work-items/CONVI-7671.md)
- [sessions/2026-09-22/codex-convi-7671-html-to-text.md](../sessions/2026-10-25/scorecard-template-legacy-evidence/sessions/2026-09-22/codex-convi-7671-html-to-text.md)
- [log/2026-09-22.md](../sessions/2026-10-25/scorecard-template-legacy-evidence/log/2026-09-22.md)
