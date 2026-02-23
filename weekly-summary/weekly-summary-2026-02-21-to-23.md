Weekly Summary: Feb 21–23, 2026

Progress

- **Backfill scorecards (appeal cleanup):** Executed full appeal scorecard cleanup across all 8 production clusters — deleted ~70.5M scorecards + ~650.5M scores from ClickHouse, then re-backfilled 95/95 customers with clean data (no appeal scorecards). Oportun required chunked 1-day deletes (51 iterations) due to mutation size. 4 large customers (hilton, united-east, marriott, spirit) failed full-range backfill with heartbeat timeout — resolved with windowed backfills (5-day or 10-day windows based on volume), switched from sequential to parallel execution on the weekend. All completed by Feb 23 with zero failures. Verified appeal scorecards (types 1–3) correctly filtered from ClickHouse via Postgres→CH spot-check on care-oregon and 9 other customers.

Problems

- **Full-range backfill heartbeat timeout:** 4 large customers (hilton, united-east, marriott, spirit) exceeded Temporal heartbeat interval on full 51-day range. Window size selection required per-customer volume analysis: >80K scorecards/day → 5-day, <30K → 10-day.
- **Script timeout too short:** Initial 1-hour script timeout was insufficient — workflows took 2–6 hours per window. All 4 first-window workflows completed on Temporal after the script gave up. Increased to 8 hours.
- **Sequential too slow:** Estimated ~3.8 days for all 4 customers sequentially. Switched to fully parallel window execution on the weekend (lower traffic), cutting total time significantly.
- **United-east parallel windows uniformly slow:** All 8 parallel windows ran ~7 hours with none completing early, suggesting resource contention. Heartbeat monitoring (ReindexedConversations/TotalConversationCount) confirmed steady progress at ~35-50K conversations/hour per workflow.
- **Voice-prod cresta-cli cluster lookup failure:** `cresta-cli connstring` returned empty `validClusters` for voice-prod intermittently. Workaround: retry or use `AWS_REGION=us-west-2` (voice-prod RDS is in us-west-2, not us-east-1).

Plan

- **Backfill scorecards:** Done. Post to Linear ticket CONVI-6259 with final results.
- **Agent quintiles:** Resume FE implementation — Performance tables, Agent Leaderboard per metric, Coaching Hub.
- **Agent-only filter (CONVI-6247):** Phase 4 (Leaderboard) and Phase 5 (API pass-through).
