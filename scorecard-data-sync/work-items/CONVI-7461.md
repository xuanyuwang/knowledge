# CONVI-7461: Surface stale scorecards in scorecard-sync-monitor Slack/cluster reporting

**Status:** validating
**Primary domain:** scorecard-data-sync
**Primary subdomain:** none
**Official ticket:** [CONVI-7461](https://linear.app/cresta/issue/CONVI-7461/surface-stale-scorecards-in-scorecard-sync-monitor-slackcluster)
**Last updated:** 2026-08-19

## Objective and Impact

- **Objective:** Report stale scorecard counts/rates alongside missing ones so stale-only customers no longer look healthy in Slack and cluster summaries.
- **Customer/system impact:** Snap Finance-style cases (`0 missing` while auto-heal reindexed stale rows) get visible severity and candidate counts.
- **Role:** implemented

## Scope

**In scope**

- scorecard-sync-monitor Slack, cluster summary, log status, tests, and README

**Non-goals**

- Changing auto-heal dispatch or ClickHouse comparison behavior
- Orphan cleanup (CONVI-7462)

## Source Context

- **Repos:** go-servers
- **Worktrees:** `/Users/xuanyu.wang/repos/go-servers-stale-reporting`
- **Branches:** `scorecard-sync-monitor-stale-reporting`
- **PRs/commits:** https://github.com/cresta/go-servers/pull/30911 (`54e8a2ca3d`)

## Current Understanding

The reporting change is implemented and all GitHub review comments are addressed. Slack headers now include `CLUSTER_NAME`. Merge is blocked only on `qm-coaching-squad` codeowner approval.

## Findings and Decisions

- Status, Slack header, and sort order use `max(missing rate, stale rate)`.
- Review comments (jobs suffix placement, log status, Slack header test, README fences/thresholds, Given/When/Then, job pluralization) were fixed on the PR branch in follow-up commits.

## Blockers and Dependencies

- Waiting on QM Coaching Squad codeowner review/approval.

## Next Actions

- Get a codeowner review on #30911 and merge.
