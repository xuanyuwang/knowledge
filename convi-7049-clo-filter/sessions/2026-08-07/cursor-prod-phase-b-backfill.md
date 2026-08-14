# Prod Phase B backfill

## Scope and outcome

Prod Phase B is complete for all approved clusters: `eu-west-2-prod`, `ap-southeast-2-prod`, `chat-prod`, `voice-prod`, `us-west-2-prod`, and `us-east-1-prod`. `ca-central-1-prod`, `comcast-prod`, and `schwab-prod` remain explicit skips. The Insights MV-routing flag remains off.

## chat-prod verification rerun

- Dry run: https://github.com/cresta/clickhouse-schema/actions/runs/31226441237
- Apply verification rerun: https://github.com/cresta/clickhouse-schema/actions/runs/31226725121
- Scoped replays:
  - `cresta/chat-demo`: https://github.com/cresta/clickhouse-schema/actions/runs/31227858787
  - `verizon/wireless`: https://github.com/cresta/clickhouse-schema/actions/runs/31228015715
- The runner processed 27 existing configured databases and normally skipped six configured profiles whose databases do not exist.
- Logical source cardinality is 15,874,144 keys using the target's replacement key `(toStartOfHour(conversation_start_time), conversation_id, moment_template_id)` and latest `update_time`.
- 25 checksum differences (`cox_sales`: 4, `verizon_main`: 21) are valid same-key, same-`update_time` ties. Every selected target row is one of the source max-version candidates.
- `cresta_chat_demo` and `verizon_wireless` each retain one stale cross-shard row in addition to the valid latest row. Both keys have multiple exact `conversation_start_time` values within the same hour. The distributed table shards by exact timestamp while the replacing key uses hour-level timestamp, so the two versions land on different shards and `FINAL` cannot collapse them globally.
- Scoped backfill replays succeeded but correctly did not remove those cross-shard stale rows. No production delete was attempted.
- Final DDL queue: zero non-Finished entries.

## Phase A drift found during chat preflight

- `porsche_beta_beta` and `porsche_main`: full Phase A caught up on all nine hosts.
- `holidayinn_chat`: source lacks `moment_annotation_payload`; storage + distributed table only, no trigger MV.
- `neiman_marcus_main`: database/source exists on only eight of nine hosts. Partial target creation was cleaned up and the zero-row legacy database remains excluded.

## Follow-up

Before enabling the flag for `cresta/chat-demo` or `verizon/wireless`, decide whether to clean up the two stale rows and address the sharding/replacement-key mismatch. Re-running Phase B alone cannot remove cross-shard duplicates.
# Prod Phase B backfill

Date: 2026-08-07
Source repo: `/Users/xuanyu.wang/repos/clickhouse-schema`
Branch: `main`

## Scope

Backfill eligible production clusters smallest-to-largest. Explicitly excluded:

- `us-east-1-prod` was initially deferred for size; west was subsequently approved and completed.
- `ca-central-1-prod`, `comcast-prod`, and `schwab-prod` by rollout decision.
- Known legacy `indeed_mg_sbx_us_west_2`.

The Insights MV routing flag remained off.

## Results

- `eu-west-2-prod`: dry run `31215749944`; apply `31217439748`. Exact validation: 1,090,494 source FINAL rows and 1,090,494 CLO FINAL rows.
- `ap-southeast-2-prod`: dry run `31217803903`; apply `31218079605`. Exact validation: 4,019,291 source FINAL rows and 4,019,291 CLO FINAL rows.
- `chat-prod`: Phase A catch-up applied to `porsche_beta_beta` and `porsche_main`. `holidayinn_chat` is a zero-row legacy source without `moment_annotation_payload`, so it received storage + `_d` only. Inactive `neiman_marcus_main` remains excluded because its source schema exists on only 8/9 hosts. Dry run `31218545325`; apply `31218805270`. Target counts matched the intended `(toStartOfHour(conversation_start_time), conversation_id, moment_template_id)` dedup key; tiny positive deltas represented concurrent trigger writes.
- `voice-prod`: Phase A catch-up applied to `neiman_marcus_group_sandbox_voice_sandbox`, `neiman_marcus_voice`, and `porsche_voice`. Dry run `31220530743`; apply `31220875375`. The config-driven runner omitted the latter two non-empty physical DBs, so they were directly backfilled and exactly validated at 875,517 and 347,341 dedup keys. Across all 24 non-empty DBs, expected target cardinality was 80,747,749 keys; observed positives above that baseline were live trigger writes.
- `us-west-2-prod`: Phase A catch-up applied to `enova_us_west_2`, `goodtime_sbx_us_west_2`, and `goodtime_us_west_2`; seven zero-row no-payload DBs remained excluded. Dry run `31226569107`; apply `31226889264` completed in ~18 minutes with no failed DBs and only the expected code-60 indeed skip. Full validation checked 87 non-empty DBs: 197,005,915 expected target keys versus 197,007,503 CLO FINAL rows, with no negative/material mismatch; +1,588 rows were concurrent trigger writes.
- `us-east-1-prod`: Phase A catch-up applied to zero-row `bill_us_east_1` and `comcast_us_east_1`. Dry run `31232323695`; apply `31232485782` completed in ~12 minutes with no failed/skipped DBs. Full validation checked 68 non-empty DBs: 317,036,293 expected target keys versus 317,041,632 CLO FINAL rows, with no negative/material mismatch; +5,339 rows were concurrent trigger writes.

Every completed cluster ended with zero non-Finished distributed DDL entries.

## Validation note

Source `moment_annotation_d FINAL` row count is not generally expected to equal CLO target count. The source key includes annotation identity, while the target intentionally replaces rows by hour + conversation + template. Validation must compare CLO FINAL count to distinct source target keys. Exact row parity happens only where the source has no duplicate target keys.

## Ticket update

Prod Phase B was tracked by CONVI-7460; west/east completion evidence was posted and the ticket was marked Done.
