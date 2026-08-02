# Session: Coaching Hub / QM Report time-basis investigation

Date: 2026-07-30
Tool: Cursor
Ticket: CONVI-7162

## Goal

Confirm whether Coaching Hub and QM Report (ticket’s comparison surfaces) use scorecard submit time under the hood.

## Method

- Linear ticket CONVI-7162 for expected comparison step.
- Director FE call-site tracing (Coaching Hub, QA Report).
- go-servers SQL verification for `submitted_at` filters.

## Result

Stored in `deliverables/coaching-hub-qm-report-submit-time-investigation.md`.

**Summary:** Both Hub Scorecards (`RetrieveCoachingOverviews`) and QM evaluated counts (`RetrieveDirectorTaskStats` / `RetrieveManualQAStats`) filter/group on `submitted_at`. Manager Leaderboard uses `RetrieveQAScoreStats` with default `scorecard_time` — that mismatch is the ticket’s root cause relative to Hub/QM.
