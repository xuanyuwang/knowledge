# CONVI-7383 Phase A — prod verify + staging blocker check

**Date:** 2026-08-05
**Agent:** Cursor (subagent)
**Tickets:** CONVI-7431 (prod), CONVI-7420 (staging), parent CONVI-7383
**Repo:** `cresta/clickhouse-schema`

## Prod all-prod GHA

- Run: https://github.com/cresta/clickhouse-schema/actions/runs/31009131842
- Inputs: `cluster_target=all-prod`, `dry_run=false`, ref `main`
- Workflow: success (all jobs green)
- Verdict: **PARTIAL**

| Cluster | Outcome |
| -- | -- |
| ap-southeast-2-prod | All DBs OK (5/5) |
| ca-central-1-prod | Failed: `cresta_ca_ca_central_1`, `telus_ca_central_1` — Code 209 SocketTimeoutError on connect (~30s) |
| chat-prod | All DBs OK (11 cust / 33 profiles) |
| comcast-prod | All DBs OK (8/8) |
| eu-west-2-prod | All DBs OK (9/9) |
| schwab-prod | All DBs OK (6/6) |
| us-east-1-prod | All DBs OK (93 cust / 95 profiles) |
| us-west-2-prod | 1 failed: `indeed_mg_sbx_us_west_2` — Code 47 missing `moment_annotation_payload`; ~163 other profiles OK |
| voice-prod | All DBs OK (27 cust / 52 profiles) |

Linear CONVI-7431 left **In Progress**; status comment posted.

## us-west-2-staging blocker

- Still blocked — **did not** trigger Phase A apply
- Pod `chi-conversations-conversations-0-0-0`: CrashLoopBackOff, 102 restarts, node `ip-10-18-202-229`
- Code 246 CORRUPTED_DATA: clickhouse binary checksum mismatch
- `distributed_ddl_queue`: Inactive host `chi-conversations-conversations-0-0:9440`
- Probe DROP ON CLUSTER SYNC → code 159 timeout (1/9 hosts)
- voice-staging already succeeded earlier (run 31006348375)

Linear CONVI-7420 status comment posted (still In Progress).

## Follow-ups

1. Platform: recover staging host 0-0 (corrupt binary / node)
2. Re-run Phase A for `ca-central-1-prod` after connect works
3. Optional: skip/fix `indeed_mg_sbx_us_west_2` schema
4. After staging healthy: GHA `cluster_target=us-west-2-staging` dry_run=false
