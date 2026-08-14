# Phase B backfill performance (voice-staging)

**Date:** 2026-08-05
**GHA:** https://github.com/cresta/clickhouse-schema/actions/runs/31023390119
**Commit:** `69d6a05` (full history backfill, no date filter)
**Linear:** CONVI-7423, CONVI-7424

## Observed

| DB | Wall time | clo_raw | src_final (m14) |
|---|---:|---:|---:|
| cresta_walter_dev | ~17.3s | 482,209 | 485,416 |
| cresta_voice_integration_e2e | ~1.1s | 11,361 | ~11.3k |
| empty DBs | ~1s | 0 | 0 |

**Throughput (walter-dev):** ~28k rows/sec.

## Ballpark projection (linear, same cluster shape)

| FINAL m14 rows | Est. wall time |
|---:|---:|
| 1M | ~35s |
| 10M | ~6 min |
| 50M | ~30 min |
| 100M | ~1 h |

## Caveats

- Staging ≠ prod load / skew / concurrency
- INSERT into Distributed `_d`
- Prefer monthly chunks for large customers; validate one large prod pilot first
