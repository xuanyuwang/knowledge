# Submitted-scorecard editing and independent lock reasons

**Primary domain:** scorecard-workflows
**Primary subdomain:** permissions-and-visibility
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-09-02
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

The final submitted-editor contract is users + teams + groups on template permissions. It covers normal conversation/process scorecards, criteria, comments, notes, and reset after submission; first submit, appeal artifacts, and calibration artifacts remain outside this rule. Empty/unset editors fall back to ordinary edit behavior rather than denying everyone. FE proactively evaluates the submitted-lock permission and retains a reactive 403 freeze fallback.

The history records two separate disagreements. CONVI-7350 changes write/reset enforcement to latest-template permissions to match proactive evaluation, while content/scoring remain pinned. Separately, acknowledged-scorecard UI locks and reset creator/admin rules can deny an otherwise submitted-editor-allowed user. The RCG August notes contain differing scorecard/request contexts; the canonical acknowledged scorecard had an empty allowlist and allowed backend evaluation, so its QA_SPECIALIST UI lock must not be conflated with a separate persisted-allowlist denial.

The August 10 decision supersedes unconditional enforcement/flag-removal proposals: staged customer configuration was the recorded rollout direction. The September 2 review of Director #22401 versus #22149 found visual fields locked through `lockReason` while autosave still used narrower legacy `readOnly`. That is historical review evidence, not a claim about today's PR head.

## Source context and limits

Director /Users/xuanyu.wang/repos/director-convi-6862, branch xwang/convi-6862-submitted-scorecard-editors-v2; related go-servers and config work.

Inactivity evidence: Latest dated PR review; earlier rollout proposals are superseded within the source. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

Continue in the canonical [CONVI-6862](CONVI-6862.md), [CONVI-7350](CONVI-7350.md), [CONVI-7598](CONVI-7598.md), and [CONVI-7612](CONVI-7612.md) items; full histories are preserved there. On resumption, refresh actual rollout and PR state; test latest permissions, acknowledged locks, reset restrictions, and consistent visual/write guards.

## Preserved evidence

The original folder is retained intact at [convi-6862-disable-editing-on-submitted-scorecard legacy evidence](../sessions/2026-10-25/convi-6862-disable-editing-on-submitted-scorecard-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/convi-6862-disable-editing-on-submitted-scorecard-legacy-evidence/README.md)
- [decisions/2026-05-29-final-fe-submitted-editor-behavior.md](../sessions/2026-10-25/convi-6862-disable-editing-on-submitted-scorecard-legacy-evidence/decisions/2026-05-29-final-fe-submitted-editor-behavior.md)
- [sessions/2026-09-02/codex-pr-22401-review.md](../sessions/2026-10-25/convi-6862-disable-editing-on-submitted-scorecard-legacy-evidence/sessions/2026-09-02/codex-pr-22401-review.md)
- [work-items/CONVI-6862.md](../sessions/2026-10-25/convi-6862-disable-editing-on-submitted-scorecard-legacy-evidence/work-items/CONVI-6862.md)
- [work-items/CONVI-7350.md](../sessions/2026-10-25/convi-6862-disable-editing-on-submitted-scorecard-legacy-evidence/work-items/CONVI-7350.md)
- [work-items/CONVI-7598.md](../sessions/2026-10-25/convi-6862-disable-editing-on-submitted-scorecard-legacy-evidence/work-items/CONVI-7598.md)
- [work-items/CONVI-7612.md](../sessions/2026-10-25/convi-6862-disable-editing-on-submitted-scorecard-legacy-evidence/work-items/CONVI-7612.md)
