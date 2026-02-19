# PR for Phase 1.1 – Proto: add quintile_rank to QAScoreGroupBy

## Worktree

- **Path:** `../cresta-proto-quintiles` (from cresta-proto repo root: `/Users/xuanyu.wang/repos/cresta-proto-quintiles`)
- **Branch:** `feature/agent-quintiles-proto` (from `origin/main`)

The proto change is already applied in this worktree.

## What's in the change

- **File:** `cresta/v1/analytics/qa_stats.proto`
- **Changes:**
  1. Added `enum QuintileRank` with values `QUINTILE_RANK_UNSPECIFIED = 0`, `QUINTILE_RANK_1 = 1` through `QUINTILE_RANK_5 = 5`. Score bands: 1 = 80+, 2 = 60–79, 3 = 40–59, 4 = 20–39, 5 = 0–19.
  2. In `message QAScoreGroupBy`, added:
     - `QuintileRank quintile_rank = 7 [(google.api.field_behavior) = OUTPUT_ONLY];`
     - Only set when response is grouped by agent.

## Before opening the PR

1. **Regenerate generated code** (from the worktree directory):
   ```bash
   cd /Users/xuanyu.wang/repos/cresta-proto-quintiles
   mage generate          # Python + TypeScript
   mage generateGoProto && mage generateServiceMock   # Go
   ```
2. **Commit** proto + any updated files under `gen/`:
   ```bash
   git add cresta/v1/analytics/qa_stats.proto gen/
   git status   # confirm only intended files
   git commit -m "analytics: add quintile_rank to QAScoreGroupBy for agent quintiles (Phase 1.1)"
   ```
3. **Push** and open PR:
   ```bash
   git push -u origin feature/agent-quintiles-proto
   ```

## Suggested PR title and description

**Title:** `analytics: add quintile_rank to QAScoreGroupBy for agent quintiles`

**Description:**
- Adds `enum QuintileRank` (QUINTILE_RANK_1 through QUINTILE_RANK_5) and `QuintileRank quintile_rank` field to `QAScoreGroupBy` in `cresta/v1/analytics/qa_stats.proto`.
- Used to expose agent quintile by QA score band: 1 = 80+, 2 = 60–79, 3 = 40–59, 4 = 20–39, 5 = 0–19. Only set when RetrieveQAScoreStats response is grouped by agent.
- Part of agent quintiles support (Phase 1.1). BE will populate this in a follow-up (go-servers).
