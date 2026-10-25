# CONVI-7777 backend Step 1 — shared eligible population

- Date: 2026-10-16, America/Toronto.
- Primary domain/subdomain: analytics / assistance-insights.
- Source repo: `/Users/xuanyu.wang/repos/go-servers`.
- Execution worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage`.
- Branch/base: `xw/convi-7777-step-1-population`, `40faeac75f2d55b06a4e3e29ac64fe782b8abd99`; Go and Bazel pin cresta-proto v2.32.18.
- Authorization: user requested Step 1 implementation, then a report update and review pause. No later implementation, commit or PR is authorized by this checkpoint.

## Result and contract

Implemented the approved shared builder in `insights-server/internal/analyticsimpl/common_conversation_population_clickhouse.go`. `conversationPopulationInputs` accepts resolved filters, time range/field, bot policy, stale-metadata policy and external-user-table configuration. `conversationPopulationQuery` bundles a complete SELECT, ordered arguments and external tables. The relation emits one eligible `(conversation_id, agent_user_id)` row with selected conversation/label time under `conversation_time` and matching-message `MAX(handle_time_secs)` under `aht_sec`.

Moved AA selection and metadata eligibility, including inclusion/exclusion and selected-or-missing logic, out of the ConversationStats reader. Empty metadata lists generate no metadata CTEs or joins. The common builder selects and intersects conversations/messages before final aggregation. It preserves filters before latest matching version selection, conversation-ID-only metadata joins, the existing source/developer differences among default/ON/OFF label branches and argument ordering.

ConversationStats now wraps the shared SELECT with its existing distinct conversation/user counts and handle-time sum. Existing frequency/timezone/week expressions are retained with the selected-time alias substituted. The handler’s ACL/user resolution, empty-selection guard, caching, tenant/profile connection selection, response conversion and Team aggregation are unchanged. Tests execute a key-only projection without adding an unused production AssistanceUsage reader. Public AssistanceUsage RPC orchestration and feature queries are deferred.

## Verification

The focused metadata parser test passed before implementation. The first broader baseline invocation could not discover Colima’s Docker socket. Started the existing Colima instance and used `DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock` for real ClickHouse runs.

The four-entry-point focused run passed, with 45 named test nodes, zero failures, and elapsed 73.107s:

```sh
DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock go test ./insights-server/internal/analyticsimpl -run '^(TestConversationPopulationCH|TestRetrieveConversationStats|TestRetrieveConversationStatsValueOrMissingMetadataFilter|TestConversationStatsConversationVersionCH)$' -count=1 -json
```

The new real-ClickHouse suite covers shared conversation pairs; AHT 20 from matching 10/20 with excluded voicemail 90; one distinct conversation/two users/AHT sum 35; missing-message and message-only exclusion; inclusive lower/exclusive upper boundaries; filtering before latest matching conversation selection; inline and external user tables; conversation-scoped metadata; stale/latest metadata; selected/missing/selected-or-missing; default/ON/OFF AA policies; bot/source/developer distinctions; and mixed false/true label records. After the focused pass, strengthened the user-filter fixture with an otherwise eligible Charlie participation outside the selected Alice/Bob users and reran the new suite. The strengthened suite passed in 44.409s with zero failures, using `-run '^TestConversationPopulationCH$' -count=1 -json`.

The initial new-test failures were test fixture/scanner mistakes, corrected before the passing run: row MAX is Int32, Select requires struct destinations, the fixture must explicitly request voicemail exclusion, and conversation_with_labels has no source/developer columns. No production semantic change was needed to fix those failures. Existing versioned end-time daily/HST integration cases passed, as did unchanged response expectations and 12 regenerated SQL goldens. Golden formatting used sql-formatter’s ClickHouse dialect; temporary snapshot-writing test code was removed.

`bazel run //:gazelle` passed. The package has an existing `gazelle:exclude *_test.go` convention. Used temporary explicit excludes and Gazelle’s file-test mode to generate the new focused `common_conversation_population_clickhouse_test` rule, restored the existing exclusion convention and reran Gazelle. The final generated target remains; unrelated generated BUILD changes were restored. The generated Bazel target compiled and passed in 44.1s (45.565s total invocation) with fresh test containers. Initial invocations inherited the user’s reusable-container setting and failed during local readiness, then ZooKeeper schema setup; these were environment failures. Supplying an empty reusable-container name used the same independently bootstrapped test-harness path as the passing Go run. No source/config change was needed.

Independent read-only semantic review found no blocking issue or demonstrated regression. Its suggested shared-conversation aggregate assertion was added. A dedicated weekly runtime fixture remains a non-blocking coverage extension; existing parser/source inspection supports preservation. No production data, latency or load benchmark was collected.

The interactive report contains actual source excerpts, summarized before/after changes, the complete Go diff, scenario evidence and the review stop. All five topics, both code views, Previous/Next and boundary disabled states were exercised. Visually checked the default viewport and a 390px viewport; fixed long-source-name overflow and confirmed body width equals viewport width. Report JavaScript syntax and source diff whitespace checks pass.

## Branch and retained work

The superseded draft was preserved with `git stash push --include-untracked` before the fresh branch. Stash commit: `4dc871101b1a0c6ba4d19ffab69c2be652abb321`. It was not applied to this implementation. Branch preparation stays outside the numbered construction report. Code remains uncommitted for review; no PR created. Step 1 remains the first proposed PR in the stack. Step 2 feature-family PR boundaries and combining Steps 3/4 remain subject to actual code size.

## Credentials and follow-up

No AWS, Okta, Azure or SSH credential/profile was explicitly selected, read or transmitted. Git/Bazel used their normal self-authentication; local Docker/ClickHouse fixtures use the test harness.

Next action: review Step 1 with the user, resolve questions, and wait for acceptance before Step 2. [Report](../../deliverables/convi-7777-assistance-usage-construction-review.html) · [Contract](../../deliverables/convi-7777-backend-step-one-contract.md) · [Work item](../../work-items/CONVI-7777.md).

Final Bazel command:

```sh
bazel test //insights-server/internal/analyticsimpl:common_conversation_population_clickhouse_test --test_env=DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock --test_env=REUSABLE_CLICKHOUSE_CONTAINER_NAME= --test_output=errors
```

Final outcome: focused Go entry points pass; strengthened population suite passes; generated Bazel target passes; Gazelle and source/report whitespace and JavaScript checks pass. Ready for user review; paused before Step 2.

## Review explanation refinement

User requested a code-level operation summary before the detailed walkthrough. Added a visible three-operation summary to the report: move existing selection and metadata logic into the shared file; return eligible pair/time/handle-time rows before aggregation; wrap those rows with ConversationStats counts, sums and grouping. This clarifies extraction versus the changed query boundary. Product code and the Step 1 review checkpoint remain unchanged.

## Detailed code editing walkthrough

Continued Step 1 review in the original session at the user’s request. Re-read the report, approved contract, branch/base and live worktree; the existing implementation is uncommitted on `xw/convi-7777-step-1-population` at base `40faeac75f2d55b06a4e3e29ac64fe782b8abd99`. Expanded the existing construction report from five conceptual views into nine concrete edits: add input/result contracts; move AA selection unchanged; move/rename metadata helpers; return population rows; assemble the shared query; restore the existing aggregate wrapper; wire the reader; verify rows/aggregates; update parser callers, SQL fixtures and generated BUILD registration. Each view has current-state/problem/change/reason/result text, actual source excerpts, a comparison against the pinned base and current file/line locations. A static edit map remains readable without JavaScript. The order is reconstructed for source reading, not presented as historical commits.

Compared the moved `buildAgentAssistFiltersSubQuery` body byte-for-byte against the pinned base: unchanged. JavaScript syntax, HTML ID uniqueness, all nine complete topic records and script escaping passed. Browser exercised all nine navigation buttons, both code views, Previous/Next and disabled first/last boundaries; default-viewport screenshot inspected. A SHA-256 of the backend tracked diff plus both new Go files matched before and after the report update. No product code changed and no backend tests were rerun; earlier Go/Bazel results remain labeled as implementation-session evidence. No AWS/Okta/Azure/SSH credential or profile was selected/read/used in this follow-up. Step 1 remains awaiting user review; no Step 2, commit or PR action was taken.


## Approved commit and population-only RPC

The user accepted Step 1 and authorized commit, followed by the conversation-population part of the new API, brief one-line inline TODOs for following PRs, and direct PR creation without another construction walkthrough. Committed extraction as `a9eeda0e75` and opened [#33992](https://github.com/cresta/go-servers/pull/33992) against main. Fresh extraction regression tests passed (74.516s); Gazelle and diff checks passed.

Created `xw/convi-7777-population-api` on the extraction commit. Implemented `RetrieveConversationAssistanceUsageStats` population orchestration in one action file: clone filters, resolve existing user/ACL scope, skip empty audiences, resolve bot policy, execute the shared population query and scan conversation/agent/start-time keys. Query/scan/iteration failures return errors and discard partial rows. The protobuf contract requires complete feature-use counts before success, so the handler returns `Unimplemented` after the read. Three single-line TODOs cover feature evidence, overall/Agent/Team totals and start-time buckets, and opt-in details. No feature evidence, aggregation, details, caching or Director changes.

Verified a failing invalid-parent test before implementation, then passing focused tests. New same-file lightweight test suites avoid the legacy PostgreSQL harness and are registered through Gazelle in a standalone target. Final Go tests passed (17.318s), covering real ClickHouse ACL restriction, start-time interval, message-bearing population, shared-agent pairs, both user-filter modes, bot policy, request preservation, empty audiences and error handling. `bazel test //insights-server/internal/analyticsimpl:action_retrieve_conversation_assistance_usage_stats_test` passed in 17.6s with Colima Docker and fresh ClickHouse containers. Gofmt, Gazelle and diff checks passed. Independent review confirmed no important remaining issues; its earlier Bazel support-file concern was resolved with the standalone test harness.

Committed as `73da2b45e3` and opened [#33993](https://github.com/cresta/go-servers/pull/33993) against `xw/convi-7777-step-1-population`; both PRs are attached to this chat. Code worktree is clean; remote checks are pending. No staging or production verification performed. Git and gh used their existing authentication; no explicit AWS/Okta/Azure/SSH credential or profile was selected, read or used.
