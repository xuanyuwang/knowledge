# Group Calibration CSV comments and remaining parity gaps

**Primary domain:** scorecard-workflows
**Primary subdomain:** group-calibration
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-07-06
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

CONVI-7208 adds grade/comment column pairs for both answer-key and reviewer rows. All needed criterion comments already flow from PostgreSQL scores.comment through ListScorecards FULL and Director transformation; getCSV assembles the file in the browser, so no new backend export endpoint was needed. Collaboration comments are excluded. Blank criterion cells and the existing scorecard-level Comment column remain; scorecard-level blank-versus-None behavior differs from backend export.

The accepted export did not add comment_access_roles filtering in this QA-admin download. That is an explicit recorded product/export choice, not proof that every comment-reading surface has that policy. Grade formatting was left unchanged in the comment feature.

July 6 identified separate gaps: labeled/dropdown numeric grades exported raw values (3 instead of Great), and columns could follow score order rather than template order. Existing row.template supports both improvements without a new API. Template-order partial changes were reverted awaiting a decision; numeric-label work is a fix plan, not a landed fix. The README says both shipped and PR open, so preserve implementation evidence without inferring final release status.

## Source context and limits

Director RecentCalibrationsThreeDotsMenu.tsx, buildCriterionCsvColumns.ts; historical Director PR #20388; related go-servers ExportScorecards formatting reference.

Inactivity evidence: Latest investigations include deliberately reverted template-order work. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

Refresh PR/deployment state only if reopening. Treat template order and numeric labels as separate follow-ups and validate answer key/reviewer alignment, blank comments, and explicit access expectations.

## Preserved evidence

The original folder is retained intact at [group-calibration legacy evidence](../sessions/2026-10-25/group-calibration-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/group-calibration-legacy-evidence/README.md)
- [decisions/2026-07-03-convi-7208-director-only-export.md](../sessions/2026-10-25/group-calibration-legacy-evidence/decisions/2026-07-03-convi-7208-director-only-export.md)
- [deliverables/convi-7208-pm-behavior-summary.md](../sessions/2026-10-25/group-calibration-legacy-evidence/deliverables/convi-7208-pm-behavior-summary.md)
- [log/2026-07-06.md](../sessions/2026-10-25/group-calibration-legacy-evidence/log/2026-07-06.md)
