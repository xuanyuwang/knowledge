# CONVI-7777 backend API implementation

- Date: 2026-10-20 (America/Toronto).
- Primary domain/subdomain: analytics / assistance-insights.
- Authorization: check prerequisites; rebase #34044 onto latest fetched main; unstack #33993 from #34002; implement the new API locally on #34044; deliver a construction report. No merge requested.
- API worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage`, branch `xw/convi-7777-population-api`.
- Refactor worktree: `/Users/xuanyu.wang/.codex/worktrees/convi-7777-summary-genai-refactor/go-servers`, branch `xw/convi-7777-summary-genai-refactor`.

## Approved polish and publication

The user approved the report and authorized polishing and pushing. Published commit `f383b394c48e8eb45505d6c7ff5b275816141674` to [PR #33993](https://github.com/cresta/go-servers/pull/33993), still based on #34044. The API worktree is clean; neither PR was merged. Rewrote the PR title/body around the completed backend counts behavior and current dependency, using the repository template.

Polish: descriptive conversation-set names, typed slices.SortFunc, isolated bucket-boundary calculation, readable Summary SQL, complete proto comparison for the evidence-union RPC test, and Given/When/Then spacing. No feature predicates, counting grain, timezone semantics or scope changed.

Fresh post-polish validation: Gazelle and gofmt pass; full API Bazel target passes36.1s; focused race suite49 named nodes passes50.959s; git diff --check passes. [Final Bazel evidence](convi-7777-api-evidence/polish-bazel.txt), [race outcomes](convi-7777-api-evidence/polish-race-results.json), [published diff](convi-7777-api-evidence/published-implementation.patch). #34044 CI is now green (required checks passed); checks for the newly pushed implementation commit are starting. Original pending-CI and uncommitted descriptions below record the earlier checkpoint and are superseded by this section.

## Prerequisites and topology

Fetched main is `384cd3fda9c429c418a860c79a7352c618a8daf9`. It has the shared population plus KB, Suggestions, Hints, Notes and Workflow row helpers. #34044 provides the final Summary/GenAI helpers. The rebased Go module pins cresta-proto v2.32.42 and contains the dedicated RPC/results. Existing user/ACL/Team resolution and timezone/week helpers supply the remaining infrastructure.

Rebased #34044 from `2f888a26e202c40205876bc9e9cd481aa13525e3` to `e40609a8aaa01a034fa448d9b9870eb3cb95d12f`. Resolved a BUILD source-list conflict by retaining all helpers, regenerated Gazelle and reran both legacy suites. Rebased only the two API commits above the already-main extraction onto #34044, producing `d3d36443692d164c5da809a05c62d01f6136a5d6`. Both committed branch heads were pushed atomically with explicit leases. Backups remain in `xw/convi-7777-summary-genai-before-api-20261020` and `xw/convi-7777-population-api-before-20261020`.

`gh stack unstack 34002` detaches all unmerged members: #33993 and #33992 are standalone; the old stack retains merged #34001. GitHub rejected selective base changes while native stack membership existed. Retargeted #33993 to `xw/convi-7777-summary-genai-refactor`; #33992 retains main. Verified remote heads, bases, #33993 stack=null, #34044 merged=false and stack #34002 membership. Local CLI tracking is stale: do not run stack submit/sync/rebase without inspecting topology. Neither PR was merged. The new API implementation is local and uncommitted; the pushed #33993 head remains the rebased population checkpoint.

## Code operations and resulting behavior

1. The population reader returns its resolved user/Team mappings and bot policy alongside eligible conversation/agent/start-time rows. It clones attributes, applies existing ACL/cohort logic, and preserves explicit agent-only/inactive-user restrictions when shared all-users optimization would otherwise discard them. Empty restricted audiences skip reads.
2. A new evidence adapter reuses all seven shared row-query families. Each query receives its own external eligible-pair table; population metadata/time filters run once. Evidence joins the population by conversation/agent keys, while existing feature-internal predicates remain intact. Notes/Workflow use the events connection.
3. Run at most three evidence reads concurrently, union pairs after every required read succeeds, and fail the whole RPC on connection/query/scan/iteration failures. Repeated events or multiple features never inflate adoption.
4. Intersect with population and aggregate distinct conversations overall/per agent. Team counts sum member-agent counts, including shared conversations and overlapping membership. Top-level counts stay distinct regardless of grouping. DAILY/WEEKLY occupied buckets use population start time and existing metadata semantics.
5. Validate parent/time/frequency/grouping/week/timezone requests. Conversation details return Unimplemented explicitly; no Director or details implementation was included.

## Validation

- TDD: population-only numerator test failed before implementation; explicit agent/activity filter regressions failed against the inherited optimization and passed after the endpoint fix.
- Full API Go suite: 90 named pass nodes, zero failures, 36.195s. Real ClickHouse covers all ten categories, exclusions, stale copied KB start time, duplicates, wrong-agent evidence, Summary type2/type3 and time matching, GenAI source/session limits, cohort/message/bot/ACL/user filtering. Agent/Team overlap and daily/weekly buckets have full proto aggregation expectations.
- Focused race check: 49 named pass nodes, zero failures, 51.068s; includes mock concurrent union, required failures and aggregation/validation. This is not a full-package race claim.
- API Bazel target: passed 37.3s after Gazelle generated both new source entries and direct test dependencies. First attempt exposed missing direct dependencies because the legacy package excludes tests from normal Gazelle. Generated the API target with temporary explicit test exclusions plus `gazelle:go_test file`, restored the existing wildcard directive, then reran normal Gazelle successfully.
- Rebased #34044 Bazel legacy suites: Knowledge Assist passed272.8s; Summary passed267.3s. Build elapsed752.139s including first-worktree compilation.
- Commands used `DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock` and empty `REUSABLE_CLICKHOUSE_CONTAINER_NAME` to create fresh isolated ClickHouse containers. No production data or benchmark was queried.
- Independent reviewer inspected implementation and subsequent fixes; no remaining blocker within the preserved-feature scope. The reviewer did not independently rerun tests.
- HTML construction controls verified for all six steps, both code views, Previous/Next and narrow layout; JavaScript syntax and local source paths checked. [Report](../../deliverables/convi-7777-api-implementation-review.html).
- Machine-readable outcomes, final Bazel tails and complete local patch: [evidence](convi-7777-api-evidence/summary.json), [patch](convi-7777-api-evidence/implementation.patch).

## Post-push CI

Post-rebase GitHub CI hit proxy.golang.org dependency download/TLS timeouts in Detect affected services (run39508790631). Failed jobs were rerun; CI retry is pending. Local regression passes remain valid, but this is not a green-CI or merge-readiness claim. [Run](https://github.com/cresta/go-servers/actions/runs/39508790631). No product change was made for the network failure.

## Review limits and follow-up

Feature predicates remain authoritative: GenAI source0/8; Summary valid generation plus copy datatype1/2 joined by conversation/start/agent; Notes CopyEntity only; Workflow existing document/session semantics. This is not a cleanup of existing feature analytics defects. Buckets preserve the current whole-hour fixed timezone offset; historical DST/fractional-hour behavior remains inherited. Sparse buckets omit empty intervals.

Each evidence query transfers the full eligible-pair cohort and has no timestamp partition bounds. Correct pair filtering and concurrency three do not prove production latency, read volume or memory. Benchmark representative cohorts before rollout; timing instrumentation remains follow-up. Details requests fail explicitly while details are deferred. A separate session implemented a local Director column; this backend turn did not modify that work or validate deployed integration.

Review the local counts diff/report, then decide whether to commit/push it into #33993 and approve/merge #34044. No new implementation commit or merge was made. No AWS, Okta, Azure or SSH credential entry was selected or read; git, gh and Bazel used their normal authentication.
