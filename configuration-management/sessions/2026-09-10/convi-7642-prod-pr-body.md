## Summary

Enable the existing Director frontend flag `enableNAScore` for the current safe production cohort. Comcast- and Schwab-related identities are excluded by request. `cresta-testing-ca` and `the-zebra` are temporarily isolated because their ConfigService state contains absent-from-history summarization values that would otherwise cause unrelated effective changes.

## Scope

- 380 current production customer files inspected
- 360 customers included in this PR
- 1,963 effective profile/use-case scopes targeted
- 1,949 explicit `enableNAScore: true` additions across 360 files
- No additions, removals, or rewrites of any other configuration key

Excluded identities (18): `charles-schwab`, `comcast`, `comcast-ab`, `comcast-business`, `comcast-bve-sbx`, `comcast-dev`, `comcast-preprod`, `comcast-qa`, `cresta-comcast`, `cresta-schwab`, `cresta-testing-comcast`, `cresta-testing-schwabx`, `schwab`, `schwabdev`, `schwabpreview`, `schwabstaging`, `cmcst` (Comcast Business), and `cresta-sandbox` (Schwab-hosted).

Temporarily isolated: `cresta-testing-ca` and `the-zebra`. A whole-customer sync exposes stored empty `summarization_config` values that are absent from customer history. Adding those values would violate this PR's flag-only source-diff contract, while omitting them would remove effective configuration. Both customers require a separate patch-level reviewed path before the ticket's requested cohort is complete.

## Structural exceptions

Five included test/sandbox customers each have one email use case without a frontend feature-flag map: `cresta-testing-au`, `cresta-testing-east`, `cresta-testing-gcp-prod-us`, `cresta-testing-west`, and `marriott-gc-sbx`.

For these customers, the flag is set at profile level and all 14 use cases inherit it. This mirrors a separate ConfigService `LEVEL_PROFILE_ONLY` operation and avoids the `LEVEL_ALL` behavior that would reject the entire customer because of the mapless email use case. The inheritance-aware verification confirms all 1,963 scopes included in this PR resolve to true.

## Validation

- Requested flag diff: 360 files and 1,949 `enableNAScore: true` additions
- Every added line is exactly `enableNAScore: true`; there are no removed lines
- All changed paths are under `customer_config_history/prod/customers/`
- All 18 exclusion identities are unchanged
- `the-zebra` and `cresta-testing-ca` are unchanged pending a safe patch-level path
- Repeat updater dry runs are idempotent
- `yarn ajv-validate:configdb-frontend` passes
- The latest ConfigService dry-run artifact must contain only the 1,949 requested flag additions and zero removals or unrelated changes

## Rollout

The QA team previously tested this feature in staging. Production review is proceeding in parallel with the remaining staging read-back, per the rollout decision recorded on CONVI-7642. Do not auto-merge; after review and merge, verify the non-dry-run forward sync and complete ConfigService-to-`configv3` effective read-back.
