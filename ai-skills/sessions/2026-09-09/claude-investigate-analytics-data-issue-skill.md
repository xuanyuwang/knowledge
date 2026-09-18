# Session: investigate-analytics-data-issue skill for coaching-qm-skills

- Source repo: `/Users/xuanyu.wang/repos/coaching-qm-skills` (branch `xw/investigate-analytics-data-issue-skill`, cut from `origin/main` at `055310f` after PR #2 merged)
- Date: 2026-09-09
- Tool: Claude Code

## Objective

Distill the reusable investigation skill for data issues on the Performance Insights and
Leaderboard pages — high level: query PG to confirm the source of truth, cross-check ClickHouse —
from this knowledge repo, and land it as a skill in `coaching-qm-skills`.

## Inputs reviewed

Fan-out synthesis across three parallel reads of this repo:

1. `analytics/` domain and subdomains (performance-insights, leaderboard,
   shared-analytics-platform, qa-score, conversation-volume, active-days, quintiles,
   insights-user-filter, assistance-insights) plus `deliverables/time-range-timestamp-mapping.md`
   and the other analytics deliverables/work-items/sessions.
2. `analytics/` and `scorecard-data-sync/` investigation sessions for concrete query patterns
   (cresta-cli access, PG SQL, CH SQL, validation loops).
3. Legacy top-level ticket folders (convi-6260, convi-6968, convi-7230, convi-6842, convi-6808,
   convi-7162, hilton-coaching-discrepancy, pg-ch-scorecard-sync-investigation,
   historic-scorecard-missing, convi-5565, large-user-id-clickhouse, active-days,
   agent-stats-active-days-fix).

Also cross-checked for overlap against `.claude/skills/query-pg-coaching`,
`.claude/skills/query-ch-insights`, and
`pg-ch-scorecard-sync-investigation/deliverables/pg-ch-data-sync-investigator/SKILL.md`.

## Key design decisions

- The new skill owns **page-metric lineage and diagnosis routing** (metric → API → table →
  timestamp column; PG source-of-truth confirmation; CH cross-check; mismatch classification).
- Connections delegate to the existing `connect-customer-app-db` / `connect-customer-clickhouse`
  skills; repair execution delegates to the PG→CH sync playbook (targeted reindex, never direct
  CH writes). This keeps the new skill non-duplicative with both.
- The recurring core of the skill: `scorecard_time` vs `scorecard_submit_time` time basis,
  exact-ID (not time-window) existence checks, `FINAL`/argMax dedup, expected-zero exclusion
  classes, bit-for-bit number reproduction as the confirmation standard, and the root-cause
  catalog with previously-rejected hypotheses.

## Deliverable

- `investigate-analytics-data-issue/` in `coaching-qm-skills`: `SKILL.md`,
  `agents/openai.yaml`, and four references (`metric-source-mapping.md`,
  `pg-source-of-truth-queries.md`, `clickhouse-crosscheck-queries.md`,
  `root-cause-catalog.md`), plus a README layout entry.
- Commit `fe7efac` on branch `xw/investigate-analytics-data-issue-skill`;
  PR [coaching-qm-skills#3](https://github.com/cresta/coaching-qm-skills/pull/3).

## Next steps

- PR review; if a Linear ticket is wanted for tracking, create one the way CONVI-7659 was used
  for PR #2.
- After merge: symlink the skill into `~/.claude/skills`, `~/.codex/skills`,
  `~/.cursor/skills` per the repo README.
