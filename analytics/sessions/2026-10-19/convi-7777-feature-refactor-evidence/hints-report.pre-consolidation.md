# Hints extraction evidence

The tests-only branch is `0b494885bc574dfa5e99f54d833bfd76a43c077e` on base `6ba478ae4d7d7322e6f904dce2eb3f4ff2ea58c9`. The refactor branch fast-forwarded that commit before changing production. The frozen test file has SHA256 `5e85448324ce26726b2287dfe3d5bd0df2b732a1a9eb0c88bb56520f16a5e7ea` and its bytes are unchanged between the two worktrees. Baseline completed with 68 passing nodes, zero failures, in 225.989s. The initial extraction run also passed the same 68 nodes in 227.011s; a final run on the consolidated renderer is pass with 68 passing nodes and elapsed 237.526s.

The extraction moves only row selection into `common_hint_usage_clickhouse.go`. Four relations retain behavioral sent, nonbehavioral sent, behavioral followed and knowledge-followed raw rows. `query("", nil)` returns an independently executable raw projection plus metadata CTEs and arguments. The explicit fields include annotation/event IDs, conversation and agent IDs, conversation start, create/update timestamps, policy/behavior IDs, and the applicable action/adherence fields. Every field has an explicit canonical alias so downstream outer queries can resolve it after metadata joins. It deliberately retains physical duplicate row grain; consumers choose their distinct key.

The legacy RPC still owns COUNT DISTINCT, group/time aliases, both UNION ALLs, the MAX sent/conversation/user denominators, the sent-anchored LEFT JOIN and followed SUM. Reader, conversion, policy enrichment/pruning and sorting are unchanged. Both golden suites passed without modifying any SQL snapshot. The group denominator is the hint-selected population; it does not switch to `conversation_d`.

## Observed preserved semantics

- **Mixed sent populations use MAX**: Sent/conversation/user denominators are independent branch MAX values, not summed or union-distinct populations. Observed real baseline and refactor; preserved, not corrected.
- **Moment-based sent and followed can lack delivered actions**: Default sent includes it; both followed paths count it. Action mode excludes it only from sent. Observed real baseline and refactor; no qualifying-use redefinition.
- **Knowledge follows accept empty and unlinked action IDs**: COUNT DISTINCT includes the empty string and IDs with no matching sent action. Observed baseline and refactor.
- **Metadata exclusion depends on nullable joins**: Unmatched IDs are empty strings; both exclusion-only and combined filters return empty. Nullable joins yield intended filtered cohorts. Observed with explicit settings0/1 on local CH23.8.16; production connection settings were not inspected.
- **Action mode loses behavior-filtered populations**: Action table has no behavior column, so the existing false filter removes sent groups. Observed baseline and refactor.
- **Agent attribution can separate sent from followed**: Action-mode recipient group has sent1/followed0; detector-only followed group is dropped by the sent-anchored LEFT JOIN. Observed baseline and refactor.
- **Conversation totals retain grouping multiplicity**: Each agent group counts the conversation once and the response sums to2. Observed baseline and refactor.

## New raw API verification

The supplemental verifier source is `hints-report.raw-smoke.go`; its successful log is `hints-report.raw-smoke.jsonl`. It executes every relation with external user filtering in both sent modes and with metadata include/exclude enabled or disabled. It checks raw count and distinct conversation/agent dimensions, including three annotations sharing two action IDs and two knowledge events sharing one action ID. Seven nodes passed in 46.612s. The first smoke run caught qualified raw output names after metadata joins; canonical aliases fixed it. The failed log is retained as `hints-report.raw-smoke-initial.jsonl`.

The supplemental test was temporary and removed from the repository before final validation; the committed characterization file and all expectations remain frozen. The raw relation now uses the same production renderer as the legacy aggregates, avoiding an otherwise unused private raw-only entry point.

## Limits and exact concerns

These are local CH23.8.16 and PostgreSQL reader results. Production connection settings, deployment state and performance were not inspected. In particular, the metadata exclusion observation is explicitly tied to local `join_use_nulls` settings0/1, not a claim about every production configuration. The raw relations preserve legacy sent/followed populations and must not be treated as the new AssistanceUsage qualification policy. No proto/schema/new API integration or behavior correction is included.

The first development run corrected a fixture resource name and actual exclusion expectations while production was untouched, then encountered a schema-bootstrap `clickhouse-client` exit137. The full baseline and later runs use fresh containers and passed; no global configuration or credentials were changed. No AWS/Okta/Azure/SSH credentials were used. Only local Git commits were made; no push, PR or external communication occurred.

Exact commands, hashes, per-stage node results, scenario evidence and the raw field lists are in `hints-report.json`. Baseline and final logs are preserved alongside this report.
