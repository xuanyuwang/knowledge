# Scorecard access Advanced layout

**Primary domain:** scorecard-workflows
**Primary subdomain:** permissions-and-visibility
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-07-09
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

The submitted-editors control needs permission-style description-above-dropdown layout. Replacing it with `DirectorFormInputWithLabel` moved the description to the wrong location. The recorded correction keeps `PermissionInputWithLabel` and changes its split layout gap, label minimum width, and font weights to align surrounding access controls. Require publish receives centered label alignment.

Trailing parenthetical text is rendered secondarily without changing translations; attempted locale JSON copy edits were reverted. Package lint and an opened-page screenshot were the recorded checks. Root lint printed success but returned status 1, so it was not counted as a clean root-level check.

## Source context and limits

Director /Users/xuanyu.wang/repos/director-convi-7237; branch convi-7237-performance-config-template-scorecard-access-tab-advanced.

Inactivity evidence: Single dated session/log. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

If revisited, compare the Advanced section with the flag enabled and preserve the existing permission-control description placement. No live merge status was established.

## Preserved evidence

The original folder is retained intact at [convi-7237 legacy evidence](../sessions/2026-10-25/convi-7237-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/convi-7237-legacy-evidence/README.md)
- [sessions/2026-07-09/codex-scorecard-access-advanced.md](../sessions/2026-10-25/convi-7237-legacy-evidence/sessions/2026-07-09/codex-scorecard-access-advanced.md)
- [log/2026-07-09.md](../sessions/2026-10-25/convi-7237-legacy-evidence/log/2026-07-09.md)
