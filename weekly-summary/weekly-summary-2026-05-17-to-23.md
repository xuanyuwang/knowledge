# Weekly Summary - Week of 2026-05-17

**Created:** 2026-05-22

## Progresses

### CONVI-6841: Process Scorecard Update Race Condition
- Investigated customer report of "coaching submission error" — reframed as process scorecard failing to update on creation.
- Traced the root cause to read-after-write inconsistency: `UpdateScorecard` reads from a Postgres replica before the newly created scorecard is visible.
- Implemented a narrow backend fix: force the initial `UpdateScorecard` existence-check to read from the write DB.
- Opened PR https://github.com/cresta/go-servers/pull/27934, iterated after CI failures to narrow the fix scope.
- Verified from code that the failure is Postgres-only (not ClickHouse), removed an alternate theory from the knowledge doc.
- **Status:** PR open, fix narrowed and CI-passing.

### CONVI-6862: Disable Editing on Submitted Scorecard
- Reviewed existing knowledge base across scorecard templates, appeal flows, reversed scorecards, and template permissions.
- Confirmed no dedicated post-submit edit permission exists today; submitted scorecards are currently still updatable.
- Created an initial engineering design draft for hard-locking everything post-submit, with space for future fine-tuning.
- After the 2026-05-22 Linear thread, narrowed scope: only normal Closed Conversations and process scorecards in scope; appeals and group calibration out of scope.
- Pivoted proto/design direction from role-based `submitted_scorecard_editors` to audience-style permitted-user configuration.
- Updated docs to include `ResetScorecard` in the submitted-lock scope and to allow first submit for unsubmitted scorecards.
- Prepared backend worktree for a reset-and-restart from `origin/main`.
- **Status:** Design refined, implementation not yet started.

### Scorecard & Template Working Reference
- Restructured the scorecard-template project from a single system-reference document into a layered working reference.
- Added six new deliverables: domain skeleton, concept map, template lifecycle, scorecard lifecycle, business-rules catalog, and ticket-pattern log.
- Updated README, project.yaml, and reading order to reflect the new structure.
- **Status:** New deliverables written and uncommitted, ready for review and commit.

### Staff Growth Framing
- Added Project 4 (Scorecard/Template Domain Stewardship) to the staff-project growth document.
- Framed the shift from local ticket execution to domain stewardship as a Staff-level growth axis.
- **Status:** Draft added, uncommitted.

## Problems

### Technical Issues
- CONVI-6841 initial fix was too broad and broke `TestScorecardAsyncOrder` in CI. Resolved by narrowing the forced-primary read to just the single existence-check lookup.

### Blockers
- CONVI-6862 product scope was initially unclear (which scorecard types, which operations). Resolved through the 2026-05-22 Linear thread — scope is now confirmed and documented.

### Learnings from Failures
- The CONVI-6841 root cause (replica lag on read-after-write) is a pattern that could recur wherever a create-then-immediately-update flow exists. Worth watching for in other coaching flows.

## Plan

### Next Week Priorities
1. Get CONVI-6841 PR reviewed and merged.
2. Begin CONVI-6862 backend implementation: proto changes for audience-style permitted-user config, server-side enforcement in `UpdateScorecard` / `ResetScorecard`.
3. Commit the scorecard-template working reference deliverables.

### Follow-ups Required
- Refresh Linear issue CONVI-6862 to match the updated knowledge snapshot.
- Add a BE/FE execution-plan comment on the Linear ticket pointing to the knowledge plan.

### Pending Reviews/Decisions
- CONVI-6841 PR awaiting review.
- CONVI-6862 proto design (audience-style permitted-user config) needs team alignment before implementation.
