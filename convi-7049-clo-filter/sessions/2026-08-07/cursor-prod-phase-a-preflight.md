# CONVI-7383 prod Phase A preflight

**Date:** 2026-08-07
**Source repo:** `/Users/xuanyu.wang/repos/clickhouse-schema`
**Linear:** CONVI-7431

## Intent

Check prod ClickHouse availability before restoring Phase A DDL to remote `main`. No `clickhouse-schema` branch or schema was changed.

## Results

Local Kubernetes access worked after AWS SSO refresh.

Seven clusters passed full local preflight:

- `ap-southeast-2-prod`
- `chat-prod`
- `eu-west-2-prod`
- `schwab-prod`
- `us-east-1-prod`
- `us-west-2-prod`
- `voice-prod`

For each, all nine conversations pods were Ready, all nine ClickHouse hosts responded, and the distributed DDL queue had zero non-`Finished` target tasks. No read-only or session-expired replicas were found. Small replication queues of two or three entries existed on some clusters with at most one second delay.

`comcast-prod` has no local Kubernetes context. A read-only Phase B dry-run through the production GHA/OIDC path succeeded for all eight configured profiles:

- https://github.com/cresta/clickhouse-schema/actions/runs/31211026542

`ca-central-1-prod` failed the gate:

- no conversations ClickHouse pods exist in the `clickhouse` namespace;
- the `clickhouse-conversations` LoadBalancer service has zero endpoints;
- direct connection to `clickhouse-conversations.ca-central-1-prod.internal.cresta.ai:9440` times out;
- current config still places `cresta-ca/ca-central-1` and `telus/ca-central-1` there.

## Conclusion

Do not restore Phase A to `main` or run all-prod now. All healthy prod clusters already received Phase A in run 31009131842; re-running them would unnecessarily drop and recreate their trigger/distributed objects. The only required non-sandbox target is ca-central, which is unavailable. After its conversations workload is restored, switch `queries.sql` from Phase B to Phase A on `main` and run only `cluster_target=ca-central-1-prod`. Continue skipping `indeed_mg_sbx_us_west_2` unless schema hygiene is explicitly requested.
