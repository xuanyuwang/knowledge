# Scorecard sync monitor stale reporting PR

Date: 2026-08-07

## Context

Weekly note: Snap Finance Slack showed `0 missing` while auto-heal reindexed 3 scorecards. Root cause: Slack/cluster summary reported missing only; auto-heal also heals stale scorecards (present in CH with mismatched submit/update time).

## Local fix location

- Worktree: `/Users/xuanyu.wang/repos/go-servers-stale-reporting`
- Branch: `scorecard-sync-monitor-stale-reporting`
- Files:
  - `cron/task-runner/tasks/scorecard-sync-monitor/result_collector.go`
  - `cron/task-runner/tasks/scorecard-sync-monitor/result_collector_test.go`
  - `cron/task-runner/tasks/scorecard-sync-monitor/README.md`

## Outcome

- Committed and rebased onto `origin/main`
- PR: https://github.com/cresta/go-servers/pull/30911
