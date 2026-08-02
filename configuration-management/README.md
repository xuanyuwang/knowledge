# Configuration Management

## Scope

Customer configuration schemas, Config Admin editing workflows, GitOps review, synchronization to ConfigService, and safe feature-flag rollout practices.

## Current understanding

- `customer_config_history/` in the `config` repository is the reviewed source of truth for active customer configuration.
- Cresta Admin / Tenant Admin provides bulk feature-flag editing and creates a config PR for review instead of mutating production directly.
- A Director flag intended for all production customers is materialized at every selected profile and use-case. There is no single wildcard production default in the current bulk workflow.
- The normal application level is **Auto Inheritance** (`LEVEL_ALL`), which applies the flag to selected profiles and their underlying use cases.
- PR CI runs schema/YAML validation and a diff-only ConfigService sync. Merging to `master` performs the real multi-region sync.

## Operational references

- [Global frontend feature-flag rollout practice](sessions/2026-07-29/codex-global-frontend-feature-flag-rollout.md)
- [Example: AI Coach rollout for everyone](https://github.com/cresta/config/pull/150115)
- [Example: production rollout with explicit exclusions](https://github.com/cresta/config/pull/150001)
