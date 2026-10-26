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
- The current labeler has a confirmed long-session false-negative: since `7950abdd8e`, its ClickHouse event query selects sessions by start time inside a ±3-hour batch window instead of true interval overlap. An Agent Assist session can span all agent messages but be omitted if it began too early. See [INSI-4262](../../work-items/INSI-4262.md).

## Consolidated Historical Cases

- [CONVI-6192](../../work-items/CONVI-6192.md): conversation-source eligibility and actual adoption evidence are different questions. The February proposal excluded postcall source 8 while retaining optional bot behavior; its rollout was not established by the archived note.
- [CONVI-6242](../../work-items/CONVI-6242.md): open-conversation labeling plus a forward-only watermark can retain stale assignment/use-case values. ReplacingMergeTree cannot reconcile rows with different sorting keys, even with `FINAL`. The February Alaska repair recorded zero duplicates after regeneration; its remaining PG/CH population difference was not fully explained. Historical cleanup commands depend on the old key and database layout and are not current instructions.

## Legacy Sources and Cases

- [med-mutual-active-days-and-behavior-guides](../../work-items/med-mutual-active-days-and-behavior-guides.md)
- [active-days-join-and-stale-label-diagnostics](../../work-items/active-days-join-and-stale-label-diagnostics.md)
- [CONVI-6192](../../work-items/CONVI-6192.md)
- [CONVI-6242](../../work-items/CONVI-6242.md)
- `analytics/sessions/2026-10-25/agent-stats-analytics-behaviors-legacy-evidence/deliverables/active-days-behavior-guide-2026-06.md`

## Operational Checks

- Compare conversation source, label rows, agent/use-case assignment, message time, online-session intervals, and heartbeat adoption before classifying the symptom.
- Compare the whole online interval to message times; do not treat absence from the labeler's session-start window as absence of a qualifying online session.
- Separate “0” (evidence evaluated false) from “N/A” (row/denominator absent).

## Open Questions

- Define the complete canonical truth table for legacy, fallback, and message-granular rows.
- Add freshness/coverage monitoring for label generation and reassignment.

See the [October 25 historical synthesis](../../deliverables/inactive-legacy-project-consolidation-2026-10-25.md) for consolidated cases, later superseding decisions, and their evidence limits.
