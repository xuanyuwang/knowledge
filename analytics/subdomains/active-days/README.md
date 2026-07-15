# Active Days

## Purpose

Own the exact meaning of Active Days and the evidence pipeline that decides whether an agent was active with Cresta/Agent Assist.

## Current Semantics and Invariants

- `RetrieveAgentStats` is the primary API for the Leaderboard metric.
- Source-exclusion must preserve a valid zero rather than dropping the whole agent/day row.
- Agent Assist evidence is message-granular for newer label rows; matched rows use message identity/time.
- Fallback/legacy rows without a message ID use conversation anchors and can legitimately evaluate true or false.
- New rows are attributed by message time; fallback/legacy rows use conversation start time.
- Labeling open conversations can produce stale agent/use-case metadata after reassignment; final-state labeling and cleanup/backfill are required.
- Missing heartbeats can indicate desktop adoption rather than a calculation bug; product interpretation must distinguish evidence absence from join/filter defects.

## Legacy Sources and Cases

- `active-days/`
- `agent-stats-active-days-fix/`
- `convi-6192-conversation-source-config/`
- `convi-6242-cron-label-conversations/`
- `agent-stats-analytics-behaviors/deliverables/active-days-behavior-guide-2026-06.md`

## Operational Checks

- Compare conversation source, label rows, agent/use-case assignment, message time, online-session intervals, and heartbeat adoption before classifying the symptom.
- Separate “0” (evidence evaluated false) from “N/A” (row/denominator absent).

## Open Questions

- Define the complete canonical truth table for legacy, fallback, and message-granular rows.
- Add freshness/coverage monitoring for label generation and reassignment.
