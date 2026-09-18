# Shared AI Skills

Durable context for the tool-agnostic skills maintained in `/Users/xuanyu.wang/repos/coaching-qm-skills`.

## Current state

- Existing customer app DB, auth DB, and ClickHouse connection skills provide credential-safe read-only access.
- [CONVI-7659](https://linear.app/cresta/issue/CONVI-7659/add-reusable-grpc-endpoint-testing-skill) / [coaching-qm-skills#2](https://github.com/cresta/coaching-qm-skills/pull/2) adds `test-grpc-endpoint`, a reusable workflow for deployed gRPC black-box testing with short-lived Cresta auth, independently derived expectations, and scenario-level evidence. Merged as `055310f`.
- [coaching-qm-skills#3](https://github.com/cresta/coaching-qm-skills/pull/3) adds `investigate-analytics-data-issue`, the Performance Insights / Leaderboard data-issue diagnosis workflow distilled from the analytics domain, scorecard-data-sync, and legacy ticket investigations: metric→API→table mapping, PG source-of-truth confirmation, ClickHouse cross-check, and a root-cause catalog with prior cases.
- [coaching-qm-skills#5](https://github.com/cresta/coaching-qm-skills/pull/5) merged `enable-frontend-feature-flag`. It turns a natural-language customer cohort into an explicit customer manifest, creates a reviewed config PR with ConfigService-equivalent profile/use-case semantics, and treats the post-merge forward sync plus ConfigService-to-`configv3` read-back as the completion boundary. Its first live application generated the CONVI-7642 staging PR for 29 customers and 176 profile/use-case scopes.
  - The updater validates and stages the entire selected manifest before writing, so a missing or malformed later customer cannot leave a partial earlier subset on disk. This review fix is in `a628c689c64f17c458fc813baa5d3a746362a59d`.

## Safety invariants

- Never print or persist bearer tokens, database credentials, refresh tokens, cookies, or credential-bearing URLs.
- Use the narrowest cleared credentials and preserve read-only behavior for customer-data validation.
- Reflection proves contract visibility, not handler deployment; confirm deployment with a valid RPC request.
