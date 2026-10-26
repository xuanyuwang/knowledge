# NRG reversed labels caused by option value versus index

**Primary domain:** scorecard-workflows
**Primary subdomain:** evaluation-and-scoring
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-05-02
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

The NRG Canada Retention investigation traced correct stored predicted-resolution and CSAT values through a wrong frontend display. AutoQA saved `option.value`; read-only decoupled input options used array indexes. For options `[value=1 Not Resolved, value=0 Resolved]`, stored value 0 was displayed as index 0, Not Resolved. CSAT value 2 had no index 2 and displayed missing. Filters and conversation detail were correct, and `useScoreLabelProvider` already mapped option values correctly.

The recommended correction was to translate stored option values to input indexes during form initialization, covering both read-only and interactive controls. This was an investigation recommendation; the retained evidence does not establish a shipped correction. Changing AutoQA storage to array indexes was rejected as requiring historical-data migration.

## Source context and limits

Director CriterionInputDisplay.tsx, CriterionMain.tsx and scoring utils.ts; go-servers autoqa_mapper.go; recorded template revision 5e2a5bcc.

Inactivity evidence: Dated investigation and log agree. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

Recheck the current input representation before resuming. Keep option identity, array position, and business score distinct, then test reordered/nonsequential options through hydration, display, and save.

## Preserved evidence

The original folder is retained intact at [convi-6709-reversed-scorecard legacy evidence](../sessions/2026-10-25/convi-6709-reversed-scorecard-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/convi-6709-reversed-scorecard-legacy-evidence/README.md)
- [exact-flow.md](../sessions/2026-10-25/convi-6709-reversed-scorecard-legacy-evidence/exact-flow.md)
- [log/2026-05-02.md](../sessions/2026-10-25/convi-6709-reversed-scorecard-legacy-evidence/log/2026-05-02.md)
