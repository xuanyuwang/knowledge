# Prod CLO storage cost after Phase B

Measured: 2026-08-08
Tracking ticket: [CONVI-7460](https://linear.app/cresta/issue/CONVI-7460/rollout-phase-b-gha-apply-prod)

## Conclusion

- Full-history CLO target footprint across all physical replicas: **64.18 GiB**.
- Approximate one-year footprint (`partition >= 202508`): **53.11 GiB**.
- Older-than-one-year increment: **11.08 GiB**.
- Full history consumes **20.9% more** storage than a one-year-only backfill.
- Older history represents **17.3%** of the current full-history footprint.
- Immediate marginal EBS bill: **$0/month** because the data occupies already-provisioned gp3 PVCs. Billing changes only if this contributes to a future volume expansion.
- Capacity-equivalent gp3 value: approximately **$5.15/month** for full history, **$4.26/month** for the one-year approximation, and **$0.89/month** for older history.

## Physical active-part footprint

- `us-east-1-prod`: 32.327 GiB full; 28.148 GiB from 2025-08 onward; 4.180 GiB older.
- `us-west-2-prod`: 19.161 GiB full; 16.288 GiB recent; 2.873 GiB older.
- `voice-prod`: 8.509 GiB full; 6.980 GiB recent; 1.529 GiB older.
- `chat-prod`: 3.434 GiB full; 0.941 GiB recent; 2.494 GiB older.
- `ap-southeast-2-prod`: 0.579 GiB, all in the recent window.
- `eu-west-2-prod`: 0.171 GiB, all in the recent window.

## Method

Queried active `system.parts` rows for `moment_annotation_by_conversation_outcome` with `clusterAllReplicas('conversations', ...)` on every completed prod cluster. Summed `bytes_on_disk` across all replicas to measure physical capacity consumed. Classified monthly partitions `202508` and newer as the approximate one-year window.

Verified conversations PVC storage class is `gp3` on every measured cluster. Applied current list rates:

- us-east-1 / us-west-2: approximately $0.08 per GiB-month.
- eu-west-2: approximately $0.0928 per GiB-month.
- ap-southeast-2: approximately $0.096 per GiB-month.

## Boundaries

- The one-year comparison includes all of August 2025, not an exact rolling cutoff at August 8.
- Active-part size includes replicated physical storage and any parts awaiting future background merges.
- The estimate excludes EBS snapshots, data transfer, and separately provisioned IOPS/throughput.
- EBS charges provisioned volume capacity, not live bytes used; the equivalent values are allocation values, not incremental invoice charges.
