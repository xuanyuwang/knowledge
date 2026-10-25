# CONVI-7777: non-time filter follow-up

**Local implementation follow-up:** Feature source attribute forwarding is implemented locally; see [implementation, verification and compatibility limits](convi-7777-attribute-propagation-fix.md). This source audit retains its pre-change findings.

This is a read-only source comparison on October 21, 2026. The implementation inspected is the local xw/convi-7777-population-api worktree at /Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage. No product changes, test execution, publication or deployment were performed for this follow-up. Existing test bodies were inspected; prior passing runs do not establish a complete filter-by-feature comparison.

## Result

The new API resolves request eligibility once through the shared ConversationStats population, then intersects every assistance family with eligible conversation/agent pairs. Its evidence builders receive requested time bounds and selected agents; other conversation attributes are enforced by population membership. Existing feature RPCs pass request attributes to their individual source builders. These architectures preserve the new metric's numerator/denominator relationship, but a shared parser does not prove all legacy filter behavior is identical.

| Filter | Current new API path | Verification and parity limit |
|---|---|---|
| Users, teams/groups, direct membership, peer access and ACL | ParseUserFilterForAnalytics, then resolved users constrain population; exact pairs constrain assistance | Same shared resolver as feature RPCs. New tests cover ACL, selected/empty audience and user transport. Full resolver option cross-product is not independently tested here. |
| Agents only and exclude deactivated users | Resolved user set; forces restricted audience and external-table transport | Dedicated real-ClickHouse regressions exist for small, large and empty audiences. |
| Use case and language | Shared parser filters message rows, then same-agent conversation intersection | Same population implementation as ConversationStats; existing population test covers these separately and combined. Legacy feature RPCs also filter their own copied source fields. No claim of strict row-by-row feature parity. |
| Duration buckets/range | Shared parser filters message conversation_duration_secs | Existing shared-population range and combined test cases; duration bucket lowering is shared. Legacy Events event tables lack duration and skip that predicate there, while Notes/Workflow conversation subquery still receives duration. |
| Voicemail exclusion | Shared parser lowers explicit criterion or recognized voicemail exclusion into message is_voice_mail predicate | Existing shared-population message-filter tests cover explicit criterion. No complete feature-by-filter execution matrix. |
| Metadata include, exclude, selected-value-or-missing | Population owns conversation membership; flag and stale-value option are preserved | Existing population tests cover selected/missing/latest/stale, groups and flag. Notes/Workflow source/payload branch now matches legacy request shapes. Source-branch tests use supplied eligible pairs and do not prove metadata membership end to end across all families. |
| Agent Assist ON/OFF | Shared buildAgentAssistFiltersSubQuery | Preserves ConversationStats branches and label semantics. Existing population tests cover ON/OFF, bots, metadata and legacy/per-message labels. ON describes assistance availability/labels, not proof that a feature was used. |
| Source, bot, developer and nonempty-agent gates | Shared population plus feature-specific guards from reused row builders | GenAI retains source 0/8, developer exclusion and nonempty-agent rules; Hints retain their source/bot/null and policy-source rules. Summary/Notes/KB/Workflow/Suggestions do not gain new feature-source restrictions beyond their reused builders and population. Population default/OFF and ON source requirements differ as in ConversationStats. IncludeCrestaBot can broaden Hint evidence but cannot resurrect a pair excluded by population bot policy. |
| Behaviors, attribute policies, direct moment IDs, hint type | Generic Attribute is passed to population parser targeting messageTable | Unresolved API support gap: parser emits behavior_id, policy_id, moment_template_id or action_type/adherence_type predicates for message_d, absent from the checked-in schema. Handler has no explicit filter allowlist rejecting these options. This is source/schema evidence; no new runtime reproduction was run. Existing feature APIs apply these selectors to annotations/events, sometimes using false for unsupported columns. No strict parity or support claim. |
| Other generic Attribute fields | Only fields explicitly consumed by the resolver/population parser have effect | Presence in the generic proto does not imply support. No exhaustive allowlist or cross-product test exists for this endpoint. |

## Current Assistance Insights caller

The inspected Director AssistanceInsightsContainer constructs usersTeamsGroups, metadata moment groups, duration buckets and excludeDeactivatedUsers, with filterToAgentsOnly as an option. The common attribute hook can add the current use case when the use-case configuration requires it. The main container does not explicitly supply behavior, policy, direct moment-ID or hint-type selectors for the new overall metric. Thus the unsupported selector issue is an API-contract gap; this inspection does not establish a current page failure.

## Concrete interpretation

With a duration filter of at least 60 seconds, a 90-second eligible conversation can contribute to both total and assisted counts; a 30-second conversation contributes to neither, regardless of assistance events. Metadata and selected agents work through the same eligibility principle. This does not require duplicating those predicates into every evidence query.

A behavior/policy/hint selector is different: it can mean selecting the assistance record itself. It cannot be declared supported merely because the request attribute reached the population parser. Decide whether these options are outside this metric's contract and reject them clearly, or implement explicit supported semantics and regression coverage. Do not silently add those semantics during the time/source fix.

The user explicitly accepted differences caused by inconsistent stored conversation-start copies and does not want special handling or legacy bucket reproduction for that case. That concern is outside this follow-up. Existing metadata exclusion defects and other intentional inherited population behavior should also be distinguished from missing filter propagation; compatibility does not require copying an accidental defect.

## Source pointers

- Backend action_retrieve_conversation_assistance_usage_stats.go: readConversationAssistanceUsagePopulation and validateConversationAssistanceUsageRequest.
- Backend common_conversation_population_clickhouse.go: buildConversationPopulationQuery, buildAgentAssistFiltersSubQuery, buildConversationPopulationMomentFilterQuery and buildConversationPopulationSelect.
- Backend common_clickhouse.go: parseClickhouseFilter, tableColumnNameMapping, generateMomentSubQuery and generateExcludeMomentSubQuery.
- Backend action_retrieve_conversation_assistance_usage_stats_clickhouse.go: conversationAssistanceUsageEvidenceQueries and exact pair intersection.
- Backend retrieve_conversation_stats_test.go: TestMessageAttributeFilters, TestMetadataSelectedMissingLatestAndStale, Agent Assist population cases.
- Backend action_retrieve_conversation_assistance_usage_stats_test.go: ACL/user restrictions, bot policy, evidence source-branch matrix.
- Backend shared/clickhouse/testing/schemas/ch_conv_schema.sql: message/message_d schema.
- Director AssistanceInsightsContainer.tsx and hooks/insights/useFilterByAttributes.ts: current page inputs and implicit use-case selection.
