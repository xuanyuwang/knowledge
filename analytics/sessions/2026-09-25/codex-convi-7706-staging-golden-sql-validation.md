# CONVI-7706 staging golden SQL validation

Date: 2026-09-25
Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
Branch/worktree context: `/Users/xuanyu.wang/repos/go-servers-convi-7706`, commit `b1568e1d7252d7b2d06919dc8e622a18dcef985b`, PR #32440

## Objective

Execute both committed `RetrieveConversationStats` value-or-missing golden SQL files against a real staging ClickHouse database with correctly substituted runtime arguments.

## Target

- Config ref: config `origin/master` at `3517fe2ea20e5f7e1d7e89d6ab066898c281b78f`
- Config path: `configv3/staging/cresta/walter-dev/config.yaml`
- Deployment cluster: `voice-staging`
- Host: `clickhouse-conversations.voice-staging.internal.cresta.ai`
- Database selected by the connection skill: `cresta_walter_dev`
- Service: conversations

## Queries

Validated both committed files:

- `clickhouse_RetrieveConversationStats_FilterByMetadataMomentGroup_ValueOrMissing_GroupByAgent_LatestValue_request.sql`
- `clickhouse_RetrieveConversationStats_FilterByMetadataMomentGroup_ValueOrMissing_GroupByAgent_AllowMatchingStaleMetadataValues_request.sql`

First, each file was executed unchanged. Both exited successfully and returned no rows because their committed `2021` dates and `moment_id_1` values are synthetic test fixtures.

For data-bearing validation, only the fixture literals were substituted:

- time range: `[2026-09-25 00:00:00, 2026-09-27 00:00:00)`
- metadata template: `d8bee65e-a2f6-4c8a-82b6-42ba86293ca4`
- selected values, first run: `user_hangup`, `transfer`
- selected values, second run: `transfer`, `agent_hangup`

The complete generated aggregations were executed with `FORMAT Null`, then wrapped unchanged in a compact outer aggregate to prove they returned data.

## Results

| Selected values | Mode | Aggregate rows | Summed conversation count | Summed handle-time seconds |
| --- | --- | ---: | ---: | ---: |
| `user_hangup`, `transfer` | latest value | 52 | 1,173 | 44,255 |
| `user_hangup`, `transfer` | allow stale | 52 | 1,173 | 44,255 |
| `transfer`, `agent_hangup` | latest value | 50 | 698 | 11,892 |
| `transfer`, `agent_hangup` | allow stale | 50 | 698 | 11,892 |

Both SQL shapes compile, execute, join the production tables, and return non-empty grouped data with real Walter Dev values. The equality of latest and stale results means this staging sample does not demonstrate a stale-only match; it does not invalidate the distinct query branches or the local fixture coverage that exercises their structural difference.

## State and credentials

- Read-only ClickHouse queries only; no database, product code, PR, or production state changed.
- Used the explicitly inspected and cleared `voice-staging_dev` AWS SSO profile to retrieve the configured ClickHouse secret. The secret was passed only through `CLICKHOUSE_PASSWORD` and was not printed or persisted.
- Config fetch used the existing GitHub keyring token over HTTPS. No SSH, Okta, Azure, or other AWS profile credential was used.
