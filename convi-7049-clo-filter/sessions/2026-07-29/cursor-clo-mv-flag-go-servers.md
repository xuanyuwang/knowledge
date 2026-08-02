# Session: apply CLO MV Insights flag in go-servers

Date: 2026-07-29
Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7383`
Branch: `convi-7383-clo-mv-flag`
PR: https://github.com/cresta/go-servers/pull/30588

## Config check
- Flag present in `config` origin/master `json-schema/configv3/Insights.json` as `useConversationOutcomeMomentAnnotationMaterializedView`.
- Schema sync commit references cresta-proto `386f8c1a6f` (#9400).

## Implementation
- Bump cresta-proto to v2.15.86.
- `parseMomentConditionsForQAAttribute` returns per-group `momentAnnotationFilter` with source.
- Metadata always uses metadata MV; CLO uses raw table unless Insights flag enables outcome MV typed-column path.
- Wired in QA score stats + QA conversations ClickHouse readers via `GetProfileConfig`.
- Test schema adds `moment_annotation_mv_by_conversation_outcome[_d]`.

## Note
Do not enable the customer flag until ClickHouse MV exists and 180d backfill is validated.
