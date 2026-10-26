# Export the comment corresponding to the resolved criterion

**Primary domain:** scorecard-workflows
**Primary subdomain:** appeals
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-04-16
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

The original CSV comment came from the original score row even after an appeal changed the final criterion. The recorded implementation loads submitted resolve scorecards through Original → Replica → Request → Resolve using three forward joins rather than the general bidirectional recursive CTE. It builds an original-scorecard/criterion comment map and replaces the original comment with the latest submitted resolve reason; collaboration comments are still appended.

`Score.comment` is overloaded by scorecard type: grader comment, appeal reason, or resolution reason. Choosing a comment requires the workflow relation and latest submitted round, not merely the original row. If appeal lookup fails, export proceeds without appeal comments and logs the error. The notes record implementation but provide no test transcript or live shipment confirmation.

## Source context and limits

go-servers apiserver/internal/coaching/action_export_scorecards.go; shared/scoring/appeal_workflow.go; original/replica/request/resolve chain.

Inactivity evidence: Log records April 15 implementation and April 16 capture. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

Before extending, verify approved versus rejected resolution behavior, latest-round selection, empty reasons, and fallback on lookup failure. Keep collaboration-comment appending separate from the chosen criterion reason.

## Preserved evidence

The original folder is retained intact at [export-appeal-comments legacy evidence](../sessions/2026-10-25/export-appeal-comments-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [research.md](../sessions/2026-10-25/export-appeal-comments-legacy-evidence/research.md)
- [log/2026-04-15.md](../sessions/2026-10-25/export-appeal-comments-legacy-evidence/log/2026-04-15.md)
- [log/2026-04-16.md](../sessions/2026-10-25/export-appeal-comments-legacy-evidence/log/2026-04-16.md)
