# Configuration Management

## Scope

Customer configuration schemas, Config Admin editing workflows, GitOps review, synchronization to ConfigService, and safe feature-flag rollout practices.

## Current understanding

- `customer_config_history/` in the `config` repository is the reviewed source of truth for active customer configuration.
- Cresta Admin / Tenant Admin provides bulk feature-flag editing by calling ConfigService `BatchSetFeatureFlags`; ConfigService merges the selected profile/use-case flag maps and creates a config PR for review. The page does not dispatch a config GitHub Action for existing-customer enablement.
- A direct reviewed PR can reproduce that operation by merging only the requested boolean into every explicitly selected profile and use-case legacy frontend flag map. No existing config workflow performs this mutation; use an explicit customer manifest and semantic coverage checks.
- A Director flag intended for all production customers is materialized at every selected profile and use-case. There is no single wildcard production default in the current bulk workflow.
- The normal application level is **Auto Inheritance** (`LEVEL_ALL`), which applies the flag to selected profiles and their underlying use cases.
- PR CI runs schema/YAML validation and a diff-only ConfigService sync. Merging to `master` performs the real multi-region sync.
- [coaching-qm-skills#6](https://github.com/cresta/coaching-qm-skills/pull/6) hardens profile-only fallback and inline YAML handling. Its review findings are addressed, tests and skill validation pass, and it now awaits human approval.
- CONVI-7642 is **Done** in Linear. Production [config#153693](https://github.com/cresta/config/pull/153693) and Zebra follow-up [config#153777](https://github.com/cresta/config/pull/153777) are effective in `configv3`: 1,969/1,969 customer scopes across 361 included identities. Scheduled read-back PR #153802 materialized the Zebra profile value, and its five use cases inherit true. Comcast and Schwab remain excluded. Staging remains separately observed at 173/176 effective scopes, with only `customerg/us-west-2` and its two use cases absent; prior QA was accepted. `cresta-testing-ca` remains a separate internal test identity.

## Operational references

- [Global frontend feature-flag rollout practice](sessions/2026-07-29/codex-global-frontend-feature-flag-rollout.md)
- [Example: AI Coach rollout for everyone](https://github.com/cresta/config/pull/150115)
- [Example: production rollout with explicit exclusions](https://github.com/cresta/config/pull/150001)
- [CONVI-7642 rollout state](work-items/CONVI-7642.md)
