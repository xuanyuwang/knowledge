# Weekly Summary - Week of 2026-03-23

**Created:** 2026-03-30

## Progresses

### Active Days Investigation (CONVI-6423)

- Root cause found: Low Desktop app adoption (88-90% of agents not using it), not heartbeat detection failures
- Active Days is working as designed — it measures Desktop app usage, not general conversation activity
- Recommendations: investigate adoption, deploy to Desktop app, consider alternative metrics

### Agent Quintiles Support (CONVI-6219)

- PR #26430 (shared directionality helpers) merged
- Wired directionality into QAScoreStats + UserOutcomeStats (PR #26517)
- Discovered mixed-criteria aggregation issue: CH query aggregates across all criteria with no criterion_id in result
- Reworked approach: added `ranking_weighted_percentage_sum` column with conditional negation for lower-is-better criteria via ext table
- Split UserOutcomeStats into separate PR #26616
- All tests passing

### Scorecard Template Deep Dive

- Mapped full template structure: data model, scoring algorithm, criterion options, N/A handling flow
- Designed `scoreableNA` feature: N/A behaves like regular scored option with fixed label, zero scoring pipeline changes
- Revised scoring approach: moved NAScore handling to calculation layer (`ComputeCriterionPercentageScore`) instead of post-processing to avoid persisting modified N/A flags
- Confirmed no ClickHouse schema changes needed
- Updated Coda design doc with FE requirements and CH schema investigation

### Backfill Process Scorecards (CONVI-6298)

- Tested on cresta-sandbox-2 (voice-prod): 12 scorecards + 114 scores written to CH successfully
- Created `backfill-all-customers.sh` for all 8 prod clusters with `clean_up_before_write=true`
- Phase 1 (2026 data): 265/276 completed across 7 clusters
- Phase 2 (pre-2026): 275/276 completed
- Remaining: schwab-prod needs image update, brinks-care has duplicate criterion data quality issue

### Live Assist Ext Table Regression (CONVI-6476)

- Root cause: nil arg appended when ext table enabled, shifting positional args in CH query
- Regression from CONVI-6316 combined with ENABLE_EXT_TABLE_FOR_USER_FILTER
- Fix: guard append with `if c.arg != nil`. PR #26519 merged

### Raises Answered Bug Discovery (CONVI-6494)

- Found while verifying CONVI-6476: "Raises Answered %" always 0% for all customers
- Root cause: `GROUP BY manager_user_id` splits raise hand events (empty manager) from whispers (non-empty manager)
- Fix verified on CH: split into two CTEs. CONVI-6494 ticket filed

### CONVI-6247 Agent-Only Filter

- Changed default from `true` to `false` (opt-in, matching "Exclude deactivated users" pattern)
- Filter only appears on filter bar when toggled to `true`

### Virtual Group Filter (CONVI-6174)

- Discovered current behavior is intentional: virtual groups for filtering, TEAM groups for aggregation
- Key question raised: is this a bug fix or feature request?

### Team Enable / Claude Code Plugin

- Created shared `cleanup-feature-flag` skill in marketplace (PR #34)
- Used skill to identify 17 stale feature flags across codebase

## Problems

### Technical Issues

- Backfill heartbeat timeouts on us-east-1-prod: 90 concurrent workflows overwhelmed activity workers — resolved by re-triggering
- schwab-prod running old image, could not run backfill — pending image update
- brinks-care duplicate criterion data quality issue — needs decision on cleanup

### Learnings

- Ext table nil arg pattern: when conditionally adding ext tables, guard against nil to prevent positional arg shift
- Mixed-criteria aggregation in quintiles needs special handling at CH query level, not post-processing
- N/A score design: keep mutations in post-processing layer, calculations in calculation layer — don't mix concerns

## Plan

### Next Week Priorities

1. Push and finalize agent quintiles PRs (#26517, #26616)
2. Update schwab-prod image and re-trigger scorecard backfill
3. Decide on brinks-care data quality issue resolution
4. NAScore design review and implementation kickoff

### Follow-ups Required

- Virtual group filter: get PM clarification on bug vs feature
- CONVI-6494 (Raises Answered 0%): create fix PR
- CONVI-6476: verify fix on voice-staging after deploy

### Pending Reviews/Decisions

- Agent quintiles PR #26517 rework review
- Claude Code marketplace PR #34 adoption tracking
- NAScore design approval from team

