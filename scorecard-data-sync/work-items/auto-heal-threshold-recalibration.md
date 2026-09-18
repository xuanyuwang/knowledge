# Auto-Heal Threshold Recalibration

**Status:** active
**Primary domain:** `scorecard-data-sync`
**Primary subdomain:** none
**Official ticket:** none
**Last updated:** 2026-09-11

## Objective and Impact

- **Objective:** Recalibrate the per-profile auto-heal ceiling from observed repair volume, workflow completion, and post-repair convergence instead of retaining the original `1000` rollout gate by default.
- **Customer/system impact:** A better ceiling reduces routine manual repair while preserving protection against incident-scale dispatch storms and false repair success during ClickHouse delivery failures.
- **Role:** diagnosed

## Scope

**In scope**

- Historical `#scorecard-sync-monitor` dispatch and threshold-skip evidence.
- Existing Temporal completion and post-repair verification evidence.
- Candidate-volume coverage at alternative thresholds.
- A staged recommendation and required guardrails.

**Non-goals**

- Changing production configuration or product code.
- Claiming exact p50/p95 workflow latency without Temporal execution data.
- Treating Temporal completion as proof that repaired rows are queryable in ClickHouse.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/knowledge`, `/Users/xuanyu.wang/repos/go-servers`
- **Worktrees:** knowledge main checkout; go-servers main checkout (read-only, 369 commits behind `origin/main` at inspection time)
- **Branches:** knowledge current branch; go-servers `main`
- **PRs/commits:** existing scorecard-sync-monitor implementation through local commit `517e94614c`; no new code change

## Current Understanding

The `1000` ceiling is still the locally documented default, while each accepted profile is already split into jobs of at most `5000` scorecards. It was introduced as a rollout blast-radius gate, not derived from measured workflow capacity.

The original Slack-only sample understated current coverage because Glean exposed only three dispatch-era summaries. Groundcover fills the gap. From 2026-06-20 through 2026-09-11, 1,403 profile-day observations had a nonzero missing count: 67.9% were at or below 1,000, 82.5% at or below 5,000, 87.2% at or below 10,000, and 92.4% at or below 25,000. This metric excludes stale candidates, so the exact recent action distribution was recovered from monitor logs.

Across the most recent 14 complete daily runs, the monitor dispatched 492 workflows for 27,821 candidates and skipped 63 profile runs containing 1,178,791 candidates. The current 1,000 ceiling already handled 88.6% of profile actions. A 5,000 ceiling would have handled 94.1% while adding only 30 profile runs and 80,982 candidates over 14 days; the busiest day would add seven workflows for 24,023 candidates. This is the clearest current elbow and aligns the per-profile ceiling with the existing one-workflow chunk size.

Recommendation: raise the immediate per-profile ceiling to `5000`, retaining the `5000` chunk size. Consider `10000` only after observing the 5,000 rollout. Keep `25000` as an incident/manual ceiling until a cluster-wide candidate or concurrency budget exists.

## Findings and Decisions

- Accessible Slack evidence is sparse: Glean returned daily summaries for 2026-06-23, 2026-06-26, 2026-07-29, and 2026-08-07 only. The June 23 post had no dispatched workflow links, so the threshold sample uses the other three dates.
- Groundcover monitor metrics cover 84 daily runs from 2026-06-20 through 2026-09-11 and approximately 353-398 active profiles per run.
- Among 1,403 nonzero missing-count profile-days, missing-count coverage was:

  | Ceiling | Profile-days covered | Coverage |
  | ---: | ---: | ---: |
  | 1,000 | 952/1,403 | 67.9% |
  | 5,000 | 1,157/1,403 | 82.5% |
  | 10,000 | 1,224/1,403 | 87.2% |
  | 25,000 | 1,296/1,403 | 92.4% |
  | 50,000 | 1,331/1,403 | 94.9% |

- Because missing-count gauges omit stale candidates, recent monitor logs provide the exact dispatch-or-skip distribution for the last 14 days:

  | Ceiling | Profile actions handled | Case coverage | Candidate mass handled |
  | ---: | ---: | ---: | ---: |
  | 1,000 | 492/555 | 88.6% | 27,821 / 1,206,612 (2.3%) |
  | 5,000 | 522/555 | 94.1% | 108,803 / 1,206,612 (9.0%) |
  | 10,000 | 528/555 | 95.1% | 158,802 / 1,206,612 (13.2%) |
  | 25,000 | 545/555 | 98.2% | 472,111 / 1,206,612 (39.1%) |
  | 50,000 | 550/555 | 99.1% | 643,232 / 1,206,612 (53.3%) |

- Candidate mass is extremely concentrated in a few large outliers. Raising the threshold to automate most cases is not equivalent to automatically processing most scorecards; doing the latter would intentionally admit incident-scale profiles.
- At 5,000, the last 14 days would have added 30 workflows and 80,982 candidates. The largest single-day increment was seven workflows / 24,023 candidates. At 25,000, the increment grows to 53 profile runs and 444,290 candidates.
- The Slack-only sample remains useful historical evidence but is no longer the primary sizing dataset: it contained 84 profile actions and overrepresented the June/July incident period.
- On 2026-08-03, a deliberate production run used a `200000` per-profile ceiling and `5000` chunks. It dispatched 57,773 candidates through 47 workflows, including 11 CNG chunks; all workflows completed. The auto-heal job timestamp was 13:49:19Z and verification started at 13:58:49Z, bounding monitor execution, dispatch, workflow completion, and handoff to verification to under 9 minutes 30 seconds.
- Post-run verification reported 26 missing and 3 stale rows. The remaining 26 submitted Alaska Air rows were not recreated by their successful workflow, so the observed convergence was high but not universal.
- A separate 124-scorecard Guitar Center workflow completed in 5.85 seconds while 0/124 were immediately visible because writes entered a Distributed-table backlog. Workflow duration is therefore not sufficient as the auto-heal success metric.
- The staging override already demonstrated dispatch above the current ceiling: 1,122 candidates launched successfully with a manual `5000` override.
- Groundcover's `reindex_scorecards` Temporal queue recorded 2,519 completed and 2 failed workflows over the 83-day window: 99.92% terminal success, with no failures in the last 30 days.
- Full-window Temporal latency was 1.92 seconds p50, 8.35 seconds p95, and 44.23 seconds p99; mean 3.22 seconds. Over the last 30 days it was 2.42 seconds p50, 8.55 seconds p95, and 30.94 seconds p99; mean 2.86 seconds.
- For triggered profiles with a nonzero prior-day missing count, the next daily monitor showed 51.7% fully clear and 71.3% improved. This is only a rolling-window proxy: it cannot prove repair of the same scorecard IDs because the 24-hour population changes between runs.

## Blockers and Dependencies

- Temporal queue metrics provide aggregate duration and terminal status, but they do not carry workflow ID or candidate count, so duration cannot be correlated with payload size per workflow.
- Monitor metrics do not emit stale count or exact reindex-candidate count. Recent logs fill this gap, but a durable candidate-count metric is still needed.
- The monitor emits only a pre-repair gauge. The next daily run is too late and uses a different rolling cohort, so exact time-to-ClickHouse-convergence remains unknown.
- Before a `25000` automatic default, add or verify a cluster-wide dispatch budget/concurrency cap so medium profiles cannot create an unbounded regional repair wave.

## Validation and Rollout

- First change: raise to `5000`; retain `5000` chunks, auto-heal allowlisting, and existing drill-down guards.
- Record for each created job: candidate count, queued/start/close times, status, workflow duration, and time until a follow-up monitor confirms convergence.
- Track separately: Temporal success rate, ClickHouse convergence rate, p50/p95 workflow duration, p50/p95 convergence duration, residual count, and retry count.
- Reassess after at least two weeks. Raise toward `10000` only if cluster load and convergence stay within an agreed budget; retain higher thresholds for explicit incident repair until a regional cap exists.

## Next Actions

1. Confirm whether `5000` is acceptable as the immediate ceiling.
2. Add exact Temporal and convergence telemetry to make the next threshold capacity-based.
3. Add exact stale and total reindex-candidate gauges.
4. Add or verify a cluster-wide candidate/workflow cap before considering `25000` or higher.

## Timeline

- 2026-09-11 — Reconstructed the accessible Slack action sample and initially recommended 25,000; then added 84 days of Groundcover metrics and 14 days of exact candidate logs, measured 99.92% Temporal terminal success, and revised the immediate recommendation to 5,000. Evidence: `../sessions/2026-09-11/codex-auto-heal-threshold-recalibration.md`, `../log/2026-09-11.md`.
