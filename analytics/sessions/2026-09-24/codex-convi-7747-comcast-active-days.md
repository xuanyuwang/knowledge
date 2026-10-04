# CONVI-7747 Comcast Active Days investigation

- Date: 2026-09-24 (America/Toronto)
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Source context: clean main checkout at merged overlap-fix commit `772c0f2590b45803acd07092a9460dc15cdb61f9`
- Secondary repo: `/Users/xuanyu.wang/repos/config` at latest fetched `origin/master` `aa9f2cafce47ebfbf6cbf54a109b8bfdb556dbf5`
- Ticket: https://linear.app/cresta/issue/CONVI-7747/comcast-residential-agent-leaderboard-active-days-shows-0n-a-for-all

## Finding

Comcast Residential's Acquisition voice use case is `customers/comcast/profiles/prod`, not Bill West. The earlier August 20 preparation in this project's `codex-active-days-aug20-backfill-prep.md` was a provisional wrong-customer assumption; its Bill West command must not be run for this ticket.

CONVI-7747 reports a uniform August 20 transition across two teams. It provides exact August 19 conversation IDs for Nick DiVeglia and Matthew Nadolski. Conversation existence alone does not establish Active Days: the metric requires qualifying Agent Assist online evidence matching a conversation/message, and the labeler then writes `conversation_with_labels_d`. The confirmed Bill West defect can produce false negatives when a long online session begins before the three-hour event query window, but the Comcast evidence needed to show this exact fingerprint has not been obtained.

Latest config has only `configv3/prod/comcast/prod/config.yaml` under the `comcast` customer; it identifies the `prod` profile, `comcast-prod` cluster, and `acquisition` voice use case. The app DB secret reference identifies database `comcast` in us-east-2. The checked-in profile and acquisition insights YAML do not set `excludeIngestionPipelineFromAgentStats`; in current `IncludeIngestionPipelineInAgentStats` code, absent config means source 8 remains included. This is not proof of live effective ConfigService state.

Historical config changes on August 20: `56ff49dbc9f` added Acquisition skill `9993199`; `d2e47686493` added demo chat routing skills; `2d14d7194ca` changed `preventAgentConversationAccess` true to false. None is an evidenced Active Days root cause. No Active Days-specific profile insights change appears in that narrow August 18-22 config history.

## Production access attempts

- Read `connect-customer-app-db` and `connect-customer-clickhouse` skill instructions. Both fetched the latest config, confirming the same profile.
- App DB default `cresta-cli connstring -i --read-only` could not resolve `comcast-prod` as a CLI account.
- The individually reviewed ordinary `cresta-pci-account_dev` AWS SSO profile was refreshed and tried through the skill's read-only account override. That account's RDS cluster list does not include `comcast-prod`, so no SQL was run.
- ClickHouse skill could not find local AWS profile `comcast-prod_dev`; it did not retrieve a secret or run SQL.
- Reviewed `~/.ssh` directory, GitHub host config, and the GitHub public-key comment before using that GitHub SSH credential to fetch the latest config. Did not read or use the separate marked SSH credential. Did not read or use the unrelated local AWS credentials-file entry.
- No credentials or credential-bearing URI were written to notes or output.

## Exact next read-only checks

Resolve both conversation IDs in app `chats` including source, profile, usecase, agent, start and end times, and inspect agent-role `messages.platform_timestamp`. Query events ClickHouse `user_event_d` for the corresponding agent IDs, application types 0/2/4/8, `isOnline`, and session start/end on August 19 and 20. Query conversations ClickHouse `conversation_with_labels_d FINAL` by exact IDs, including agent, usecase, `has_agent_assistance`, message ID/time, and update time. Compare with a same-agent August 20 positive example and reproduce the API request's local daily bucket. Evaluate both the historical start-window predicate and corrected overlap predicate for each conversation's end-time batch. Then classify the cause and prepare a bounded replay only if the persisted labels are wrong.

## Limitation

There is no live database evidence yet that the Comcast ticket shares the Bill West root cause. Config and source inspection narrows the target and hypotheses; it does not justify a backfill job.
