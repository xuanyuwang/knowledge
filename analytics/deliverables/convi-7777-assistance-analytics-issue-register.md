# CONVI-7777: Assistance analytics issues and PM decisions

Draft for PM review · Updated 22 October 2026 · Priorities remain proposed; AA-16 records an approved decision to defer performance optimization until benchmarking.

## Purpose and scope

This register captures existing correctness issues and metric inconsistencies found while preparing the Assistance Usage API. It consolidates all 19 observations in the “Preserved correctness issues and quirks” sections of the five feature construction reports into 13 entries, then adds two related population/metric-contract entries. AA-16 adds a performance follow-up for the new API. Duplicate observations share one entry. Keep these IDs stable as investigation and decisions progress.

The refactors deliberately preserve existing RPC behavior so that extraction and behavior corrections can be reviewed separately. Preserved behavior is not automatically the desired specification for the new API. Existing event, action, and session totals also cannot establish the percentage of distinct conversations with assistance use.

Evidence below comes from the pinned source and recorded before/after tests in local ClickHouse environments. Tests were not rerun to assemble this register. Customer prevalence, deployment status, and performance impact have not been measured. AA-01 now includes a narrowly scoped live production connection check; other entries remain based on local evidence. “Impact” describes the consequence when the recorded trigger occurs, not confirmed customer impact.

## PM review summary

| ID | Topic | Classification | Proposed next step |
| --- | --- | --- | --- |
| AA-01 | Metadata exclusions discard valid rows | Reproduced defect, setting-dependent | Engineering fix; PM prioritizes customer impact after prevalence check |
| AA-02 | Source and developer eligibility varies by feature | Product contract decision | Agree one new-API population and any legacy alignment scope |
| AA-03 | Agent Assist ON has different population requirements | Preserved eligibility inconsistency | Investigate prevalence and decide whether to align branches |
| AA-04 | Hint totals use independent branch maxima | Aggregation inconsistency | Choose intended combined population/count semantics |
| AA-05 | Hint adherence can count without delivered evidence | Use-evidence decision | Confirm delivered-action requirement for qualifying use |
| AA-06 | KB hint follows count empty or unlinked action IDs | Evidence integrity issue | Define handling of invalid links and historical records |
| AA-07 | Behavior filtering removes action-mode hints | Reproduced filter limitation | Choose supported behavior-filter contract |
| AA-08 | Hint delivery and adherence credit different agents | Attribution decision | Choose which agent receives use credit |
| AA-09 | Grouped totals repeat shared conversations | Counting contract decision | Document distinct overall versus grouped totals |
| AA-10 | Hint per-user averages become zero without frequency | Reproduced response inconsistency | Engineering correction after denominator agreement |
| AA-11 | Guided Workflow accepts empty session IDs | Use-evidence decision | Decide whether document-use evidence requires a session |
| AA-12 | Ungrouped Notes/Workflow expose a year-1 timestamp | Response contract issue | Decide whether unbucketed rows should omit time |
| AA-13 | GenAI averages differ by response level | Denominator contract issue | Define one meaning for the displayed per-user average |
| AA-14 | GenAI usage without matching messages disappears | Population/evidence decision | Decide when annotation-only evidence can qualify |
| AA-15 | Legacy feature denominators differ from the new cohort | New-API metric contract | Use one eligible population and distinct conversation membership |
| AA-16 | Assistance Usage population transfer and memory | Performance follow-up; no demonstrated production defect | Benchmark representative large requests, then optimize measured bottlenecks |

## Detailed register

### AA-01 — Metadata exclusions discard rows that should survive

**Affected:** KB Search, Chat Suggestions, Hints, Summary, GenAI, Notes, and Guided Workflow.

**Trigger and observation:** A request excludes a metadata value, but a qualifying conversation has no matching excluded metadata row. The legacy LEFT JOIN predicate checks the joined key with `IS NULL`. With `join_use_nulls=0`, the unmatched String key is an empty string, so the check fails and valid rows disappear. KB Search, Suggestions, and Hints explicitly compare nullable and nonnullable joins. Summary/GenAI record the same default-join failure. Notes/Workflow record zero aggregates for exclusion-only and combined include/exclude requests; their reports do not independently isolate the connection setting as the cause.

**Impact:** An exclusion intended to narrow a report can remove nonexcluded conversations and their usage, including returning empty or zero results. Different join settings can change results for the same fixture.

**Proposed follow-up:** Engineering should reproduce this in the shared metadata builder, implement an explicit unmatched-row test compatible with supported join settings, and cover exclusion-only plus combined inclusion/exclusion across callers. Inspect production settings and affected request usage before estimating customer impact. PM input concerns prioritization, not whether an unmatched row satisfies an exclusion.

**Evidence:** [KB], [Chat], [Hints], [Summary/GenAI], [Notes/Workflow]. Hints case: `TestMetadataExclusionAndCombinedFilters`; Notes/Workflow cases: `TestMetadataLatestIncludeAndExclude`.

**Production verification — 19 October 2026:** Resolved `brinks-sbx/us-west-2` to deployment cluster `us-west-2-prod`, endpoint `clickhouse-conversations.us-west-2-prod.internal.cresta.ai`, database `brinks_sbx_us_west_2`, from config revision `7d17e6a77e559db02794c0ea0d9520b0b9f03367`. A live `admin` connection on ClickHouse `26.3.12.3` returned `join_use_nulls=0`, `changed=0`. A separate connection to replica `chi-conversations-conversations-0-2-0` also returned 0. Using only inline synthetic values A/B/C, with A excluded and B/C holding 3/1 searches, the legacy `IS NULL` predicate retained 0 conversations and 0 searches; checking unmatched empty keys retained the expected 2 conversations and 4 searches. No customer conversation rows were read by this reproduction.

**Production evidence limits:** The setting query reached replica `chi-conversations-conversations-1-0-0`; the reproduction reached `chi-conversations-conversations-0-2-0`. This is not an exhaustive replica survey or an end-to-end RPC reproduction. One completed KB-shaped SELECT in the coordinator's preceding 24-hour query log ran as `admin` without an explicit `join_use_nulls` override; its client name was empty, so it cannot be conclusively attributed to insights-server. Inspected deployment configuration selects an admin-password secret and no username override, and the application connection builder supplies no join setting. This confirms the failure on production infrastructure under the observed connection settings, not its prevalence in real customer requests. Authentication used the individually reviewed AWS SSO profile `us-west-2-prod_dev`; no passwords or tokens are retained in this register.

### AA-02 — Source and developer eligibility differs across features

**Affected:** KB Search, Chat Suggestions, Summary, GenAI, and their eventual intersection with the shared population.

**Trigger and observation:** KB Search and Chat Suggestions retain source-9 and developer-user evidence. A valid Summary generation/copy pair with source 7 and a developer user is counted, whereas GenAI separately rejects the corresponding source/developer evidence. These are pinned legacy differences, not changes introduced by extraction.

**Impact:** Feature totals can describe different sets of conversations. Applying a shared population can remove otherwise valid feature evidence; preserving the feature predicate alone does not preserve the final count.

**PM decision:** Confirm source, bot, and developer eligibility for the new report, then separately decide whether legacy dashboards should converge. Recommended new-API model: retain the feature-specific proof of use and intersect its conversation/agent keys with one authoritative eligible population. Do not assume every legacy feature filter should be copied into the new API unchanged.

**Evidence:** [KB], [Chat], [Summary/GenAI]. The population's Agent Assist ON branch needs the separate qualification in AA-03.

### AA-03 — Agent Assist ON does not enforce the same conversation-row requirements

**Affected:** Shared ConversationStats population and callers that reuse it.

**Trigger and observation:** Agent Assist ON selects assisted label rows instead of requiring the same `conversation_d` row used by the default/OFF branches. Characterization tests admit conversation-row source/developer exceptions and a label-only pair under ON. Matching message requirements still apply; this is not a blanket statement that all developer or unsupported-source messages qualify.

**Impact:** Changing the assistance filter can change eligibility rules beyond the assistance state itself. A new API inheriting this population also inherits those exceptions.

**Proposed follow-up:** Decide whether these branches should use identical conversation eligibility. Engineering should measure which historical records rely on the exceptions before correcting them. Preserve this distinction in explanations of the current population.

**Evidence:** [Population], `TestAssistanceSourceAndDeveloperPolicies`, including `On`, `OnWithBots`, and `OnWithMetadata`.

### AA-04 — Combined hint statistics use branch maxima rather than a combined population

**Affected:** Hint sent counts and conversation/user denominators.

**Trigger and observation:** Behavioral and nonbehavioral branches contain different conversations or users. The aggregate independently takes each branch's maximum sent count, conversation count, and user count. A fixture with branch populations of 2 conversations/2 agents and 3 conversations/1 agent reports 3 conversations and 2 users; those values do not describe a union-derived cohort.

**Impact:** Combined totals can lose contributions or combine denominators from different branches. A rate need not describe either branch or their union.

**PM decision:** Define whether combined totals mean distinct conversations/users across all hint types, total actions, or another measure. Engineering must retain membership until that aggregation is applied.

**Evidence:** [Hints], `TestMixedBranchesRetainMaximumDenominators`.

### AA-05 — Positive hint adherence can count without a delivered action

**Affected:** Behavioral hint sent/followed statistics and future qualifying-use logic.

**Trigger and observation:** A positive adherence annotation references a nonexistent delivered action. The default moment-based mode includes it in sent; both followed paths count it. Action mode removes it only from sent.

**Impact:** Reported following can lack evidence that an action was delivered. Sent and followed can describe different populations; aggregate followed counts are insufficient proof of recorded conversation-level use.

**PM decision:** Confirm whether qualifying hint use requires positive adherence linked to an actual delivered action. Decide how historical orphan annotations should be handled. Proposed new-API rule: require the linkage; keep legacy correction separate.

**Evidence:** [Hints], `TestBehavioralDeliveryModesAndDuplicateAnnotations`.

### AA-06 — KB hint follows include empty and unlinked action IDs

**Affected:** Knowledge hint followed counts.

**Trigger and observation:** Follow events of type 9 contain an empty `action_annotation_id` or an ID without a matching sent action. DISTINCT counting includes the empty string and unlinked IDs. Multiple events referencing one action count once.

**Impact:** Followed counts can include unverifiable action links. Counting a missing key once does not establish one valid delivered hint.

**PM decision:** Specify whether invalid links count as use, are excluded, or are reported separately. Engineering should measure historical link completeness and add explicit validity/linkage rules for the chosen contract.

**Evidence:** [Hints], `TestKnowledgeFollowsAreDistinctActionsIncludingMissingID`.

### AA-07 — Behavior filters remove action-mode hint populations

**Affected:** Hints with a behavior filter and action-based sent calculation.

**Trigger and observation:** The action table has no behavior column, and the existing parser supplies a false condition. The same behavior selection returns moment-owned results in default mode but removes action-mode sent groups.

**Impact:** Selecting a behavior can empty a report depending on the configured calculation mode.

**PM decision:** Should this combination be supported through linked behavior evidence, explicitly rejected, or unavailable in the UI? Engineering should implement the agreed contract with both modes covered.

**Evidence:** [Hints], `TestBehaviorGroupingAndFilterUseMomentRows`.

### AA-08 — Hint delivery and adherence can credit different agents

**Affected:** Agent-level hint sent/followed attribution.

**Trigger and observation:** A delivered action belongs to one agent and its adherence annotation to another. In action mode, the recipient group has sent=1/followed=0; the annotation-only agent group is dropped because the join starts from sent groups.

**Impact:** A recorded positive adherence can disappear from the displayed agent results or fail to credit the action recipient.

**PM decision:** Choose the credited agent: recipient of the delivered action, annotation agent, or another explicit rule. The new API must match the chosen agent as well as the conversation.

**Evidence:** [Hints], `TestDeliveredRowsOwnAgentAndTimeAttribution`.

### AA-09 — Grouped conversation totals can count one conversation multiple times

**Affected:** Hints, KB Search, Notes, Guided Workflow, and comparisons between grouped and overall totals.

**Trigger and observation:** One conversation appears under multiple agents or documents. Each group can count it once; some response totals sum those group counts. Hints reports 2 for a conversation with two agents. Notes/Workflow distinguish one overall conversation from two summed agent-group conversations. Workflow document grouping can also repeat a shared session/conversation.

**Impact:** Adding group rows does not necessarily yield the number of unique conversations. This is a counting-contract issue, not automatically a defect.

**PM decision:** Document the grain at every response level, including distinct overall conversations and any intentional sum of agent/team/document counts. Do not change legacy totals without a separate decision.

**Evidence:** [Hints], `TestSameConversationCountsForEachAgent`; [Notes/Workflow], `TestSameConversationMultipleAgentsGroupedAndOverall` and `TestGroupingByDocumentCountsSessionWithinEachDocument`; [KB] shared-conversation fixtures.

### AA-10 — Hint per-user averages become zero when frequency is unspecified

**Affected:** Hint results grouped by GROUP and POLICY without frequency buckets.

**Trigger and observation:** Group-level conversion derives the per-user denominator from time buckets. A no-frequency query omits those buckets, leaving group-level per-user averages at zero while top-level rates retain their values. Both delivery modes are characterized.

**Impact:** A populated report displays zero at one response level and a nonzero rate at another.

**Proposed follow-up:** Confirm the intended user denominator with PM, then compute it independently of optional bucket output. Cover both delivery modes and compare group and response rates.

**Evidence:** [Hints], full-RPC group/policy no-frequency cases in `TestRPCUserGroupPolicyAndTimeCoverage`.

### AA-11 — Guided Workflow counts an empty or missing session ID as use

**Affected:** Guided Workflow document-use statistics.

**Trigger and observation:** An event passes the used-document and document-type gates and contains a nonempty document ID, but its session ID is empty or absent. One distinct empty session key contributes one use. Both JSON payload and typed-column paths retain this behavior.

**Impact:** Legacy session totals can include records without a usable session identity and collapse multiple such records into one key. That does not establish how many conversations had valid document-use evidence.

**PM decision:** Is a qualifying document-use event sufficient, or must it carry a nonempty session ID? Resolve this before treating legacy session counts or extracted rows as the new API's qualification rule.

**Evidence:** [Notes/Workflow], `TestDocumentPayloadGates`, `TestDistinctSessionsAcrossEventsDocumentsAndConversations`, and `TestMetadataTypedDocumentGatesAndEmptySession`.

### AA-12 — Unbucketed Notes/Workflow results expose a year-1 timestamp

**Affected:** Notes and Guided Workflow response conversion.

**Trigger and observation:** Frequency is unspecified, including a zero aggregate. The row's `TimeRange.StartTimestamp` is `0001-01-01T00:00:00Z`.

**Impact:** Consumers receive a concrete sentinel date for an unbucketed result. Whether users see it depends on the UI; no UI impact was verified.

**PM/API decision:** Should an unbucketed result omit its bucket time or return the requested range? Engineering should verify consumer handling before changing the response.

**Evidence:** [Notes/Workflow], ungrouped/no-matching-data cases and preserved-quirk section.

### AA-13 — GenAI per-user averages differ across response levels

**Affected:** GenAI results with time-only grouping.

**Trigger and observation:** A fixture has two users on day one and one on day two. Bucket averages are 0.5 and 1; the result average is 1; the top-level response average is 2.

**Impact:** The same report exposes different denominator interpretations at different levels. Comparing or combining the displayed averages can mislead.

**PM decision:** Define the intended measure: usage per distinct user over the whole range, per active user-day, or another explicit denominator. Engineering should implement each response level from that definition and avoid averaging rates without weights.

**Evidence:** [Summary/GenAI], `TestGenAIUsageRegressionCHSuite/TestTimeGroupingDenominatorsAcrossDays`.

### AA-14 — GenAI annotations disappear without a matching message group

**Affected:** GenAI feature statistics.

**Trigger and observation:** Qualifying used annotations/actions exist but no qualifying message group exists. The legacy message-anchored query returns no result group.

**Impact:** Usage evidence alone does not guarantee inclusion. Missing messages and message filters can remove recorded use.

**PM decision:** For the new API, eligibility should come from the agreed conversation population, with feature evidence proving use inside it. Decide whether a valid annotation belonging to an eligible pair may qualify even when it would fail the legacy feature reader's separate message anchor. Investigate ingestion completeness before choosing any historical recovery rule.

**Evidence:** [Summary/GenAI], annotation-only reader-contract fixtures.

### AA-15 — Legacy denominators cannot define the new conversation-use percentage

**Affected:** All assistance categories and cross-feature totals.

**Trigger and observation:** KB Search and Chat Suggestions retain unrelated events to populate legacy conversation/user denominators. Notes/Workflow require joined event evidence, so a conversation with no event contributes nothing. Hints use feature-rooted branches, while GenAI is message-anchored. Numerators also count different units: event IDs, action IDs, or session IDs.

**Impact:** Summing feature totals or borrowing a feature's denominator cannot answer how many eligible conversations used at least one assistance feature. Repeated use and overlapping features cause additional double counting.

**PM decision:** Confirm one eligible conversation population, qualifying-use rules by feature, agent attribution, and aggregation grain. Proposed computation: select eligible `(conversation_id, agent_user_id)` pairs; select qualifying evidence per feature; intersect with eligible pairs; union/deduplicate membership before counting. Keep eligible pairs with no use in the denominator. For example, three searches in conversation A count as one conversation with KB use; eligible conversation B with no searches remains in the denominator.

**Evidence:** Denominator-only and distinct-ID fixtures in [KB] and [Chat]; no-event and distinct-session fixtures in [Notes/Workflow]; branch aggregation in [Hints]; anchoring/session fixtures in [Summary/GenAI]; pair relation in [Population].

### AA-16 — Benchmark and optimize Assistance Usage population transfer and memory

**Affected:** `RetrieveConversationAssistanceUsageStats`, eligible-population reads and all seven feature evidence queries.

**Classification and decision — 22 October 2026:** Performance follow-up, not a demonstrated correctness defect or performance-based merge blocker. The user approved deferring optimization until representative requests are benchmarked. External tables are an appropriate mechanism for large key sets; millions of rows alone do not demonstrate a problem. Production memory, latency, and capacity impact remain unmeasured.

**Trigger and observed implementation:** At PR #34044 head `7fdfd2b4aed1114fbc7f166c064faf325c543281`, the API materializes the eligible `(conversation_id, agent_user_id, conversation_time)` population in Go. Each of the seven evidence queries builds and uploads its own external table of eligible conversation/agent pairs, with at most three queries active concurrently. Larger populations and concurrent API requests increase buffer memory, transfer volume, and ClickHouse join work. These are source-level costs, not observed customer failures.

**Existing mitigations:** Agent lists use external data instead of large inline SQL lists; evidence scans use the requested conversation-start-time bounds; feature pairs are reduced before intersection; query concurrency is capped at three. These mitigations do not eliminate repeated population uploads. Preserve the requested range and all existing eligibility/use semantics; do not introduce an arbitrary fixed-day cutoff.

**Proposed follow-up ticket:** Benchmark Assistance Usage population transfer and memory, then optimize measured bottlenecks. Cover representative large eligible populations, selective metadata filters, short and long supported ranges, Events/Conversations source branches, and concurrent requests. Record eligible-pair count, external-data bytes, returned-pair count, query count, latency including p95, Go peak memory, ClickHouse peak memory, and rows/bytes scanned. Establish a baseline before selecting an optimization and rerun the same scenarios afterward. Verify exact pair membership and overall/Agent/Team/bucket counts, including authorization and filters, remain unchanged.

**Options to evaluate:** Combine feature reads sharing physical sources; keep population selection inside ClickHouse for same-cluster queries; compare downloading distinct evidence for Go-side intersection when that is cheaper; batch or stream data to bound buffers; adapt concurrency to population size. Consider precomputed conversation-level use only if measurements justify its freshness, correction, and backfill complexity. Same-cluster approaches must account separately for Events-cluster reads; batching can increase scans and Go-side intersection can transfer excessive unrelated evidence.

**Owner/ticket/status:** Owner unassigned; follow-up candidate recorded; no separate Linear ticket created. Benchmark and optimization are deferred. Do not infer an out-of-memory failure, production latency regression, or safe maximum request size from static inspection.

**Evidence:** [Review comment](https://github.com/cresta/go-servers/pull/34044#discussion_r4344192573); [pinned evidence-query implementation](https://github.com/cresta/go-servers/blob/7fdfd2b4aed1114fbc7f166c064faf325c543281/insights-server/internal/analyticsimpl/action_retrieve_conversation_assistance_usage_stats_clickhouse.go#L115-L197); [pinned population reader](https://github.com/cresta/go-servers/blob/7fdfd2b4aed1114fbc7f166c064faf325c543281/insights-server/internal/analyticsimpl/action_retrieve_conversation_assistance_usage_stats.go#L50-L134). Current source was checked during review; no benchmark was run for this entry.

## Suggested review order and tracking

1. Agree the new metric's population, attribution, evidence, and counting contract: AA-02, AA-03, AA-05, AA-06, AA-08, AA-09, AA-11, AA-14, and AA-15.
2. Triage the reproduced filter/response issues separately: AA-01, AA-07, AA-10, AA-12, and AA-13. Define AA-04's combined-hint meaning before choosing a correction.
3. Track AA-16 as a deferred performance follow-up: benchmark first, then choose optimizations from measured costs.
4. For each accepted change, record the PM decision, engineering owner, follow-up ticket, production prevalence evidence, and rollout/validation plan. No follow-up tickets or owners have been assigned by this document.

Future entries should record: stable ID; affected metric/API; classification; exact trigger; observed behavior; user consequence; evidence and revision; proposed contract/fix; PM decision; owner/ticket; status. Keep measured production impact separate from local reproduction.

## Evidence index

Links below point to immutable GitHub revisions so this document can be shared without local files. The five feature reports use baseline `6ba478ae4d7d7322e6f904dce2eb3f4ff2ea58c9`; the listed test revisions contain the characterization suites. Their recorded before/after results support preservation of the covered scenarios, not every possible state.

| Reference | Pinned evidence |
| --- | --- |
| KB | [KB regression suite](https://github.com/cresta/go-servers/blob/e2392e8254f1b74259a7b286933f63c5e29b0120/insights-server/internal/analyticsimpl/retrieve_knowledge_base_stats_test.go); [extracted selection](https://github.com/cresta/go-servers/blob/782f12c88aa815ae3ce63655b5545352bc80a1f9/insights-server/internal/analyticsimpl/common_knowledge_base_usage_clickhouse.go) |
| Chat | [Suggestions regression suite](https://github.com/cresta/go-servers/blob/34399998b3bc90c6d1f8b32682903366b88e81e8/insights-server/internal/analyticsimpl/retrieve_suggestion_stats_regression_test.go) |
| Hints | [Hints regression suite](https://github.com/cresta/go-servers/blob/3dab1f940dd30546b802b449915645225703566d/insights-server/internal/analyticsimpl/retrieve_hint_stats_test.go) |
| Notes/Workflow | [Notes regression suite](https://github.com/cresta/go-servers/blob/45e60762a15a85d9fb233349c5027b4d187fe289/insights-server/internal/analyticsimpl/retrieve_note_taking_usage_integration_test.go); [Workflow regression suite](https://github.com/cresta/go-servers/blob/45e60762a15a85d9fb233349c5027b4d187fe289/insights-server/internal/analyticsimpl/retrieve_guided_workflow_usage_integration_test.go) |
| Summary/GenAI | [Summary regression suite](https://github.com/cresta/go-servers/blob/99276db4a0264d82a6c69abfcbd94a8412689592/insights-server/internal/analyticsimpl/retrieve_summarization_usage_regression_clickhouse_test.go); [GenAI regression suite](https://github.com/cresta/go-servers/blob/99276db4a0264d82a6c69abfcbd94a8412689592/insights-server/internal/analyticsimpl/retrieve_genai_usage_regression_clickhouse_test.go) |
| Population | [Population regression suite](https://github.com/cresta/go-servers/blob/9406a383379ba2b15565c55e72b75d5a76080cff/insights-server/internal/analyticsimpl/retrieve_conversation_stats_test.go); [shared population selection](https://github.com/cresta/go-servers/blob/9406a383379ba2b15565c55e72b75d5a76080cff/insights-server/internal/analyticsimpl/common_conversation_population_clickhouse.go) |

[KB]: https://github.com/cresta/go-servers/blob/e2392e8254f1b74259a7b286933f63c5e29b0120/insights-server/internal/analyticsimpl/retrieve_knowledge_base_stats_test.go
[Chat]: https://github.com/cresta/go-servers/blob/34399998b3bc90c6d1f8b32682903366b88e81e8/insights-server/internal/analyticsimpl/retrieve_suggestion_stats_regression_test.go
[Hints]: https://github.com/cresta/go-servers/blob/3dab1f940dd30546b802b449915645225703566d/insights-server/internal/analyticsimpl/retrieve_hint_stats_test.go
[Notes/Workflow]: https://github.com/cresta/go-servers/tree/45e60762a15a85d9fb233349c5027b4d187fe289/insights-server/internal/analyticsimpl
[Summary/GenAI]: https://github.com/cresta/go-servers/tree/99276db4a0264d82a6c69abfcbd94a8412689592/insights-server/internal/analyticsimpl
[Population]: https://github.com/cresta/go-servers/blob/9406a383379ba2b15565c55e72b75d5a76080cff/insights-server/internal/analyticsimpl/retrieve_conversation_stats_test.go
