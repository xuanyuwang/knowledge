# Read-after-create race in process scorecard autosave

**Primary domain:** scorecard-workflows
**Primary subdomain:** scorecard-lifecycle
**State:** historical synthesis; live ticket/PR/deployment state not refreshed
**Last meaningful source content:** 2026-05-27
**Consolidated:** 2026-10-25

## Meaning and recorded outcome

Director creates a process scorecard, stores its returned name, then autosaves using `UpdateScorecard`. Its first database existence read could use a lagging replica and return `NOT_FOUND` for the just-created row. The reported coaching submission error was actually the UpdateScorecard toast.

The recorded fix pins only the initial `dao.GetScorecards` lookup to `dbresolver.Write`. Pinning the entire function had failed `TestScorecardAsyncOrder`, so it was narrowed. Focused local testing was constrained by Docker-backed ClickHouse fixtures. The May 27 follow-up distinguishes apiserver customer traffic from runner traffic: the dashboard was apiserver-only, whereas later NOT_FOUND and PERMISSION_DENIED spans came from runners and did not by themselves reproduce the customer incident.

## Source context and limits

go-servers main checkout; branch convi-6841-update-scorecard-write-db-read; historical go-servers PR #27934.

Inactivity evidence: Later monitoring interpretation is meaningful beyond the May 19 implementation. All substantive evidence reviewed predates the 2026-09-25 cutoff. Filesystem timestamps and later bulk Git capture do not establish later engineering activity. Statements about tests, implementation, rollout, and PRs above describe the retained record, not checks rerun during this documentation move.

## Continuing work

If reopened, check the exact initial lookup and fresh-create/autosave behavior, then compare apiserver NOT_FOUND traffic. Do not use runner-only permission errors as evidence that the customer race persists.

## Preserved evidence

The original folder is retained intact at [convi-6841-process-scorecard-update-race legacy evidence](../sessions/2026-10-25/convi-6841-process-scorecard-update-race-legacy-evidence/README.md); its internal chronology remains authoritative for historical detail.

- [README.md](../sessions/2026-10-25/convi-6841-process-scorecard-update-race-legacy-evidence/README.md)
- [log/2026-05-20.md](../sessions/2026-10-25/convi-6841-process-scorecard-update-race-legacy-evidence/log/2026-05-20.md)
- [log/2026-05-27.md](../sessions/2026-10-25/convi-6841-process-scorecard-update-race-legacy-evidence/log/2026-05-27.md)
