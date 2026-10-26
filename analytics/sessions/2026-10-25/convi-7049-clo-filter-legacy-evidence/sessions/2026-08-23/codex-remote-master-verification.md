# Remote master CLO verification

Verified current `cresta/config` remote default branch (`master`) after repair PR #152115.

## Results

- PR #152115 merged on 2026-08-22 at commit `b4432a033b5de6d9bd507a2416973542f7f27856`.
- Current `origin/master` customer history has:
  - 1,891 eligible `enableCLOFilters: true` maps.
  - Zero missing or false values among eligible customers.
  - Exactly 117 intentional missing maps across the 17 Comcast/Schwab exclusions.
- The post-merge Batch Sync to ConfigService run was cancelled.
- Current generated `configv3/prod` contains the enabled flag in 83 files. Spruce now has it, while Uber, Twilio, and eBay still do not.

## Production replay

- Manually dispatched `sync_to_config_service_batch.yaml` from `master` with `diff-only=false`, `environment=prod`, and all 354 eligible customers; Comcast and Schwab customer configs were excluded from the input.
- Workflow [run 32655059116](https://github.com/cresta/config/actions/runs/32655059116) completed successfully on 2026-08-23.
- Every production regional sync step succeeded, including `ap-southeast-2`, `ca-central-1`, `comcast`, `eu-west-2`, `gke-us-central1`, `schwab`, `us-east-1`, and chat/voice `us-west-2`.
- The regular Flux repository dispatch and Terraform customer-regeneration trigger also succeeded.

Conclusion: remote Git desired state is complete and the full eligible fleet replay completed successfully. Effective ConfigService/Admin read-back remains the final verification step.
