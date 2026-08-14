# CONVI-7383 Phase B staging apply

**Date:** 2026-08-07
**Source repo:** `/Users/xuanyu.wang/repos/clickhouse-schema`
**Target:** `us-west-2-staging`
**Linear:** CONVI-7423

## Cost estimate

Read-only inventory found 32 physical databases with `moment_annotation_d`; GHA config resolved 29 active profiles. Only five databases had `moment_type=14` rows, totaling 4,029 raw rows.

Using the voice-staging baseline of approximately 28k inserted rows/second:

- estimated data time: ~0.15 seconds;
- runner per-DB delay: ~29 seconds for configured profiles;
- estimated wall time including GHA provisioning: under five minutes.

## Runs

- Dry run: https://github.com/cresta/clickhouse-schema/actions/runs/31191070864
  - All 29 configured profiles processed successfully.
  - Wall time ~4m12s, dominated by runner provisioning.
- Apply: https://github.com/cresta/clickhouse-schema/actions/runs/31191460937
  - Wall time ~5m44s; DB processing ~29s.
  - Expected failure only: `authtest_profile_1` Code 47 because its source lacks `moment_annotation_payload`; source CLO row count is zero.

## Validation

Source FINAL and CLO FINAL counts matched exactly for every non-empty database:

- `cresta_email_dev`: 38
- `e2e_ghost_profile_1`: 3,480
- `e2e_target_profile_1`: 51
- `manual_qa_profile_1`: 457
- `manual_qa_profile_2`: 3

The distributed DDL queue had zero non-`Finished` tasks.

voice-staging Phase B had already completed in run 31023390119 and was intentionally not rerun. CONVI-7423 was updated and marked Done. The Insights flag remains off pending separate pilot validation.
