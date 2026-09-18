# CONVI-7667 Oportun process-scorecard Performance Insights investigation

**Date:** 2026-09-10
**Primary domain:** analytics / performance-insights
**Source repo:** `/Users/xuanyu.wang/repos/director`
**Related repos:** `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/config`
**Context:** read-only investigation; no product-code changes

## Inputs

- Linear issue CONVI-7667 and its three embedded screenshots, read through the authenticated Linear MCP.
- Requested repository skill: `coaching-qm-skills/investigate-analytics-data-issue`.
- Read-only customer access through the repository's app-DB and ClickHouse skills.
- Current Director and go-servers request/query paths plus Director commit history.

## Screenshot resolution

- Populated screenshot: calendar labels start `Tue 07/01`, identifying July 2025; displayed 99% and volume 352.
- Zero screenshots: labels start `Wed 07/01`, identifying July 2026; displayed 0% and volume 0.
- The ticket's claim that these are identical date windows is therefore false at the year level.

## Data lineage

- PI score/volume: `RetrieveQAScoreStats` -> ClickHouse `score_d` / `scorecard_d`.
- For process scorecards, CH `scorecard_time` is PostgreSQL `director.scorecards.process_interaction_at` when populated.
- QM/source truth: PostgreSQL `director.scorecards`, exact template resource ID `94e6b75d-b1eb-4e41-a477-1d070d96fa9c`, revision `c0139eb5`, type 2.

## PostgreSQL evidence

Indexed by `(customer, profile, template_id, template_revision)` using customer `oportun`, profile `us-west-2`:

| Window | Process-interaction rows | Submitted rows | Scored rows |
|---|---:|---:|---:|
| July 2025, UTC half-open month | 353 | 353 | 353 |
| July 2026, UTC half-open month | 338 | 338 | 338 |

All 5,750 scorecards for this revision span 2025-04-02 through 2026-09-09 in `process_interaction_at`. The July-2025 rows were created/submitted within July; the latest was July 31, which can explain a historical screenshot taken before the final row appeared.

The exact template JSON still contains scorable `labeled-radios` criteria with Yes=1 / No=0; this is not a Text/Date-only or evaluate-scores-disabled template.

## ClickHouse evidence

Database `oportun_us_west_2`, distributed tables with `FINAL`:

| Window | `scorecard_d` distinct | `score_d` scorecards with valid rows | Weighted score |
|---|---:|---:|---:|
| July 2025 | 353 | 353 | 99.3404% |
| July 2026 | 338 | 338 | 99.7207% |

All rows use `backoffice-processes`; no dev-user rows were observed. The ClickHouse projection agrees with PostgreSQL for the decisive July populations.

## Root cause trace

1. Director `bd4074c5f4` (2026-08-26, CONVI-7586 / PR #22057) changed PI's initial `dateRangeTarget` from `undefined` to `CONVERSATION_ENDED_AT`.
2. `modifyFiltersState` removes many conversation-only filters for process templates, but leaves `dateRangeTarget` intact.
3. `useQAScoreStatsRequestParams` copies that state into `conversationTimeRangeField` for every QA request.
4. The backend's ended-at golden SQL builds a `conversation_d` CTE and inner-joins scorecards on `conversation_id`.
5. Process scorecards are standalone and have empty conversation IDs, so the join returns exactly 0 scorecards / 0 score rows for the affected request, matching the UI.

The date-target UI is only exposed when `hasEmailChannel` is true, so the Oportun voice process page silently carries a close-time target the user cannot see or change.

## Classification

**Presentation/request-construction regression.** PG and CH agree; no backfill or reindex is indicated.

## CI miss

PR #22057 added date-target control tests but no process-template request-construction regression. Existing process-template normalization had no assertion that conversation time targets are cleared.

## Recommended fix/validation

- Clear/omit `dateRangeTarget` whenever the selected scorecard template is type PROCESS, including initial state, cached state restoration, and template switches.
- Assert generated `RetrieveQAScoreStats` and `RetrieveQAConversations` requests omit `conversationTimeRangeField` for process templates.
- After deployment, verify July 2026 returns 338 scorecards and approximately 99.72% against the same live snapshot.

## Access notes

- Linear MCP was authenticated.
- Browser-control MCP access to Linear had earlier been unavailable because its permission request was dismissed; the dedicated Linear MCP fully replaced it.
- AWS SSO for `us-west-2-prod_dev` was refreshed before database access.
- The gRPC test helper was not used because `RetrieveQAScoreStats` is not in its reviewed read-only-method allowlist.
