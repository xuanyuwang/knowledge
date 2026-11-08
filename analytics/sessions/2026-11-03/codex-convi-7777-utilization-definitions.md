# CONVI-7777 utilization definitions Q&A

Date: November 3, 2026 (America/Toronto).
Primary domain: analytics / assistance-insights.
Primary source repo: /Users/xuanyu.wang/repos/go-servers.
Source context: main checkout, HEAD 35c364ff698e86e4d8eef77f4504eb85dc76c5cc; read-only source inspection. Related Director checkpoint 34ad89e76d0050eaaf9ec6e8cf1b5148933ca84e and cresta-proto a415f145cf366c1f5dc9750fd61969130ed1049d. No fresh remote/deployment/customer-data verification.

## Question and conclusion

Can the collected CONVI-7777 documents explain engagement versus use, search-tab coverage, workflow completion requirements, Summary actions, and Assistance Used versus Convos with Assistance Used %? Mostly yes. Search producer coverage for all four tabs remains unproven; Summary wording conflicts with the preserved SQL predicate.

## Definitions

- Used is feature-specific recorded evidence, not one universal click or completion rule. Generation/delivery alone does not qualify.
- Behavioral/checklist hints: linked positive adherence evidence. KB/workflow hints: used document-chip event. These are the four hint categories included in the new metric.
- GenAI: KA_GEN_AI_REPLY_USED detected-use moment with a nonempty knowledge-session ID, rather than merely an answer being shown.
- Notes: CopyEntity; edits alone do not count.
- Search: USED_KB_SEARCH means search issued and response returned. Existing counts deduplicate event IDs; no result-count, article-open or query-type restriction is applied. Typed, suggestion and chip initiations are covered by the contract. Proto QueryType names ALL/Best Match, KB Article and Guided Workflow (plus zero search); it does not establish all-four-tab GenAI producer coverage. Any such request counts only if it emits the qualifying event. GenAI answer use is separately detected.
- Guided workflow document: USED_KB_DOCUMENT, workflow document type and nonempty workflow identifier. Open/navigation suffices; no step-count, completion percentage, dwell-time or completion test. Existing per-feature count deduplicates search session IDs at the query grouping level, not every step click or distinct completed workflows. Hint-originated opens and step navigation may produce the event. Workflow-hint engagement separately uses the hint chip event.
- Summary: valid generated-summary action plus matching used event at conversation/start/agent. Preserved SQL uses stored types 1/2; stored schema says 1=copy all, 2=edit note, 3=copy note. Thus existing behavior counts copy-all or edit-note and omits single-note copy. New metric reuses that helper, preserving the discrepancy. Generated-only is insufficient. Existing Summary count is distinct conversations per aggregation bucket, not copy-click volume. N/A describes absence of a valid generated opportunity; valid but unused should be distinguished from N/A.
- Chat Suggestions: normal, hint-originated or search-originated used-suggestion event, not merely shown/sent suggestions.

## Engagement versus conversation coverage

Existing hint engagement uses followed/sent. Overall Engagement pools four hint categories plus Summary used/generated and GenAI used/viewed. It does not include Search, standalone Workflow, Notes or Suggestions in that overall ratio. Summary used is capped by Summary generated in this calculation. This is different from the broad Assistance Used count/card/chart.

Existing Assistance Used combines heterogeneous per-feature counts (events, sessions and used-summary conversations), optionally shown as per-agent/per-interval averages. It measures volume; repeated qualifying actions can increase it, subject to each category's own deduplication. It is not a distinct conversation count.

New Convos with Assistance Used % = 100 * qualifying assisted conversation participations / all eligible filtered conversation participations for the Agent/Team view. One conversation counts once per agent even with multiple qualifying uses/features. Team sums agent participations, so a transferred conversation can contribute for more than one agent; ungrouped overall deduplicates conversation IDs. Denominator includes conversations with no assistance use, not just those offered assistance. Period rate is a ratio of counts, not an unweighted average of daily rates.

New V1 excludes Smart Compose and all email assistance, which existing card/chart can include where applicable. New hint categories exclude Reminder and Silence/Hold. These differences prevent exact reconciliation simply by summing displayed categories.

Illustration: for one agent with 10 eligible conversations, three qualifying searches and one qualifying workflow action in one conversation create 4 feature uses but only one assisted conversation, yielding 10% coverage.

## Evidence

Collected documents:
- ../../deliverables/convi-7777-pm-feature-inventory.md (October 9 proposal, contains early decisions; not current deployment status).
- ../../deliverables/convi-7777-assistance-api-semantics-audit.md (October 12 decision preserves existing per-feature SQL).
- ../../deliverables/convi-7777-assistance-analytics-issue-register.md (existing predicate/counting discrepancies).

Checked source paths (relative to the named repo):
- go-servers insights-server/internal/analyticsimpl/retrieve_knowledge_base_stats_clickhouse.go and common_knowledge_base_usage_clickhouse.go: distinct event IDs, no query-type predicate.
- go-servers insights-server/internal/analyticsimpl/retrieve_guided_workflow_stats_clickhouse.go: distinct session_metadata_id and workflow use predicate.
- go-servers insights-server/internal/analyticsimpl/common_summarization_usage_clickhouse.go: valid-summary join and data types 1/2.
- go-servers apiserver/sql-schema/protos/event/event.proto: UsedSummarization oneof fields 1/2/3.
- go-servers insights-server/internal/analyticsimpl/action_retrieve_conversation_assistance_usage_stats_clickhouse.go: seven evidence families, feature exclusions and cohort intersection.
- cresta-proto cresta/v1/conversation/event.proto: USED_KB_SEARCH / USED_KB_DOCUMENT contracts and UsedKbSearch.query_type.
- cresta-proto cresta/v1/search/search.proto: QueryType tab names.
- director packages/director-app/src/components/insights/assistance/assistance-insights-statistics/utils.ts: existing card/chart category selection and count/average mapping.
- director packages/director-app/src/components/insights/assistance/assistance-insights-statistics/assistance-used-page/assistance-used-leaderboard/hooks/useAssistanceUsedByAgentLeaderboard.tsx: Overall Engagement numerator/denominator.

## Actions and remaining questions

Documentation only. No source changes, tests, commits, pushes, external messages or customer queries. No AWS, Okta, Azure or SSH credentials were read or explicitly selected; no authentication was needed.

Before claiming every knowledge tab contributes Searches, trace current client producers (especially GenAI and automatic/tab-switch requests). Resolve the Summary copy-only product intent separately from the preserved implementation. No new product decision is implied by this explanation.
