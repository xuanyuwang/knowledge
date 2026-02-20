Weekly Summary: Feb 17–20, 2026

Progress

- **Agent quintiles (CONVI-new):** Full BE implementation — proto `QuintileRank` enum, `setQuintileRankForPerAgentScores` with true percentile-based ranking (flat, not score bands), 14+ unit tests, go-servers PR #25795 open with review fixes (AGENT_TIER leak, SliceStable, defense-in-depth). Config feature flag `enableQuintileRank` merged (config #140396). FE Agent Leaderboard implemented — `QuintileRankIcon` component (gold/silver/bronze), column after Live Assist, gated on feature flag.
- **Agent-only filter (CONVI-6247):** Completed Phases 2–3 — FE types (`listAgentOnly`, `FilterKey.LIST_AGENT_ONLY`), Performance page "Agents only" BooleanFilter with i18n, consolidated into single PR (director #16777). Phase 4 (Leaderboard) and Phase 5 (API pass-through) remain.
- **User filter consolidation (B-SF-3):** Fixed Divergence 5 — `ParseUserFilterForAnalytics` now uses UNION instead of INTERSECTION for combined user+group selections. 3 new tests added, all 62 tests pass. PR #25829 merged to main.
- **CONVI-6260 (team leaderboard):** Cherry-picked bugbot fix for dynamic group filters dropping team aggregates, restored customer profile filtering comment. PR #25733 has 4 commits addressing all review comments.
- **CONVI-6242 (cron-label-conversations):** Completed full Alaska Air 2026 backfill — bulk deleted 1.65M rows, re-ran 49 daily cron jobs in 16.5 minutes, result: 1.54M clean rows with 0 duplicates, ~100K stale usecase labels corrected.
- **Backfill scorecards:** Completed Mutual of Omaha backfill on voice-prod (delete + 5 parallel jobs, 27 min, appeal scorecards removed: -31K scorecards, -272K scores). Created rollout tooling (`list_ch_databases.sh`, `cluster_cleanup.py`) for all-customer cleanup across 8 clusters (~260 customers total).

Problems

- **Quintile definition changed mid-implementation:** Originally used fixed score bands (80+→Q1, etc.), but realized all agents could cluster in same band. Pivoted to true percentile-based ranking, requiring rewrite of BE logic and test expectations.
- **Quintile rank leaked into AGENT_TIER responses:** `setQuintileRankForPerAgentScores` was called in the ClickHouse converter for all grouping types, not just AGENT. Required removing the misplaced call and adding defense-in-depth clearing in `createTieredScoreObject`.
- **FE quintile work still incomplete:** Agent Leaderboard done, but Performance tables (2 table types), Agent Leaderboard per metric, and Coaching Hub all still need quintile columns/icons. Coaching Hub tooltip "based on last 7 days" may imply a separate time window — needs product confirmation.
- **Appeal scorecard cleanup is large-scale:** 8 clusters, ~260 customers. Large customers (cvs, oportun) need 1-day sequential splits (51 days each). Execution planned for weekend but carries operational risk.

Plan

- **Agent quintiles:** Get go-servers PR #25795 merged. Implement remaining FE pages — Performance tables (LeaderboardByScorecardTemplateItem, LeaderboardPerCriterion), Agent Leaderboard per metric, Coaching Hub. Clarify Coaching Hub "last 7 days" tooltip requirement with product.
- **Agent-only filter (CONVI-6247):** Phase 4 (Leaderboard page BooleanFilter, disable on Manager tab) and Phase 5 (API pass-through replacing hardcoded `filterToAgentsOnly: true`).
- **Backfill scorecards:** Execute all-customer appeal scorecard cleanup across 8 clusters over the weekend using `cluster_cleanup.py`.
- **User filter consolidation:** Continue phased migration — the B-SF-3 fix is merged, proceed with remaining divergences and API migrations.
