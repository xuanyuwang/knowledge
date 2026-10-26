# Opera annotations drive email AutoQA outcomes

**Primary domain:** scorecard-workflows
**Primary subdomain:** evaluation-and-scoring
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-07-08
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

Opera generates moment annotations during conversation execution; AutoQA later consumes them rather than inferring behavior from raw messages. DDX means detected/pass and DNX means not-detected/fail. SDX-only filtered evidence can yield N/A. SNX trigger annotations are not themselves bucketed as scoring evidence; synthetic SDX anchors provide their windows.

Email per-message and per-agent scorecards filter annotations against the appropriate agent message IDs and complete message order. For positive behaviors, DDX applies at its message while DNX can span a window. For SNX behaviors, successful DDX spans and failure DNX is point-only. Applying the positive rule to SNX dropped middle-agent evidence and produced N/A; COA-2566 / go-servers #29694 addressed that filtering path. Manually graded scorecards are skipped by AutoQA so automation does not overwrite them.

The reference describes ENABLE_SDX_WINDOW_FILTERING default true and a legacy direct-message fallback as of the recorded code. No live flag, deployment, or current coverage was checked during consolidation.

## Source context and limits

go-servers shared/scoring/autoqa_scoring.go and autoqa_dao.go, apiserver/internal/autoqa; TestFilterContextForMessages and email integration references.

Inactivity evidence: Single dated reference/investigation session. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

Use this model to distinguish annotation generation, window filtering, outcome mapping, and option scoring when diagnosing N/A. Refresh the actual source/flags before relying on historical defaults.

## Preserved evidence

The original folder is retained intact at [outcome legacy evidence](../sessions/2026-10-25/outcome-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/outcome-legacy-evidence/README.md)
- [adherence-types.md](../sessions/2026-10-25/outcome-legacy-evidence/adherence-types.md)
- [autoqa-scoring.md](../sessions/2026-10-25/outcome-legacy-evidence/autoqa-scoring.md)
- [email-window-filtering.md](../sessions/2026-10-25/outcome-legacy-evidence/email-window-filtering.md)
- [pr-29694-snx-fix.md](../sessions/2026-10-25/outcome-legacy-evidence/pr-29694-snx-fix.md)
