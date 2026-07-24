# Time Range Filter Timestamp Investigation

**Status:** active
**Primary domain:** `analytics`
**Primary subdomain:** none (cross-cutting: `shared-analytics-platform`, `performance-insights`, `leaderboard`)
**Official ticket:** none — self-directed investigation
**Last updated:** 2026-07-18

## Objective and Impact

- **Objective:** Map every UI component on the Performance Insights and Leaderboard pages to the exact timestamp column(s) the time range filter is applied to in the backend.
- **Customer/system impact:** Enables precise reasoning about what data a user sees for a given date range — critical for diagnosing "wrong numbers" tickets and for understanding cross-page consistency.
- **Role:** led

## Scope

**In scope**

- UI components on Performance Insights page and their API calls
- UI components on Leaderboard page (Agent, Team, Manager tabs) and their API calls
- Backend handler logic that applies the time range filter to ClickHouse queries
- Which timestamp column each API uses: `scorecard_submit_time`, `scorecard_time` (conversation start time), `conversation_start_time`, or other
- How process scorecards (no conversation) are handled by the time range filter
- How API responses are combined/transformed before display in a single chart or table

**Non-goals**

- Changing the time range behavior
- Investigating non-time-range filters (user filter, outcome filter, etc.) — except where they interact with time range
- Scorecard lifecycle or sync mechanics

## Source Context

- **Repos:** `director`, `go-servers`, `cresta-proto`
- **Worktrees:** main checkouts
- **Branches:** `main`
- **PRs/commits:** none yet

## Current Understanding

Phase 1-2 complete. The full mapping is in `deliverables/time-range-timestamp-mapping.md`.

**Core finding**: The time range filter maps to different ClickHouse timestamp columns depending on which API serves the data:

- **Scorecard/QA APIs** (`RetrieveQAScoreStats`): filter by `scorecard_time` — the conversation start time associated with the scorecard
- **Conversation/Agent APIs** (`RetrieveConversationStats`, `RetrieveAgentStats`, `RetrieveAssistanceStats`, etc.): filter by `conversation_start_time`
- **Scorecard activity APIs** (`RetrieveScorecardStats`, `RetrieveCommentStats`): filter by `scorecard_submit_time` — when the scorecard was submitted

This means the Leaderboard page mixes timestamp semantics: conversation volume uses `conversation_start_time`, QA scores use `scorecard_time`, and manager comments use `scorecard_submit_time`. Performance Insights (Conversations tab) is internally consistent on `scorecard_time`.

## Findings and Decisions

- All PI Conversations tab components use `RetrieveQAScoreStats` → `scorecard_time`
- All PI Agent Outcomes tab components use `RetrieveUserOutcomeStats` → timestamp TBD
- Leaderboard Agent/Team tabs mix ~8 different API hooks, each with its own timestamp column
- Manager tab adds `RetrieveCoachingSessionStats` and `RetrieveCommentStats` (uses `scorecard_submit_time`)
- Backend column name mapping: `conversation_start_time` is remapped to `scorecard_time` or `conversation_time` in score tables

## Blockers and Dependencies

- Process scorecard `scorecard_time` population needs verification against scorecard-data-sync

## Validation and Rollout

- N/A — investigation deliverable, no code changes

## Next Actions

1. ~~Catalog UI components on Performance Insights and Leaderboard pages, map each to its API hook and request~~ (done)
2. ~~Trace each API call into the backend handler, identify the timestamp column used for time range filtering~~ (done)
3. ~~Refine to column-level granularity for all table components~~ (done)
4. ~~Trace remaining APIs: RetrieveUserOutcomeStats, RetrieveManagerStats, RetrieveCoachingSessionStats~~ (done)
5. Verify process scorecard `scorecard_time` population
6. Check if any frontend component passes `conversationTimeRangeField` to switch timestamp target
7. Update subdomain READMEs with time range semantics

## Timeline

- 2026-07-18 — Phase 1-2 complete. Mapped all PI and Leaderboard UI components to API calls and backend timestamp columns. Deliverable: `deliverables/time-range-timestamp-mapping.md`. Session: `sessions/2026-07-18/claude-time-range-filter.md`
- 2026-07-18 — Phase 2-3 complete. All APIs traced including RetrieveUserOutcomeStats (`outcome_time`), RetrieveManagerStats (`event_time`), RetrieveCoachingSessionStats (`manager_submitted_at` on PostgreSQL). Refined deliverable to column-level granularity for all tables. Manager Leaderboard confirmed as most heterogeneous: 5 APIs, 4 timestamp columns, 2 databases.
