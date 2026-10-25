# Shared AI Skills

Durable context for the tool-agnostic skills maintained in `/Users/xuanyu.wang/repos/coaching-qm-skills`.

## Current state

- `record-director-pr-videos` is drafted in `/Users/xuanyu.wang/repos/coaching-qm-skills-director-pr-videos`. It covers comparable full-screen Before/After recordings, clip QA, GitHub inline attachments, live PR-body preservation, and QA metadata verification. The skill and README entry are validated locally and remain uncommitted.
- [CONVI-7803](https://linear.app/cresta/issue/CONVI-7803/add-reusable-interactive-code-review-skill) / [coaching-qm-skills#10](https://github.com/cresta/coaching-qm-skills/pull/10) merged `review-code-change`, a shared skill for standalone interactive HTML code review reports. The PR #33223 construction review was the trial model; the skill merged as `59489d2` after its review finding was addressed.
- Existing customer app DB, auth DB, and ClickHouse connection skills provide credential-safe read-only access.
- The DB skills now distinguish missing AWS profiles from expired sessions, point to single-account `cresta-cli setup-aws-profiles`, and support guarded `cresta-cli --force` plus cache refresh for dedicated accounts omitted from its static list. The Comcast app PG path passed a live read-only `SELECT 1`; the auth DB account resolution and missing-profile paths were validated without an auth DB connection.
- [coaching-qm-skills#8](https://github.com/cresta/coaching-qm-skills/pull/8) contains these DB access skill changes and is open for review.
- [CONVI-7659](https://linear.app/cresta/issue/CONVI-7659/add-reusable-grpc-endpoint-testing-skill) / [coaching-qm-skills#2](https://github.com/cresta/coaching-qm-skills/pull/2) adds `test-grpc-endpoint`, a reusable workflow for deployed gRPC black-box testing with short-lived Cresta auth, independently derived expectations, and scenario-level evidence. Merged as `055310f`.
- [coaching-qm-skills#3](https://github.com/cresta/coaching-qm-skills/pull/3) adds `investigate-analytics-data-issue`, the Performance Insights / Leaderboard data-issue diagnosis workflow distilled from the analytics domain, scorecard-data-sync, and legacy ticket investigations: metric→API→table mapping, PG source-of-truth confirmation, ClickHouse cross-check, and a root-cause catalog with prior cases.
- [coaching-qm-skills#9](https://github.com/cresta/coaching-qm-skills/pull/9) merged `diagnose-active-days`. It routes absent versus zero/N/A metrics, online-session overlap, stale agent identities, source filters, and repair validation, using INSI-4262 and CONVI-7747 as distinct evidence precedents. The PR included only the skill and README entry; it did not run a backfill or change product code.
- [coaching-qm-skills#5](https://github.com/cresta/coaching-qm-skills/pull/5) merged `enable-frontend-feature-flag`. It turns a natural-language customer cohort into an explicit customer manifest, creates a reviewed config PR with ConfigService-equivalent profile/use-case semantics, and treats the post-merge forward sync plus ConfigService-to-`configv3` read-back as the completion boundary. Its first live application generated the CONVI-7642 staging PR for 29 customers and 176 profile/use-case scopes.
  - The updater validates and stages the entire selected manifest before writing, so a missing or malformed later customer cannot leave a partial earlier subset on disk. This review fix is in `a628c689c64f17c458fc813baa5d3a746362a59d`.

## Safety invariants

- Never print or persist bearer tokens, database credentials, refresh tokens, cookies, or credential-bearing URLs.
- Use the narrowest cleared credentials and preserve read-only behavior for customer-data validation.
- Reflection proves contract visibility, not handler deployment; confirm deployment with a valid RPC request.
