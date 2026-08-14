# Large prod Phase B estimate refresh

Date: 2026-08-07
Source repo: `/Users/xuanyu.wang/repos/clickhouse-schema`
Branch: `main`

## Request

Refresh the Phase B time estimate for `us-west-2-prod` and `us-east-1-prod`. Do not start either backfill.

## Live inventory

- West: 244,348,864 raw `moment_type=14` rows; 87/173 physical databases non-empty.
- East: 435,480,504 raw rows; 68/96 physical databases non-empty.
- Both distributed DDL queues had zero non-Finished entries.

## Calibration

Completed prod runner intervals:

- eu-west-2: 19.4s
- ap-southeast-2: 17.2s
- chat: 159.7s
- voice: 436.5s

Their combined 115.2M raw rows / 632.9s gives about 182k raw rows/sec effective. Chat alone provides a slower observed production baseline near 115k/s.

Resulting apply-runner estimates:

- West: ~22m at 182k/s; ~35m at 115k/s. Planning window including workflow overhead: 30–45m.
- East: ~40m at 182k/s; ~63m at 115k/s. Planning window including workflow overhead: 45–75m.
- Sequential apply workflows: roughly 75–120m total, excluding dry runs and post-validation.

The old 28k/s staging baseline gives 2.42h west / 4.32h east but is no longer the expected case after production calibration.

## Largest-query risk

- `united_east_us_east_1`: 198,993,320 raw rows; ~18m expected / ~29m conservative.
- `cvs_us_west_2`: 140,801,206 raw rows; ~13m expected / ~20m conservative.

Both are below the 3,600-second per-query limit at observed prod rates. Monthly chunks remain prudent for retries and unexpected load.

## Phase A drift

Normal-schema databases requiring Phase A:

- West: `enova_us_west_2`, `goodtime_sbx_us_west_2`, `goodtime_us_west_2`.
- East: `bill_us_east_1`, `comcast_us_east_1`.

Approved zero-row skips because `moment_annotation_payload` is absent:

- `angi_us_west_2`
- `hsbc_us_west_2`
- `indeed_mg_sbx_us_west_2` (partial storage only)
- `mpa_sbx_us_west_2`
- `mpa_us_west_2`
- `nyman_turkish_us_west_2`
- `rcg_us_west_2`

These do not alter row-volume timing.

## Timeout investigation

- `.github/workflows/conversations-schema-operations.yaml` sets `timeout-minutes: 90` on each cluster matrix job.
- `main.py` sets `max_execution_time=3600`, `send_receive_timeout=3900`, and `distributed_ddl_task_timeout=900`.
- West must average about 47k raw rows/s and east 84k/s to fit the GHA window after allowing roughly four minutes of setup. The conservative observed prod rate is 115k/s.
- CVS must exceed about 39k/s and United East 55k/s to fit the 60-minute per-query limit.
- Individual query exceptions are caught, appended to `failed_dbs`, and only logged. `main.py` does not return a non-zero exit code when `failed_dbs` is non-empty, so GHA can be green despite a timed-out DB.
- The reusable workflow has `max-parallel: 1`; a group target is serial, contrary to the old parallel-fleet assumption. Use separate west/east targeted runs.
- `start_from_customer` provides a resume checkpoint. Re-running the CLO insert is logically deduplicated by the target ReplacingMergeTree under FINAL, but it creates temporary physical duplicates until merges, so scoped retries or monthly chunks are safer.

## Artifact

Updated `deliverables/clo-prod-backfill-cost-estimate-2026-08-05.md` and created the Cursor canvas `prod-clo-backfill-estimate.canvas.tsx`.
