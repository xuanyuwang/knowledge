# Agent Tier Logic Investigation

**Created:** 2026-02-18
**Updated:** 2026-02-18

## Overview

Agent tiers (TOP_AGENTS, AVERAGE_AGENTS, BOTTOM_AGENTS) are determined by **volume-weighted partitioning** — tiers represent percentages of total conversation volume, not agent count.

## Constants

**File:** `go-servers/insights-server/internal/analyticsimpl/retrieve_conversation_outcome_stats.go`

```go
const (
    topAgentPercentage    = 0.25  // 25%
    AvgAgentPercentage    = 0.5   // 50%
    bottomAgentPercentage = 0.25  // 25%
)
```

| Tier | Volume Share | Score Rank |
|------|-------------|------------|
| BOTTOM_AGENTS | 0–25% cumulative volume | Lowest QA scores |
| AVERAGE_AGENTS | 25–75% cumulative volume | Middle QA scores |
| TOP_AGENTS | 75–100% cumulative volume | Highest QA scores |

## Algorithm: `PartitionUsingVolumeAndMetric`

**File:** `go-servers/shared/utils/partition.go`

### Step 1: Two-level sort (ascending)

```go
sort.Slice(slice, func(i, j int) bool {
    return metric(slice[i]) < metric(slice[j]) ||
        (metric(slice[i]) == metric(slice[j]) && volume(slice[i]) < volume(slice[j]))
})
```

- **Primary:** QA score ascending (worst performers first)
- **Secondary:** Volume ascending (for stability when scores are equal)

### Step 2: Cumulative volume prefix sum

```go
cumulativeVolume := make([]int32, len(slice)+1)
for i, elem := range slice {
    cumulativeVolume[i+1] = volume(elem) + cumulativeVolume[i]
}
```

### Step 3: Binary search for cutoff indices

Cutoffs `[0.25, 0.75]` → absolute volume thresholds → binary search for first index where cumulative volume meets/exceeds threshold.

### Step 4: Ensure non-empty partitions

`ensureNonEmptySliceIndices` guarantees each partition has ≥1 element by adjusting boundaries.

## QA Score Stats Usage

**File:** `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats.go`

```go
func aggregateTopAgentsResponse(response) {
    scoreMetric := func(qaScore) float32 { return qaScore.Score }
    volumeMetric := func(qaScore) int32 { return qaScore.TotalScorecardCount }

    agentPartitions := utils.PartitionUsingVolumeAndMetric(
        scoreSlice, scoreMetric, volumeMetric,
        []float32{0.25, 0.75},  // [bottomAgentPercentage, bottom+avg]
    )
    // partitions[0] → BOTTOM_AGENTS
    // partitions[1] → AVERAGE_AGENTS
    // partitions[2] → TOP_AGENTS
}
```

## Worked Example

```
3 agents:
  Agent A: Score 0.5, Volume 100 conversations
  Agent B: Score 0.7, Volume 100 conversations
  Agent C: Score 0.9, Volume 200 conversations
  Total Volume: 400

Sorted ascending by score:
  1. Agent A (0.5): cumulative = 100
  2. Agent B (0.7): cumulative = 200
  3. Agent C (0.9): cumulative = 400

Cutoffs:
  25% of 400 = 100 → index after Agent A
  75% of 400 = 300 → index after Agent B

Result:
  BOTTOM_AGENTS:  [Agent A]      (score 0.5)
  AVERAGE_AGENTS: [Agent B]      (score 0.7)
  TOP_AGENTS:     [Agent C]      (score 0.9)
```

## Key Characteristics

- **Volume-weighted, not count-based:** An agent handling many conversations shifts tier boundaries more than one handling few.
- **Metric = QA score:** Lower score = worse performer = bottom tier.
- **Volume = TotalScorecardCount:** Number of scored conversations per agent.
- **Deterministic:** Secondary sort by volume ensures reproducible results.
- **Edge-case safe:** Always produces exactly 3 non-empty partitions.

## Comparison: Conversation Outcome Stats

Uses the same 25/50/25 percentages but sorts by **behavior adherence descending** (best first), so partition mapping is reversed:
- partition[0] → TOP_AGENTS
- partition[1] → AVERAGE_AGENTS
- partition[2] → BOTTOM_AGENTS

## Relevance to Quintile Rank

Quintile rank and agent tiers serve different purposes:
- **Agent tiers:** Relative ranking within the current result set, volume-weighted, 3 buckets.
- **Quintile rank:** Absolute score bands (0.8+, 0.6–0.8, 0.4–0.6, 0.2–0.4, 0–0.2), 5 buckets, per-agent only.

They are independent: an agent can be in the TOP tier (relative) but QUINTILE_RANK_3 (absolute score 0.4–0.6) if all agents score low.
