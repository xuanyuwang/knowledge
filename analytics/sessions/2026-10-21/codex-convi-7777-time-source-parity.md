# CONVI-7777 requested-range/source parity and explanation follow-up

- Date: October 21, 2026, America/Toronto.
- Primary domain: Analytics; primary subdomain: Assistance Insights.
- Source repo: /Users/xuanyu.wang/repos/go-servers.
- Branch/worktree context: xw/convi-7777-population-api at /Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage; starting checkout clean. Local uncommitted changes only.
- Canonical work item: [CONVI-7777](../../work-items/CONVI-7777.md).
- Explanation and local-change details: [follow-up](../../deliverables/convi-7777-time-source-parity-follow-up.md).
- Reproduction and patch: [evidence directory](convi-7777-time-source-parity/).

## Request and action

Xuanyu asked whether an if/else or equivalent could preserve the legacy conditional source selection, agreed the requested-range change was straightforward, and requested concrete explanations of Summary matching, bucket attribution, latest selection, message matching and fractional boundaries. Implemented requested [S,E) bounds for all evidence source types. Notes/Workflow choose Events versus Conversations typed projections according to the legacy recognized metadata filter shape, including exclusion flag behavior. Metadata eligibility remains in the shared population, with the final conversation/agent intersection retained. Summary's exact three-key join was already present and is unchanged. Shared population/version/precision behavior is unchanged.

A small shared Workflow builder signature change makes its projection choice explicit; its legacy caller supplies the same derived boolean as before. No event-enum or payload-contract correction is folded into this change. The package guide was updated. No commit, push, PR update, external message or deployment performed.

## Verification and caveats

New tests failed on the original implementation: fourteen out-of-range boundary cases and four metadata source cases. Full API Go tests passed in 38.912s after the fix. Expanded source tests cover no metadata, include, exclusion, and value-or-missing request shapes, with data present in only one cluster. Full RPC tests cover start-inclusive/end-exclusive boundaries for every family; source tests isolate a single family's real SQL/connection using a supplied eligible pair to avoid overlapping synthetic event-type predicates in the combined union.

The latest API Bazel target passes in 39.6s (54.398s build/test command). gofmt, Gazelle and git diff --check pass. The package excludes test files from normal Gazelle discovery; generated dependencies were obtained with a temporary scratch package containing the test file and merged into this existing target. No new exclusion directive or handwritten dependency labels were introduced. The final combined Go run passed the expanded API tests and legacy Notes/Workflow suites in 221.577s: 223 passing nodes, zero failures and zero skips. Saved final-go.jsonl and final-go-summary.json alongside the red/green and Bazel logs. The fresh testcontainers use ClickHouse 23.8.16; production execution remains unverified.

The first attempts encountered local Docker discovery and an unhealthy shared reusable ClickHouse schema setup. Tests then used the configured Colima Docker host with fresh testcontainers. The shared container was left untouched. No production data or cloud/SSH credentials were used.
