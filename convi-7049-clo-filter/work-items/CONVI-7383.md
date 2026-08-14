# CONVI-7383 — CLO materialized-view rollout

## Objective

Roll out the typed conversation-outcome ClickHouse storage table, trigger materialized view, and distributed table before backfilling or enabling Insights query routing.

## Current status

Phase A and the eligible Phase B production scope are complete. As of 2026-08-14:

- `us-west-2-staging` Phase A is complete and verified on all nine hosts: 31 databases have storage + trigger MV + `_d`; legacy `authtest_profile_1` lacks `moment_annotation_payload` and therefore has storage + `_d` only. Restarting host `1-2` recovered its DDL worker and cleared the queued DROP.
- Phase B is complete on all staging targets. us-west-2-staging backfilled 4,029 rows across five non-empty DBs with exact source FINAL/CLO FINAL parity; legacy zero-row `authtest_profile_1` was the only expected Code 47 exception. CONVI-7420 and CONVI-7423 are Done.
- Prod Phase A preflight on 2026-08-07 passed seven local clusters plus Comcast through GHA/OIDC. ca-central remains unavailable: no conversations pods, zero service endpoints, and CH:9440 timeout. Current config still assigns Cresta Canada and Telus to it. `main` was not changed.
- Prod Phase A is complete under the accepted skip scope: no new action on ca-central, Comcast, or Schwab; `indeed_mg_sbx_us_west_2` remains an intentional legacy sandbox exception. All other prod clusters already had Phase A and passed preflight. CONVI-7431 is Done.
- Prod Phase B is complete on eu-west-2, ap-southeast-2, chat, voice, and us-west-2. West Phase A catch-up covered three normal-schema zero-row DBs; dry run `31226569107` and apply `31226889264` passed. Validation covered 87 non-empty DBs with 197,005,915 expected keys and 197,007,503 CLO FINAL rows; every delta was non-negative and the +1,588 total reflects concurrent trigger writes. chat validation found two known cross-shard stale rows—one each in `cresta_chat_demo` and `verizon_wireless`—because the distributed table shards by exact conversation timestamp while the replacement key uses hour-level time; scoped replays cannot remove them.
- Prod Phase B is complete under CONVI-7460, including us-east-1. East Phase A catch-up covered `bill_us_east_1` and `comcast_us_east_1`; dry run `31232323695` and apply `31232485782` passed. Validation checked 68 non-empty DBs with 317,036,293 expected keys and 317,041,632 CLO FINAL rows; no target was behind and +5,339 rows were concurrent trigger writes. ca-central, Comcast, Schwab, and no-payload legacy DBs remain excluded.
- Post-backfill physical storage is 64.18 GiB across all replicas. A one-year-only backfill would be approximately 53.11 GiB; older history adds 11.08 GiB (20.9%). Existing gp3 PVC capacity means no immediate incremental EBS bill; equivalent allocated value is ~$5.15/month full history and ~$0.89/month for older history. See `deliverables/clo-prod-storage-cost-2026-08-08.md`.
- `ca-central-1-prod` remains unreachable on ClickHouse port 9440.
- `indeed_mg_sbx_us_west_2` remains intentionally skipped because its source table lacks `moment_annotation_payload` and the customer has `skipSchemaRelease: true`.
- `clickhouse-schema` `main` still carries the Phase B backfill in `queries.sql` at `69d6a05`; restore Phase A DDL before the next Phase A GHA.
- NCLH release readiness is verified at the GitOps level. NCLH uses `us-east-1-prod` → Insights `03-prod-main`, whose declared image `main-20260809_072100z-022144a6` contains merged routing commit `68c0fd7d` from go-servers #30588.
- Config PR [#151310](https://github.com/cresta/config/pull/151310) is open to enable `use_conversation_outcome_moment_annotation_materialized_view` for NCLH and NCLH sandbox. It is safe to merge with respect to MV/backfill and declared Insights release compatibility. Live pod verification remains pending because local AWS SSO was expired.
- A low-volume NCLH browser check with the MV path enabled completed a 180-day refresh in 37.5–40.5 seconds, directionally 2.12–2.29× faster than the historical 85.83-second raw-table query. This is not a controlled request pair or p95 result.
- Ready-for-review config PR [#151643](https://github.com/cresta/config/pull/151643) aligns the customer-facing `enableCLOFilters` flag with the MV-read flag at every supported production scope: 1 profile + 18 use cases for NCLH and 5 profiles + 6 use cases for Holiday Inn voice. `holidayinn_chat` remains unset for both flags. CI is green and GitHub reports the PR clean and mergeable.

## Next actions

1. Review, merge, and deploy config #151643 for the exact supported NCLH and Holiday Inn production scopes.
2. Capture comparable NCLH and eligible Holiday Inn CLO requests after enablement, including query-log evidence; NCLH has a historical baseline, while Holiday Inn does not.
3. Keep legacy `holidayinn_chat` excluded: its source lacks `moment_annotation_payload`, so it has storage + `_d` but no trigger MV. Config #151311 correctly enables only Holiday Inn voice profiles and does not include chat.
4. Treat ca-central, Comcast, and Schwab as excluded from new rollout actions unless scope is explicitly reopened.
5. Preserve the accepted prod exclusions unless scope is explicitly reopened.
6. Before enabling the flag for Cresta chat-demo or Verizon wireless, resolve or explicitly accept the two-row chat cross-shard deduplication anomaly.

## Flag enablement (how)

When prerequisites pass, set per customer/profile in config:

```yaml
features:
  insights:
    useConversationOutcomeMomentAnnotationMaterializedView: true
```

Preferred: Cresta Admin **Feature Config** flags UI (`insights.useConversationOutcomeMomentAnnotationMaterializedView`) → reviewed `customer_config_history` PR. Not a Director flag. Details: `sessions/2026-08-07/cursor-enable-clo-mv-config-flag.md`. Pilot ticket: CONVI-7424.

## Evidence

- All-prod apply: https://github.com/cresta/clickhouse-schema/actions/runs/31009131842
- ca-central retry: https://github.com/cresta/clickhouse-schema/actions/runs/31017776702
- Latest blocker check: `sessions/2026-08-06/cursor-phase-a-blocker-recheck.md`
- Local staging apply attempt: `sessions/2026-08-07/cursor-phase-a-local-apply-blocked.md`
- Staging Phase B apply: `sessions/2026-08-07/cursor-phase-b-staging-apply.md`
- Prod Phase A preflight: `sessions/2026-08-07/cursor-prod-phase-a-preflight.md`
- Prod Phase B backfill: `sessions/2026-08-07/cursor-prod-phase-b-backfill.md`
