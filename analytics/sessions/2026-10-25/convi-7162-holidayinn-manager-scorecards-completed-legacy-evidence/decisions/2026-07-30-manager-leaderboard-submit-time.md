# Decision: Manager Leaderboard Scorecards Completed uses scorecard submit time

Date: 2026-07-30
Ticket: [CONVI-7162](https://linear.app/cresta/issue/CONVI-7162/holiday-inn-club-vacations-manager-leaderboard-scorecards-completed)
Slack: https://cresta.enterprise.slack.com/archives/C0BLS0UE02X
Participants: Tinglin Liu (decision), Krystal Truong (historical context), Xuanyu Wang (implementation)

## Decision

**Restore Manager Leaderboard “Scorecards Completed” to scorecard submit time** (not conversation / interaction start time).

## Why Tinglin decided to switch back

Tinglin initially believed Leaderboard had always used conversation time. After Krystal clarified the history, Tinglin reversed that assumption and chose submit time **because the pre–CONVI-6968 behavior was submit time**, and the conversation-time semantics were an unintended consequence of the QA API migration—not an intentional long-standing product choice.

Key quote (Tinglin Liu, ~12:06 PM):

> i see, I thought it was always using the conversation time. If so, I think we should update the leaderboard page to use the scorecard submit time instead

In other words: **restore prior semantics**, not invent a new Hub/QM-alignment policy in isolation. The deciding fact was “it used to be submit time, then CONVI-6968 changed it.”

## Context that informed the decision

Krystal Truong (~12:05 PM):

> We originally used scorecard submit time, but since we introduced [CONVI-6968](https://linear.app/cresta/issue/CONVI-6968/schwab-improve-leaderboard-to-support-launch) it changed the Manager Leaderboard scorecard API migration to now use QA scorecard APIs (which uses conversation start time)

Tinglin’s preceding questions (~11:56–11:57 AM) framed the issue as regression vs original behavior:

> what timestamp did leaderboard page use before?
> is that it's using the scorecard submit time and later on we changed to conversation time?

## Chronological summary

| Time (local, ~Jul 30) | Who | What |
|-----------------------|-----|------|
| 11:56 AM | Tinglin | Asked what timestamp Leaderboard used before (re CONVI-7162) |
| 11:57 AM | Tinglin | Hypothesized: was submit time, later changed to conversation time |
| 12:05 PM | Krystal | Confirmed: original = submit time; CONVI-6968 QA API migration → conversation start time |
| 12:06 PM | Tinglin | Decision: update Leaderboard back to scorecard submit time (had thought it was always conversation time) |
| 12:14 PM | Xuanyu | Agreed; fix already prepared; wrap up same day |
| 12:15 PM | Tinglin | Thanks |

## Implications

- Treat CONVI-7162 as **restoring pre–CONVI-6968 Manager Leaderboard submit-time semantics**, not as “prefer Hub/QM over Performance Insights.”
- Earlier product brainstorm about keeping interaction-time alignment with Performance Insights is **superseded** by this decision.
- Implementation path remains Manager-only opt-in to `ScorecardTimeBasis=SUBMIT_TIME` on QA scorecard APIs (proto / go-servers / director work in flight).

## Source fidelity

Captured from messages pasted by Xuanyu Wang on 2026-07-30. Channel archive link: https://cresta.enterprise.slack.com/archives/C0BLS0UE02X. Exact message permalinks / thread ts not available in the paste.
