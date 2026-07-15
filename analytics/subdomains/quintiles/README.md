# Quintiles

## Purpose

Own the five-bucket ranking model used to compare agents and present relative QA performance across analytics and coaching surfaces.

## Semantics and Invariants

- A quintile partitions an eligible ranked population into five ordered buckets; it is not an absolute score band.
- Population eligibility, time window, filters, metric direction, ties, and minimum volume are part of the result's meaning.
- `RetrieveQAScoreStats` and shared partition utilities are the backend foundation; Performance Insights, Leaderboard, and coaching may present the same result differently.
- A quintile must not be interpreted without the population and window that produced it.

## Architecture and Source Map

- **Frontend:** Performance Insights, Leaderboard, and coaching consumers
- **APIs:** primarily `RetrieveQAScoreStats`
- **Backend:** analytics-service ranking and partition utilities
- **Storage:** scorecard-score analytics rows in ClickHouse

## Operational Knowledge

- Diagnose unexpected placement by reproducing the full eligible population before inspecting bucket boundaries.
- Check tie handling, low-volume agents, missing scores, filter expansion, and ranking direction.

## Legacy Sources and Cases

- `agent-quintiles-support/`
- `agent-stats-analytics-behaviors/`

## Open Questions

- Document the exact tie and remainder allocation algorithm.
- Define minimum-volume and incomplete-window behavior for each consumer.
