# Scorecard sync missing-rate dashboard assessment

Date: 2026-08-25

Source repo: `/Users/xuanyu.wang/repos/go-servers`

Branch/worktree context: `main` in `/Users/xuanyu.wang/repos/go-servers` (local checkout was 60 commits behind `origin/main` during this read-only assessment)

## Question

Can the existing `scorecard_sync_monitor` metrics support a dashboard that answers the average sync/missing status over the past 1, 2, or arbitrary number of days?

## Conclusion

Yes. The existing gauges are enough to build the missing-rate portion of the dashboard. They are emitted per `customer_id`, `profile_id`, and `cluster` for all, submitted, and unsubmitted scorecard cohorts:

- PG eligible totals;
- ClickHouse-present counts;
- missing counts;
- precomputed missing-rate percentages;
- whether a backfill was triggered.

The aggregate dashboard should derive missing rate from the count metrics:

`100 * sum(missing_count) / sum(total_count)`

This is a volume-weighted rate and answers what fraction of eligible scorecards were missing. A plain average of the emitted per-profile percentage gauges gives a tiny profile the same influence as a large profile and is not a reliable fleet-level status.

## Time semantics

- The production cron runs once daily at 10:00 UTC.
- With no explicit start/end overrides, each run checks the preceding rolling 24 hours.
- Therefore, a dashboard range of X days represents X daily observations/cohorts. A ratio of the missing and total observations across that range is a useful X-day weighted missing rate.
- This is not an on-demand monitor query whose underlying PG/CH inventory window changes to match the Grafana time picker. It is aggregation over stored daily results.
- Unfiltered manual runs emit into the same series and can add extra observations or overwrite the expected daily cadence. Template/use-case drill-down runs deliberately skip metric emission.

## Recommended dashboard contract

Primary definition: submitted-scorecard missing rate, because submitted scorecards are the customer-visible analytical cohort. Show the all-scorecard missing rate as the broader operational companion and unsubmitted rate as a diagnostic.

Recommended variables: cluster, customer, profile, and cohort (`submitted`, `all`, `unsubmitted`).

Recommended panels:

1. Selected-range weighted missing rate: ratio of summed missing-count observations to summed total-count observations.
2. Daily weighted missing-rate trend, grouped by cluster.
3. Latest missing rate and raw missing/total counts.
4. Customer/profile table with latest rate, selected-range weighted rate, worst daily rate, missing count, total count, and sample freshness.
5. Top customers/profiles by missing count; use missing count rather than rate alone so large-impact gaps rank appropriately.
6. Monitor execution/freshness panel using `cresta.cronjob.execute{cronjob_name="scorecard-sync-monitor"}` plus the latest timestamp of scorecard-monitor totals.
7. Optional backfill-trigger trend using `cresta.scorecard_sync_monitor.backfill_triggered`.

For Groundcover's Prometheus-compatible view, metric names will likely be normalized from dots to underscores (for example `cresta_scorecard_sync_monitor_missing_count_submitted`), but the exact ingested names and label spelling should be confirmed in Metrics Explorer before saving queries.

## Query shape

Conceptual selected-range query for the submitted cohort:

```promql
100 *
sum(sum_over_time(cresta_scorecard_sync_monitor_missing_count_submitted{cluster=~"$cluster",customer_id=~"$customer",profile_id=~"$profile"}[$__range]))
/
sum(sum_over_time(cresta_scorecard_sync_monitor_total_submitted{cluster=~"$cluster",customer_id=~"$customer",profile_id=~"$profile"}[$__range]))
```

Use the same count-ratio structure for per-cluster and per-customer views. Do not compute fleet status as `avg(missing_rate_percent_*)`.

The precise range/rollup expression may need adjustment based on how Groundcover exposes DogStatsD gauge samples. Verify that one logical cron observation is not repeated across multiple ingestion samples before using `sum_over_time`. If gauges are held/repeated, first roll up to one observation per daily run, or use the backend's weighted formula/rollup support.

## Reliability gaps

- Scorecard metrics are emitted only after a successful ClickHouse check. A failed profile has no fresh sample; absence must not be interpreted as 0% missing.
- Existing scorecard-specific metrics do not include an explicit run ID, observation-window start/end, success flag, or sample timestamp gauge.
- The shared cron completion metric covers job execution, but it does not by itself prove that every customer/profile emitted a current sample.
- The monitor detects absent scorecard rows and selected stale timestamps. Missing rate does not prove field-level or criterion-level correctness.

The first dashboard can ship without backend changes if it has a visible freshness/execution warning. For a stronger status/SLO dashboard, add per-profile `run_success`/`last_success_timestamp` and a cluster-level expected-versus-emitted profile count (or a run ID/window-end label with carefully controlled cardinality).

## Evidence reviewed

- `cron/task-runner/tasks/scorecard-sync-monitor/metrics.go`
- `cron/task-runner/tasks/scorecard-sync-monitor/task.go`
- `cron/task-runner/tasks/scorecard-sync-monitor/factory.go`
- `cron/task-runner/shared/metrics.go`
- `flux-deployments/apps/cron-task-runner/releases/03-prod-main/helmrelease-cron-scorecard-sync-monitor.yaml`
- existing scorecard sync domain and auto-heal monitoring notes

No production credentials or live telemetry were used.
