# Auto-Heal Threshold Recalibration Investigation

## Context

- **Date/timezone:** 2026-09-11, America/Toronto
- **Primary domain:** `scorecard-data-sync`
- **Source repo:** `/Users/xuanyu.wang/repos/go-servers`
- **Source context:** local `main`, read-only; 369 commits behind `origin/main` when inspected
- **Knowledge checkout:** `/Users/xuanyu.wang/repos/knowledge`, main checkout
- **Question:** Does the current per-profile threshold of 1,000 candidates still make sense, and what higher ceiling would automatically repair most observed cases?

## Sources Reviewed

- `auto-backfill-missing-scorecards/implementation-and-validation-summary.md`
- `scorecard-data-sync/sessions/2026-08-03/codex-post-recovery-missing-rate.md`
- `scorecard-data-sync/sessions/2026-08-01/codex-performance-insights-missing-data.md`
- Current local scorecard-sync-monitor factory, task, and README.
- Glean search/chat results for Slack channel `#scorecard-sync-monitor` (`C0ABYUBEYSG`).
- Accessible Slack summaries:
  - [2026-06-23](https://crestalabs.slack.com/archives/C0ABYUBEYSG/p1782209637911829)
  - [2026-06-26](https://crestalabs.slack.com/archives/C0ABYUBEYSG/p1782469321174399)
  - [2026-07-29](https://crestalabs.slack.com/archives/C0ABYUBEYSG/p1785319796704249)
  - [2026-08-07](https://crestalabs.slack.com/archives/C0ABYUBEYSG/p1786098340369459)

## Implementation Baseline

- `SCORECARD_SYNC_MONITOR_MAX_AUTO_HEAL_SCORECARDS` defaults to `1000` per customer/profile.
- `SCORECARD_SYNC_MONITOR_MAX_SCORECARDS_PER_REINDEX_JOB` defaults to `5000`.
- Candidate sets accepted by the threshold are chunked after the threshold check.
- The ceiling is therefore a per-profile blast-radius gate; the 5,000 value is the per-workflow payload/processing boundary.

## Preliminary Accessible Slack Dataset

Glean exposed only four daily summary messages. The June 23 summary had no workflow dispatch links. The quantitative action sample therefore uses June 26, July 29, and August 7.

| Date | Dispatched | Skipped above 1k | Total actions |
| --- | ---: | ---: | ---: |
| 2026-06-26 | 6 | 23 | 29 |
| 2026-07-29 | 26 | 20 | 46 |
| 2026-08-07 | 9 | 0 | 9 |
| **Total** | **41** | **43** | **84** |

Candidate counts are explicit for skipped cases. Older Slack formatting often showed missing count rather than total candidates for dispatched cases; those cases are known only to have passed the 1,000-candidate gate.

Skipped-count distribution: minimum 1,045; p25 2,504; median 8,826; p75 33,683; p90 68,459; maximum 1,047,604.

## Preliminary Slack-Only Threshold Coverage

This first pass was based on a sparse, incident-heavy sample and led to an initial 25,000 recommendation. The Groundcover evidence below supersedes it for the immediate threshold decision.

| Ceiling | Added skipped cases admitted | Total cases handled | Case coverage | Max 5k chunks/profile |
| ---: | ---: | ---: | ---: | ---: |
| 1,000 | 0 | 41/84 | 48.8% | 1 |
| 2,000 | 8 | 49/84 | 58.3% | 1 |
| 5,000 | 16 | 57/84 | 67.9% | 1 |
| 10,000 | 22 | 63/84 | 75.0% | 2 |
| 25,000 | 29 | 70/84 | 83.3% | 5 |
| 50,000 | 35 | 76/84 | 90.5% | 10 |
| 100,000 | 38 | 79/84 | 94.0% | 20 |
| 200,000 | 40 | 81/84 | 96.4% | 40 |

Per-day coverage:

| Ceiling | June 26 | July 29 | August 7 |
| ---: | ---: | ---: | ---: |
| 1,000 | 20.7% | 56.5% | 100% |
| 25,000 | 65.5% | 91.3% | 100% |
| 50,000 | 82.8% | 93.5% | 100% |

Incremental candidate volume among previously skipped cases:

| Ceiling | June 26 | July 29 |
| ---: | ---: | ---: |
| 25,000 | 104,896 across 13 profiles; 29 jobs | 72,054 across 16 profiles; 24 jobs |
| 50,000 | 279,103 across 18 profiles; 65 jobs | 97,056 across 17 profiles; 30 jobs |

This excludes candidates from already-dispatched profiles because Slack did not expose their total candidate count consistently.

## Groundcover Historical Metrics

Source: [scorecard sync dashboard](https://app.groundcover.com/dashboards/74e66e76-2c48-40ce-9cc0-3d8c87742eb7), workspace `58661bf1-f4e0-42b4-8a54-0ca419eb58cb`, backend `groundcover`.

### Missing-count profile distribution

Daily gauges were sampled from the scheduled monitor interval for 84 runs, 2026-06-20 through 2026-09-11. Active profile count grew from approximately 353 to 398.

There were 1,403 profile-day observations with a nonzero `missing_count_all`:

| Missing-count ceiling | Profile-days covered | Coverage |
| ---: | ---: | ---: |
| 1,000 | 952 | 67.9% |
| 5,000 | 1,157 | 82.5% |
| 10,000 | 1,224 | 87.2% |
| 25,000 | 1,296 | 92.4% |
| 50,000 | 1,331 | 94.9% |

The gauge does not include stale candidates, so it cannot reproduce the actual threshold decision by itself.

### Exact recent candidate actions

Groundcover monitor logs include `process_count`, `conversation_count`, and exact over-threshold candidate counts. Across the 14 daily runs from August 29 through September 11:

- 492 workflows were dispatched for 27,821 candidates.
- 63 profile runs were skipped, containing 1,178,791 candidates.
- Exact case coverage by simulated ceiling:

  | Ceiling | Actions handled | Case coverage | Candidate mass handled |
  | ---: | ---: | ---: | ---: |
  | 1,000 | 492/555 | 88.6% | 27,821 / 1,206,612 (2.3%) |
  | 5,000 | 522/555 | 94.1% | 108,803 / 1,206,612 (9.0%) |
  | 10,000 | 528/555 | 95.1% | 158,802 / 1,206,612 (13.2%) |
  | 25,000 | 545/555 | 98.2% | 472,111 / 1,206,612 (39.1%) |
  | 50,000 | 550/555 | 99.1% | 643,232 / 1,206,612 (53.3%) |

At 5,000, the increment is 30 workflows / 80,982 candidates over 14 days. The busiest day adds seven workflows / 24,023 candidates. At 25,000, the increment is 53 profile actions / 444,290 candidates.

Candidate mass is concentrated in a few large profiles: optimizing for most profile cases is intentionally different from auto-processing most scorecards.

### Temporal terminal status and latency

The `reindex_scorecards` task queue exposes Temporal SDK closure and end-to-end-latency metrics:

| Window | Closed workflows | Completed | Failed | p50 | p95 | p99 | Mean |
| --- | ---: | ---: | ---: | ---: | ---: | ---: | ---: |
| 83 days | 2,521 | 2,519 | 2 | 1.92s | 8.35s | 44.23s | 3.22s |
| Last 30 days | 1,096 | 1,096 | 0 | 2.42s | 8.55s | 30.94s | 2.86s |
| Last 14 days | 503 | 503 | 0 | 2.33s | 8.36s | 33.23s | 2.85s |
| Last 7 days | 268 | 268 | 0 | 2.39s | 7.90s | 28.56s | 2.83s |

This establishes that workflow execution is fast and reliable at the current payload range. The histogram lacks workflow ID and candidate-count labels, so it cannot prove how latency scales with payload size.

### Next-run convergence proxy

For 849 profile-days that both triggered a workflow with nonzero missing rows and had a matching next-day profile observation:

- 439/849 (51.7%) were fully clear on the next daily run.
- 605/849 (71.3%) had a lower missing count.
- 410/849 (48.3%) still had a nonzero missing count.

This cannot be interpreted as a workflow failure rate. Each scheduled run examines a moving 24-hour window, so the next day's missing rows may be different scorecards. Exact time-to-convergence requires candidate IDs or a post-workflow verification metric.

## Completion and Convergence Evidence

### August 3 controlled production run

- Ceiling: 200,000 per profile.
- Chunk size: 5,000.
- 57,773 unique candidates: 56,700 missing and 1,073 stale.
- 47 workflows across 37 profiles; CNG contributed 54,844 candidates in 11 chunks.
- All 47 Temporal workflows completed successfully.
- Auto-heal job suffix `1785764959` resolves to `2026-08-03T13:49:19Z`.
- Verification job suffix `1785765529` resolves to `2026-08-03T13:58:49Z`.
- Therefore the whole monitor/dispatch/workflow-completion/handoff interval was under 9 minutes 30 seconds. This is an upper bound, not per-workflow latency.
- Verification: 26 missing and 3 stale remained. The 26 missing submitted Alaska Air scorecards survived a successful workflow and required targeted diagnosis.

### August 1 backlog counterexample

- One 124-scorecard Guitar Center workflow completed successfully in 5.85 seconds.
- None of the 124 rows was immediately queryable because the writes joined the existing ClickHouse Distributed backlog.
- Temporal completion measures application-path acceptance, not end-to-end repair convergence.

### Voice-staging threshold override

- 748 candidates under the 1,000 default launched successfully.
- 1,122 candidates were skipped at 1,000, then launched successfully with a 5,000 manual override.
- These results prove that 1,000 is not a hard workflow-capacity boundary.

## Recommendation

Raise the immediate ceiling to `5000` and retain 5,000-scorecard chunks.

This moves recent exact case coverage from 88.6% to 94.1%, keeps every accepted profile in one workflow, and adds at most seven workflows / 24,023 candidates on the busiest observed day. Moving directly to 25,000 improves recent case coverage by only another 4.1 points while admitting 363,308 more candidates across the sample.

Observe 5,000 for at least two weeks, add exact candidate-to-convergence telemetry, then consider 10,000. Keep 25,000 and higher as explicit incident/manual ceilings until a regional candidate or concurrency budget exists.

## Evidence Limitations

- Glean did not return daily summaries after August 7; Groundcover supplied the missing historical metric and recent log coverage.
- Monitor gauges omit stale and total candidate counts. Exact candidate history currently requires log extraction.
- Temporal metrics are aggregated by task queue and do not include workflow ID or payload size.
- The monitor emits no immediate post-workflow verification, so exact ClickHouse convergence time remains unavailable.
- The Temporal UI required SSO. No restricted credential was used.
- The local go-servers checkout was behind origin; Groundcover skip logs independently confirmed that production still used the 1,000 threshold during the recent 14-day sample.

## Credentials

No AWS, Okta, Azure, or SSH credential was used.
