# CONVI-7852 backend membership roles

Source repo: /Users/xuanyu.wang/repos/go-servers

Producer worktree: /Users/xuanyu.wang/repos/go-servers-convi-7852-auth-roles, branch xw/convi-7852-auth-membership-roles, base 18d5d0acf7.

User requests separate PRs after merged cresta-proto #10550. Contract: direct legacy roles only; custom roles deferred; indirect roles remain empty.

## Plan

1. Upgrade proto to v2.34.0; test and populate roles from the existing auth membership view query. Version the cache key before hashing.
2. Create a separate analytics branch atop the producer. Preserve filtered user identities, query inputs, group mappings, metrics and Team/Manager aggregation. Decorate per-agent conversation response users with direct memberships intersected with their existing candidate teams, including stats-cache hits.
3. Run focused regression tests, regenerate Bazel files and dependency metadata, review each diff, then publish two PRs and record validation.

The Director prototype is untouched. Selecting the preferred display team remains frontend follow-up.

Analytics worktree: /Users/xuanyu.wang/repos/go-servers-convi-7852-analytics-roles, branch xw/convi-7852-analytics-membership-roles. Will stack onto auth commit before publishing.

Auth regressions failed as expected for missing roles and unchanged raw/hashed keys. Initial build required Gazelle for new test imports; no clean baseline claim.

## Producer PR

https://github.com/cresta/go-servers/pull/35092, commit 6a727f919d. Full //auth/internal/service/nonpublic/user:user_test passed against local Postgres (15.3 seconds test time). Tests cover distinct direct Agent/Manager 2nd assignments, an indirect row to the same parent team, malformed/out-of-range legacy strings, unknown numeric enum values, JSON Redis round-trip/cache-hit, and raw/hashed version isolation. Existing eligibility, pagination and cache suite passed. go mod tidy, Bazel Gazelle/mod tidy and focused converter generation completed; no extra generated converter changes.

Consumer retains eligible users' membership metadata separately from FinalUsers and applies it only to Agent+Group conversation response users, intersected with each row's candidate teams. Stats caching stores metric responses before decoration; cache hits are decorated using current eligible membership data. Group order and Team rollups remain unchanged. Local ClickHouse/ZooKeeper containers started using the repository script for regression tests.

The analytics package's existing wildcard test exclusion prevents Gazelle from updating test dependencies directly. Generated the affected test dependency lists with temporary per-file Gazelle mode and merged them into the existing targets, preserving their fixture srcs/runfiles. Temporary directives and unrelated generated BUILD changes are removed.

## Consumer validation

Full Bazel targets common_user_filter_test (1.7s) and retrieve_conversation_stats_test (258.9s) passed against local Postgres plus the reusable ClickHouse container. Initial filtered invocation used the wrong suite entry name and executed no retrieval methods; corrected to TestRetrieveConversationStats and confirmed the new cache/direct-membership/missing-metadata regressions fail before enrichment. The fresh fixture also required a message row because the existing conversation population is message-based; used the established seedPair helper. Final fixture simplification shares seeded and expected per-team roles; rerun the five affected retrieval methods after it.

CI registry selected only insights-server with the Bazel backend for the consumer diff. Invoked via go run github.com/magefile/mage because mage is not installed on PATH. Production helper clones response and role arrays; only Agent+Group rows are decorated, after metrics cache storage. No query inputs, membership mappings, group order, or Team aggregation logic changes.

Git and gh authenticated themselves. No AWS profile, Okta/Azure credential, or SSH key was explicitly selected or inspected. No production operations or Slack/email messages.

## Published result

Auth: https://github.com/cresta/go-servers/pull/35092, commit 6a727f919dd718ccbb801a6419c9c49e72633e51, branch xw/convi-7852-auth-membership-roles, base main.

Analytics: https://github.com/cresta/go-servers/pull/35094, commit 9fada718de4ba4bd7d8b9682640a2e94de61624e, branch xw/convi-7852-analytics-membership-roles, base xw/convi-7852-auth-membership-roles. Verified GitHub lists exactly one consumer commit and six analytics-only files.

Final five retrieval methods passed (8.1s) after fixture simplification and the duplicate indirect-to-same-team case. Source worktrees are clean. Auth automated checks green, pending code-owner approval; consumer service CI waits for retargeting to main. Both PRs attached to this chat. No Director changes, staging/customer deployment verification, or performance measurements. Review/merge producer before consumer; retarget consumer after the producer merges.

Verified `.github/workflows/ci_generated.yaml` limits pull_request service CI to main and release/**. Stacked #35094 currently has triage/review checks only; retarget to main after producer merge to run service CI. This is reported in its PR body and handoff.
