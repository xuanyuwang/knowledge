# Weekly Summary - Week of 2026-03-16

**Created:** 2026-03-20

## Progresses

### CONVI-6247: Agent-Only Filter
- Resolved FE merge conflicts and implemented Agent Assist backward compatibility with `filterToAgentsOnly ?? true` default covering 29+ call sites
- Completed PR #17356 (FE backward compat) and PR #17394 (Agent Assist filter UI)
- All 4 FE PRs + BE PR #26301 merged and deployed to prod
- **Current status**: All PRs merged and deployed, ready for feature flag enablement and E2E testing

### User Filter Consolidation
- Changed strategy to "unify first, migrate later" to de-risk migration
- Completed Phase 1: 62 behavioral tests with 0 gaps, marked 3 tests for revision in unified impl
- Completed Phase 2: Created `shared/user-filter/types.go` and `options.go` (206 lines total)
- Key design decisions: reuse proto types (`LiteUser`/`LiteGroup`), interface named `Parser`, functional options pattern for coaching defaults
- **Current status**: PR #26451 in review, ready to start Phase 3 (Parse implementation)

### CONVI-6298: ReindexScorecards
- Addressed 12 review comments (4 from tinglinliu, 8 from flatplate): added template cache, simplified switch logic, used proto enums
- Fixed 2 blocking bugs: distributed table DELETE and hardcoded cleanup flag
- Bug fix PR #26417 merged and deployed to staging
- Verified both write path and cleanup path on walter-dev (196 scorecards, 721 scores)
- **Current status**: Staging verified, ready for prod rollout

### Agent Quintiles: Directionality Support
- Created comprehensive implementation plan covering both RetrieveUserOutcomeStats and RetrieveQAScoreStats
- Chose unified shared directionality approach with 3-PR strategy
- Implemented `shared/scoring/directionality.go` with 5 exported functions + 17 tests (all passing)
- Closed draft PR #26332 (superseded by shared package approach)
- **Current status**: PR #26430 in review, ready for analytics integration

### Large User ID ClickHouse
- Created flux-deployments PRs for voice-prod (#264060) and global rollout (#264076)
- Moved flag from per-cluster patches to app-level `00-head`
- Added flag directly to all 3 release stages (PR #264149) after discovering releaser gap
- **Current status**: Flag enabled globally, monitoring for regressions

### Insights User Filter: Wrong Team Mapping
- Investigated bswift wrong team mapping issue
- Root cause identified: duplicate GUEST_USER accounts passing `ListUsersForAnalytics` filters
- Found all 3 affected agents still have conversations routing to GUEST user IDs
- **Current status**: Investigation complete, requires platform-side fix

## Problems

### Technical Issues

**ReindexScorecards bugs found during testing:**
- **Bug 1**: Attempted DELETE on distributed tables (`score_d`, `scorecard_d`) — ClickHouse doesn't support DELETE on distributed tables
  - **Resolution**: Fixed to use local tables (`score`, `scorecard`) with `ON CLUSTER 'conversations'` mutation
- **Bug 2**: Cron job hardcoded `CleanUpBeforeWrite: true` with no env var override
  - **Resolution**: Made configurable via `REINDEX_SCORECARDS_CLEAN_UP_BEFORE_WRITE` env var, defaults to `false`
- **Impact**: Every cron-triggered reindex would have failed. Both bugs fixed in PR #26417 and verified on staging.

**Missing `filter_to_agents_only` field on prod QA score stats:**
- **Root cause**: `modifiedFiltersState` set `listAgentOnly: undefined` when feature flag off, causing field omission in `RetrieveQAScoreStats` requests
- **Resolution**: Already fixed in PR #17356 (deployed to staging, then prod) — changed default from `undefined` to `true`
- **Impact**: No actual metric impact (BE hardcoded `true` anyway), but field was missing from request payload

**Flux releaser gap for env-only changes:**
- **Problem**: Releaser only promotes `00-head` → staging → prod on image builds, not env var changes alone
- **Impact**: After removing per-cluster patches, flag was effectively OFF everywhere until manual stage file updates
- **Resolution**: Created PR #264149 to directly update all 3 release stages
- **Lesson**: For env-only changes, update stage files directly to avoid regressions

**Platform routing to duplicate GUEST accounts:**
- **Problem**: 3 bswift agents have duplicate GUEST_USER and PROD_USER accounts; platform still routes conversations to old GUEST accounts
- **Impact**: Wrong team mapping in Performance page, ClickHouse cleanup alone won't fix it
- **Status**: Investigation complete, requires coordination with platform team for routing fix

### Learnings from Failures

1. **ClickHouse table types matter**: Distributed tables (`_d`) are for queries only, not mutations. Always mutate local ReplicatedReplacingMergeTree tables with `ON CLUSTER`.

2. **Feature flag defaults**: Using `undefined` as default when flag is off causes field omission in API requests. Always use explicit boolean defaults (`true`/`false`).

3. **Releaser behavior assumptions**: Don't assume releaser will propagate env-only changes promptly. Update stage files directly for critical flags.

4. **Test profile data characteristics**: High filter rates (3524 → 196 scorecards) on walter-dev are expected due to frequently-edited templates. This matches real-time path behavior, not a bug.

## Plan

### Next Week Priorities

1. **User-filter-consolidation Phase 3**: Implement `Parser.Parse()` with 3-4 PRs:
   - Base population + ACL filtering
   - Selection filtering (selected users/groups)
   - Membership tracking (direct + hierarchical)
   - Retarget behavioral tests to unified implementation

2. **CONVI-6247 E2E testing**: Enable `enableAgentOnlyFilter` feature flag on staging and perform comprehensive E2E testing across Performance, Leaderboard, and Agent Assist pages

3. **Agent-quintiles directionality integration**: Review and merge shared package PR #26430, then wire directionality into RetrieveQAScoreStats + RetrieveUserOutcomeStats

### Follow-ups Required

- **CONVI-6298**: Plan and execute prod rollout of ReindexScorecards (monitor for errors/latency for 24-48 hours)
- **Insights user filter**: Coordinate with platform team on fixing conversation routing to duplicate GUEST_USER accounts
- **Large-user-id-clickhouse**: Continue monitoring all prod clusters for errors and latency regressions

### Pending Reviews/Decisions

- PR #26451 (user-filter types and interface) - awaiting review
- PR #26430 (shared directionality package) - awaiting review
- PR #26431 (behavioral test suite) - awaiting review after comment response
