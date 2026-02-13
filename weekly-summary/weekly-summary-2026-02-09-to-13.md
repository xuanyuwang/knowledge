Weekly Summary: Feb 9–13, 2026

Progress

- CONVI-6242 (stale labels fix): Investigated root cause (cron labels open conversations before re-assignment), implemented fix (filter by ended_at instead of created_at), merged PR #25706, closed unneeded ClickHouse schema PR #172
- CONVI-6260 (team leaderboard): Identified root cause — buildUserGroupMappings uses FetchGroups (no child expansion) instead of ListGroups, causing parent-only rows in team leaderboard. Wrote full root cause doc
- User filter consolidation: Re-evaluated project against current codebase, created 45+ behavior behavioral standard, wrote implementation plan with phased PR strategy, integrated CONVI-6260 finding as new divergence (B-GH-4 / Divergence 10), pushed Phase 1 behavioral tests (PR #25705) with CI lint fixes
- Agent stats active days (CONVI-6233): Completed FULL OUTER JOIN fix (PR #25613), verified on staging and production, created Notion doc for CSMs
- Backfill scorecards: Completed January 2026 backfill for all remaining customers (cvs, oportun) across all clusters — project done
- Deprecate FetchUsers (CONVI-6151): Scoped work to apiserver/internal/coaching (8 call sites, 6 files), created worktree and branch, planned FetchUserMapPaginated helper approach
- Hilton coaching discrepancy: Reorganized project docs from 14 scattered files into clean structure, started CONVI-6097 implementation (excludeDeactivatedUsers for RetrieveCoachingSessionStats)

Problems

- User filter consolidation is a large, multi-phase effort: 12 of ~29 APIs migrated, 17 still on old path; behavioral divergences (9 identified + 1 new from CONVI-6260) need team alignment before unification can proceed
- CONVI-6260 affects all non-Schwab clusters in production: The child group expansion bug is live for all 12 migrated APIs — needs a fix or rollback decision
- VPN drops and AWS session expiry caused repeated interruptions during backfill runs, requiring multiple resume cycles

Plan

- CONVI-6260: Discuss fix approach with team — either hotfix buildUserGroupMappings to expand child groups, or address as part of user filter consolidation Phase 3
- User filter consolidation: Get team review on behavioral standard document, land Phase 1 behavioral tests (PR #25705), begin Phase 2 (shared test infrastructure)
- CONVI-6151 (deprecate FetchUsers): Implement FetchUserMapPaginated helper and migrate the 8 call sites in coaching package
- CONVI-6097: Complete excludeDeactivatedUsers implementation for RetrieveCoachingSessionStats
