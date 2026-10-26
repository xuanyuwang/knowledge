# N/A identity, score semantics, and feature fallback

**Primary domain:** scorecard-workflows
**Primary subdomain:** evaluation-and-scoring
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-06-01
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

N/A with no score and N/A with a numeric score are different persisted representations. Legacy N/A uses notApplicable with no numeric value; scored N/A is a numeric option marked isNA with notApplicable false. Option values are lookup identities, scores are separate business values, and array position is neither. AutoQA mapping therefore needs criterion option metadata rather than inferring N/A solely from numeric values.

The early feature fixes null-score labels, prevents accidental sequential score population on AutoQA disable, and permits null only for N/A while normal options require nonnegative numeric scores. Legacy showNA migration remained, while incidental frontend score-array migration was removed; DEFAULT_CRITERION compatibility was deferred rather than silently redesigned.

June follow-ups are independent runtime bugs: CONVI-6962 normalizes scored N/A using parent criterion metadata before branch matching and combines N/A/numeric conditions with OR; CONVI-6960 converts stored scored N/A back to the legacy representation when enableNAScore is off across rendering, hydration, save, appeals, and process scoring. CONVI-6955 concerns required-comment selection remapping. A May checklist records 55 scenarios and a duplicate-N/A branching issue, but is not proof that every later change passed that suite.

## Source context and limits

Director branch xuanyu/enable-na-score; go-servers AutoQA mapper analysis; later Director branches xwang/convi-6962-scored-na-branching and xwang/convi-6960-na-flag-fallback; historical PRs #19237 and #19236.

Inactivity evidence: Later dated CONVI-6955/6960/6962 notes extend the May 13 manual checklist. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

Recheck current representations before reuse; exercise numeric N/A, no-score N/A, flag on/off reopening and saving, reordered option values, mixed branch conditions, and required comments. Keep deferred default/migration/refactor ideas separate from recorded fixes.

## Preserved evidence

The original folder is retained intact at [nascore legacy evidence](../sessions/2026-10-25/nascore-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/nascore-legacy-evidence/README.md)
- [solution-b-implemented.md](../sessions/2026-10-25/nascore-legacy-evidence/solution-b-implemented.md)
- [na-score-lifecycle.md](../sessions/2026-10-25/nascore-legacy-evidence/na-score-lifecycle.md)
- [convi-6955-required-comments-remapping.md](../sessions/2026-10-25/nascore-legacy-evidence/convi-6955-required-comments-remapping.md)
- [convi-6962-scored-na-branching.md](../sessions/2026-10-25/nascore-legacy-evidence/convi-6962-scored-na-branching.md)
- [convi-6960-na-flag-fallback.md](../sessions/2026-10-25/nascore-legacy-evidence/convi-6960-na-flag-fallback.md)
- [deliverables/manual-test-checklist.md](../sessions/2026-10-25/nascore-legacy-evidence/deliverables/manual-test-checklist.md)
