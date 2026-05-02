# Weekly Summary - Week of 2026-04-26

**Created:** 2026-05-02

## Progresses

### bswift-wrong-team-mapping
- Fixed historical conversation data for 2 of 3 affected agents (Briana Joyner: 1,752 rows, Tennisha Cox: 418 rows) in both PostgreSQL and ClickHouse
- PG fix: updated `agent_user_id` and `team_group_id` from GUEST to PROD user IDs
- CH fix: deleted stale GUEST rows, ran backfill jobs (Tennisha: 1 job, Briana: 12 jobs across 6 months)
- Verified zero conversations remaining under GUEST user IDs for both agents
- Investigated Brenda O'Neal — found her GUEST user is actually the active/correct account, no migration needed
- Status: **Briana & Tennisha complete**; Brenda pending decision on whether to consolidate

### convi-6672-achieve-behavior-na
- Investigated why 5 of 6 "Intro Trigger" behavior criteria show N/A in Performance Insights for Achieve's Welcome Call use case
- Root cause identified: `excludeFromQAScores=true` on criteria causes `computeScore()` to return nil, producing `percentage_value=-1` and `float_weight=1e-13` in ClickHouse
- Traced the full code path in `go-servers` and documented data evidence from ClickHouse
- Status: **investigation complete**, pending PG template verification to confirm template ID

### nascore (N/A Score Support)
- Corrected `options-scores-lifecycle.md` — reclassified decoupled mode as intentional design (not a pain point), updated canonical invariants and cross-validation table
- Created `na-score-lifecycle.md` — complete end-to-end lifecycle trace of the N/A score feature
- Reviewed PR #18318 for correctness — found 3 issues (P2: misleading validation error messages, P3: wrong dependency direction, P3: N/A appearing in detected/not_detected dropdowns)
- Status: PR review findings documented

### Blog Writing
- Wrote "Driving a Performance Car on an Unfamiliar Road" — retrospective on AI productivity arc (useless → powerful → lost control) using the N/A score project as a case study
- Status: draft complete, uncommitted

## Problems

### Technical Issues
- bswift Brenda O'Neal has a reversed situation (GUEST is the real account) — requires product/CSM decision on whether to consolidate, not a pure engineering fix

### Learnings from Failures
- N/A score project retrospective (blog post): recognized the pattern of AI-assisted productivity leading to loss of understanding — producing more while comprehending less, especially in unfamiliar codebases
- Key insight: AI tools amplify whatever the developer brings — deep understanding gets amplified into fast, correct output; shallow understanding gets amplified into fast, incorrect output

## Plan

### Next Week Priorities
1. Verify CONVI-6672 template ID against PG and coordinate fix (set `excludeFromQAScores=false` or toggle "Evaluate scores" ON)
2. Follow up on bswift Brenda O'Neal decision
3. Continue nascore PR #18318 review cycle if updates come in

### Follow-ups Required
- Brenda O'Neal: get CSM/product input on whether to consolidate GUEST → PROD user
- CONVI-6672: need PG access to verify template `019c203e-1bba-71be-9ae7-e093ef0c80b7` is "Technical insights Scorecard"

### Pending Reviews/Decisions
- Blog post: finalize and publish "AI driving unfamiliar roads" post
- nascore PR #18318: P2 validation message issue needs addressing
