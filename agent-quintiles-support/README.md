# Agent Quintiles Support

**Created:** 2026-02-17
**Updated:** 2026-02-27

## Overview

Support dividing agents into **quintiles** (5 buckets) in addition to the existing **tiers** (3 buckets: top / average / bottom). The current API `RetrieveQAScoreStats` supports grouping by agent tier; we want to add quintiles and expose them on:

- **Performance page**: "Quintile Rank" column (number 1–5) in Leaderboard by criteria and Leaderboard per criteria tables (Agent tab). Gold/silver/bronze icons on agent names for Q1/Q2/Q3.
- **Leaderboard page**: Quintile column (after Live Assist) in Agent Leaderboard. Gold/silver/bronze icons on agent names in both Agent Leaderboard and Agent Leaderboard per metric tables.
- **Coaching Hub page**: Gold/silver/bronze icons on agent names in Recent Coaching Activities, with tooltip "Xth quintile based on last 7 days".
- **Coaching Plan page**: TBD.

Medium-sized project across BE (go-servers) and FE (director). Use worktrees when working in go-servers or director.

## Key Findings

### 1. Backend: RetrieveQAScoreStats and tiers

- **API**: `RetrieveQAScoreStats` (not "RetrieveQAStats") in **insights-server**.
- **Relevant files**:
  - `insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go` – main handler, tier aggregation.
  - `go-servers/shared/utils/partition.go` – `PartitionUsingVolumeAndMetric(slice, metric, volume, cutoffs)`.
- **Current tier logic**:
  - Three tiers: `BOTTOM_AGENTS`, `AVERAGE_AGENTS`, `TOP_AGENTS` (see `agentTiers` in `retrieve_qa_score_stats.go`).
  - Partition uses **volume-weighted cutoffs**: sort by score (ascending), then split by cumulative **conversation/scorecard volume** at cutoffs.
  - Cutoffs for 3 tiers: `[0.25, 0.75]` (i.e. `bottomAgentPercentage` and `bottomAgentPercentage + AvgAgentPercentage` from outcome_stats – 0.25, 0.5).
  - Code: `utils.PartitionUsingVolumeAndMetric(scoreSlice, scoreMetric, conversationVolumeMetric, []float32{0.25, 0.75})` → 3 partitions, then `createTieredScoreObject(partition, groupedBy, agentTiers[i])`.
- **Proto**: `cresta-proto/cresta/v1/analytics/qa_stats.proto` – `QAScoreGroupBy` has `AgentTier agent_tier = 6`. `outcome_stats.proto` defines `enum AgentTier { TOP_AGENTS=1, AVERAGE_AGENTS=2, BOTTOM_AGENTS=3 }`.
- **Quintile approach (BE)** – **TRUE PERCENTILE-BASED:** QA scores are absolute weighted averages (0–1), NOT percentiles. Fixed score bands (80+→Q1, etc.) fail when agents cluster in the same range. **Approach:** Rank all per-agent scores together (flat, no sub-grouping), sort descending, divide into 5 approximately equal groups: Q1 = top 20%, ..., Q5 = bottom 20%. Algorithm: `floor(N/5)` per quintile, first `N%5` get one extra. Only applied when grouped by `QA_ATTRIBUTE_TYPE_AGENT`.
  - Added `QuintileRank quintile_rank = 7` (OUTPUT_ONLY) to `QAScoreGroupBy` in proto.
  - `setQuintileRankForPerAgentScores(response)` collects all per-agent scores, sorts descending, distributes into quintiles.

### 2. Frontend: Detailed requirements and investigation

See **`requirements.md`** for product requirements and **`fe-investigation.md`** for detailed code investigation.

**Two display elements across all pages:**

1. **Quintile Rank column** — plain number 1–5, no prefix. Appears on Performance page tables (Agent tab) and Agent Leaderboard.
2. **Quintile icons on agent names** — gold (Q1), silver (Q2), bronze (Q3); no icon for Q4/Q5. Appears on all agent tables across Performance, Leaderboard, and Coaching Hub.

**Pages and tables:**

| Page | Table | Column | Icon on name |
|------|-------|--------|-------------|
| Performance | Leaderboard by criteria (2nd) – Agent tab | After "Average Performance", sticky | Yes + tooltip |
| Performance | Leaderboard per criteria (3rd) – Agent tab | Last sticky/static column | Yes + tooltip |
| Leaderboard | Agent Leaderboard | After "Live Assist" | Yes |
| Leaderboard | Agent Leaderboard per metric | — | Yes |
| Coaching Hub | Recent Coaching Activities | — | Yes + tooltip "Xth quintile based on last 7 days" |
| Coaching Plan | TBD | TBD | TBD |

**Shared component:** `QuintileRankIcon` — takes `quintileRank` + optional tooltip string. Gold/silver/bronze for Q1/Q2/Q3. Returns null for Q4/Q5/unspecified.

## Implementation plan

See **`implementation-plan.md`** for the concrete BE-first plan. Summary:

- **Quintile definition:** TRUE PERCENTILE-BASED. Rank all agents together, divide into 5 equal groups. Q1 = top 20% (best), Q5 = bottom 20%.
- **Phase 1 (BE):** Proto add `quintile_rank`; percentile-based `setQuintileRankForPerAgentScores` (flat ranking, sort descending, distribute into quintiles); both Postgres and ClickHouse paths; 10 tests. ✅ Done.
- **Phase 2 (FE):** Quintile Rank column on Performance + Leaderboard agent tables; gold/silver/bronze icons on agent names across Performance, Leaderboard, and Coaching Hub. Feature flag `enableQuintileRank` in config repo gates all UI. See `fe-investigation.md` for detailed plan.
- **Worktrees:** Use separate worktrees for go-servers and director when making changes.

## All PRs

### Backend & Infrastructure

| PR | Repo | Scope | Status |
|----|------|-------|--------|
| [cresta-proto #7874](https://github.com/cresta/cresta-proto/pull/7874) | cresta-proto | `quintile_rank` field in `QAScoreGroupBy` proto | Merged |
| [cresta-proto #7910](https://github.com/cresta/cresta-proto/pull/7910) | cresta-proto | `QuintileRank`/`QuintileRankNumber` in web-client export whitelist | Merged |
| [go-servers #25795](https://github.com/cresta/go-servers/pull/25795) | go-servers | `setQuintileRankForPerAgentScores` + `AssignRankGroups` utility | In review |
| [go-servers #26332](https://github.com/cresta/go-servers/pull/26332) | go-servers | CONVI-6219: UserOutcomeStats directionality fix (draft) | Closed (superseded by #26430 + combined PR) |
| [go-servers #26430](https://github.com/cresta/go-servers/pull/26430) | go-servers | CONVI-6219: Shared directionality helpers (`shared/scoring/directionality.go`) | Merged |
| [go-servers #26517](https://github.com/cresta/go-servers/pull/26517) | go-servers | CONVI-6219: Wire directionality into QAScoreStats + UserOutcomeStats (combined) | In review |
| [go-servers #26518](https://github.com/cresta/go-servers/pull/26518) | go-servers | CONVI-6219: Coaching service refactor to use shared helpers | In review |
| [config #140396](https://github.com/cresta/config/pull/140396) | config | `enableQuintileRank` feature flag | Merged |

### Frontend (director, stacked on Foundation)

| PR | Scope | Status |
|----|-------|--------|
| [Foundation #16883](https://github.com/cresta/director/pull/16883) | `QuintileRankIcon`, types, column visibility, i18n | Merged |
| [Move Icon #16911](https://github.com/cresta/director/pull/16911) | Move `QuintileRankIcon` to `director-components` | Merged |
| [Leaderboard #16884](https://github.com/cresta/director/pull/16884) | Agent Leaderboard (quintile column + icons) + Agent Leaderboard by Metric (icons) | Merged |
| [Performance #16886](https://github.com/cresta/director/pull/16886) | Leaderboard by criteria + Leaderboard per criteria (quintile column + icons) | Merged |
| [Coaching Hub #16887](https://github.com/cresta/director/pull/16887) | Recent Coaching Activities (trophy icons + i18n ordinal tooltip) | In review |
| [Coaching Plan #16905](https://github.com/cresta/director/pull/16905) | Coaching Plan header quintile rank badge (`QuintileRankBadge` self-contained component) | Merged |
| [Trophy Icon #17028](https://github.com/cresta/director/pull/17028) | Self-contained `QuintileRankIcon` with inline gradient defs, `IconTrophyFilled`, a11y | In review |
| [Align Coaching Plan #17263](https://github.com/cresta/director/pull/17263) | Align Coaching Plan quintile request with Coaching Hub defaults (CONVI-6389) | In review |

Demo branch `feature/agent-quintiles` ([PR #16849](https://github.com/cresta/director/pull/16849)) has all changes combined for stakeholder testing.

## Status

Near complete. **FE:** Foundation, Move Icon, Leaderboard, Performance, Coaching Plan all merged. Coaching Hub (#16887), Trophy Icon (#17028), Align Coaching Plan (#17263) in review. **BE:** Quintile PR (#25795) in review. Directionality foundation (#26430) merged. Directionality wiring (#26517) and coaching refactor (#26518) in review — CI green. Draft #26332 closed (superseded). **Remaining:** FE Coaching Hub + Trophy Icon + Align Coaching Plan + BE approval + merge.

## Log History

| Date       | Summary |
|-----------|---------|
| 2026-02-17 | Project created; investigation and plan documented. Quintiles defined as score bands: Q1 80+, Q2 60–79, Q3 40–59, Q4 20–39, Q5 0–19. Concrete BE implementation plan added (implementation-plan.md). |
| 2026-02-18 | Proto PR merged (cresta-proto #7874, v2.0.534). BE implemented: `ScoreToQuintileRank`, `setQuintileRankForPerAgentScores`, 14 tests. go-servers PR: [#25795](https://github.com/cresta/go-servers/pull/25795). Agent tier logic documented (`agent-tier-logic.md`). |
| 2026-02-19 | Requirements doc created. Deep FE investigation + feature flag investigation. PR validation: proto #7874 ✅, go-servers #25795 ⚠️ (missing ClickHouse path → fixed). **Quintile revised: score bands → true percentile-based.** Removed `ScoreToQuintileRank`; rewrote `setQuintileRankForPerAgentScores` as flat percentile ranking. 7 unit tests + 2 CH tests + 1 leakage test pass. |
| 2026-02-20 | Simplified BE: removed peer-group logic (flat ranking). Config flag PR merged ([#140396](https://github.com/cresta/config/pull/140396)). **FE PR 1**: Agent Leaderboard — quintile column (position, display, flag), `QuintileRankIcon` component, icons on names. **BE fix**: quintile rank leak into AGENT_TIER responses, `sort.SliceStable`, defense-in-depth clear in `createTieredScoreObject`. |
| 2026-02-23 | BE: Extracted `AssignRankGroups` utility with tie-aware boundaries. FE: all 4 PRs created — Foundation [#16883](https://github.com/cresta/director/pull/16883), Leaderboard [#16884](https://github.com/cresta/director/pull/16884), Performance [#16886](https://github.com/cresta/director/pull/16886), Coaching Hub [#16887](https://github.com/cresta/director/pull/16887). Demo branch pushed with mock data. |
| 2026-02-24 | FE: Foundation, Move Icon (#16911), Leaderboard merged. Coaching Plan PR #16905 created. Addressed all review comments on Performance (#16886) and Coaching Hub (#16887): i18n ordinal, useTranslation, gate quintile to Agent tab, loading gate, avoid mutation, sticky width fix, cell centering, guard undefined username. |
| 2026-02-25 | Coaching Plan #16905 merged (after extracting `QuintileRankBadge` as self-contained component per review). Performance #16886 merged. Coaching Hub #16887 rebased on latest main. Demo branch reset to main + coaching hub. |
| 2026-02-27 | Per-criterion quintile investigation — no changes needed, existing FE already handles it. Trophy Icon PR #17028: addressed all review comments (dismissed conflicting CodeRabbit suggestion, added `focusable="false"` a11y fix). |
| 2026-03-09 | Outcome quintile investigation — how outcomes (AHT, Conversion, CSAT) participate in QA score & quintile. `excludeFromQAScores` flag gates inclusion; `percentage_value` not always normalized (AHT = raw seconds). See `outcome-quintile-investigation.md`. |
| 2026-03-10 | Comprehensive per-page quintile reference — all 5 surfaces with exact request parameters, outcome handling, and cross-page consistency analysis. See `quintile-rank-behaviour-reference.md`. |
| 2026-03-11 | CONVI-6389: Aligned Coaching Plan quintile with Coaching Hub defaults. PR [#17263](https://github.com/cresta/director/pull/17263) — rebased, cleaned, extracted `useQuintileRankQAStats` hook per review. Reference doc reorganized + synced to Coda. CI green. |
| 2026-03-17 | CONVI-6219: Fixed directionality for TIME/CHURN outcome types in user outcome stats top agents partitioning. PR [#26332](https://github.com/cresta/go-servers/pull/26332) (draft). QA score quintile/tier directionality investigated — needs criterion→moment→outcome type lookup. See `convi-6219-investigation.md` and `convi-6219-qa-score-investigation.md`. |
| 2026-03-20 | CONVI-6219: Created shared directionality package `shared/scoring/directionality.go` — PR [#26430](https://github.com/cresta/go-servers/pull/26430). 5 exported helpers (`IsLowerBetterOutcomeType`, `IsLowerBetterOutcomeTypeEnum`, `ExtractCriterionOutcomeMomentResourceNames`, `GetOutcomeTypesFromMomentTemplates`, `BuildLowerIsBetterCriterionSet`) + 17 tests. Revised PR strategy: close draft #26332 (superseded); merge QA score + outcome stats fix into one PR; coaching refactor as separate PR. See `convi-6219-qa-score-impl-plan.md`. |
| 2026-03-24 | CONVI-6219: Wired directionality into both APIs. PR [#26517](https://github.com/cresta/go-servers/pull/26517) — QAScoreStats tier+quintile + UserOutcomeStats tier (4 files, +181/-31). PR [#26518](https://github.com/cresta/go-servers/pull/26518) — coaching refactor to shared helpers (1 file, +3/-85, pure refactor). |

## Related

- **Backend**: `~/repos/go-servers` (use worktree)
- **Frontend**: `~/repos/director` (use worktree)
- **Config**: `~/repos/config` — feature flag `enableQuintileRank` in `src/CustomerConfig.ts`
- **Proto**: cresta-proto (analytics qa_stats, outcome_stats)
- **Existing tier**: 3 tiers (top 25%, middle 50%, bottom 25% by volume-weighted score)
