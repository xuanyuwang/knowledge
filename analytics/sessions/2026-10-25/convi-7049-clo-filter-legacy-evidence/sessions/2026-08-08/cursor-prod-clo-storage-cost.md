# Prod CLO storage cost after backfill

Date: 2026-08-08
Source repo: `/Users/xuanyu.wang/repos/clickhouse-schema`
Branch: `main`

## Request

Record storage cost after full-history Phase B, compared with the initial one-year backfill plan.

## Measurement

Queried active `system.parts` entries for `moment_annotation_by_conversation_outcome` across all replicas on the six completed prod clusters. Used `bytes_on_disk` as physical capacity consumed. Approximated one year with monthly partitions `202508` and newer.

AWS SSO contributor roles were no longer assigned, so read-only Kubernetes contexts were used to verify PVC storage class. ClickHouse credentials were retrieved from the configured Secrets Manager IDs through each cluster's `_dev` AWS profile without printing or persisting credentials.

## Result

- Full-history physical footprint: 64.18 GiB.
- Approximate one-year footprint: 53.11 GiB.
- Older-history increment: 11.08 GiB.
- Full history is 20.9% larger than one-year-only.
- Older history is 17.3% of the full footprint.

Cluster totals:

- east: 32.327 GiB
- west: 19.161 GiB
- voice: 8.509 GiB
- chat: 3.434 GiB
- AP: 0.579 GiB
- EU: 0.171 GiB

All measured conversations PVCs use gp3. The immediate marginal EBS bill is zero because EBS charges already-provisioned volume capacity. Capacity-equivalent list-price value is about $5.15/month full history, $4.26/month one-year-only, and $0.89/month for the older-history increment.

## Caveats

- The partition approximation includes all of August 2025.
- Physical size includes replication and active parts before future merges.
- Snapshot, IOPS, throughput, and data-transfer costs are excluded.

## Artifacts

- `deliverables/clo-prod-storage-cost-2026-08-08.md`
- Cursor canvas: `prod-clo-storage-cost.canvas.tsx`
- Posted the result to CONVI-7460.
