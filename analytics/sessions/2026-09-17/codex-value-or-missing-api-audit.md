# Value-or-missing metadata filter API audit

Date: 2026-09-17  
Primary source repo: `/Users/xuanyu.wang/repos/go-servers`  
Branch/worktree context: read-only audit from `/Users/xuanyu.wang/repos/go-servers-convi-7706`, `xw/convi-7706-conversation-stats-value-or-missing` at `891796f7d8`; Director inspected at `/Users/xuanyu.wang/repos/director`

## Question

Which ClickHouse analytics RPCs still reject metadata groups representing `selected value OR (no value)`, and which of those are actually reachable from current Director product surfaces?

## Contract and audit boundary

- Director's shared `useMomentGroupFilterFromFilterState` always enables combined no-value encoding. The same metadata template is placed in both `moments` and `excluded_moments`, with selected values in `metadata_value_attributes`.
- Elasticsearch's generic conversation-document converter implements this overlap as `bool.should` with minimum one match.
- `RetrieveConversationStats` is covered by the local CONVI-7706 fix. This audit covers the other current production callers of shared `parseClickhouseFilter`.
- Current `origin/main` has 16 production callers of `parseClickhouseFilter`, including `RetrieveConversationStats`; 15 remain behind the overlap rejection.
- Code-path reachability is not a production-error count. No HAR or log capture was supplied for the APIs below.

## Confirmed Director exposure: Assistance Insights

`AssistanceFiltersState` includes metadata and customer-outcome metadata state. When `hasAssistanceMetadataFilters` is enabled, Director renders those filters, builds them through `useMomentGroupFilterFromFilterState`, and passes the same request parameters into Assistance Insights stats calls.

| RPC | Current metadata behavior | Director path | Decision |
| --- | --- | --- | --- |
| `RetrieveAssistanceStats` | Value-only and missing-only implemented via `generateMetadataFilters`; overlap rejected | Legacy assistance-stats mode | Must support value-or-missing |
| `RetrieveAgentStats` | Value-only and missing-only implemented via `generateMetadataFilters`; overlap rejected | Assistance Used leaderboard | Must support value-or-missing |
| `RetrieveHintStats` | Value-only and missing-only implemented via `generateMetadataFilters`; overlap rejected | Split assistance aggregation and hint tabs | Must support value-or-missing |
| `RetrieveKnowledgeAssistStats` | Value-only and missing-only implemented via `generateMetadataFilters`; overlap rejected | Assistance cards and leaderboards | Must support value-or-missing |
| `RetrieveSuggestionStats` | Value-only and missing-only implemented via `generateMetadataFilters`; overlap rejected | Split assistance aggregation | Must support value-or-missing |
| `RetrieveKnowledgeBaseStats` | Value-only and missing-only implemented via `generateMetadataFilters`; overlap rejected | Split assistance aggregation | Must support value-or-missing |
| `RetrieveNoteTakingStats` | Value-only and missing-only implemented via `generateMetadataFilters`; overlap rejected | Split assistance aggregation | Must support value-or-missing |
| `RetrieveGuidedWorkflowStats` | Value-only and missing-only implemented via `generateMetadataFilters`; overlap rejected | Split assistance aggregation | Must support value-or-missing |
| `RetrieveSummarizationStats` | Value-only and missing-only implemented via `generateMetadataFilters`; overlap rejected | Split assistance aggregation | Must support value-or-missing |
| `RetrieveSmartComposeStats` | Calls `parseClickhouseFilter` but discards both metadata condition lists; value-only and missing-only groups are therefore not applied, while overlap is rejected | Split assistance aggregation | First implement complete metadata filtering; do not add overlap alone |

The legacy and split assistance calls are feature-flag alternatives, so they do not necessarily execute together. `RetrieveKnowledgeAssistStats`, feature-specific calls, and `RetrieveAgentStats` have additional tab/metric gating. They are nevertheless structurally reachable with the same metadata filter state.

## Remaining shared-parser callers

| RPC | Current metadata behavior | Current Director evidence | Decision |
| --- | --- | --- | --- |
| `RetrieveCustomerSnapshotStats` | Uses `generateMetadataFilters`; overlap rejected | No Director caller found | Backend-ready candidate, but audit non-Director consumers before prioritizing |
| `RetrieveLiveAssistStats` | Discards parsed metadata condition lists | Leaderboard callers intentionally omit metadata filters | Do not add overlap in isolation; clarify whether metadata filtering is part of its API contract |
| `RetrieveManagerStats` | Discards parsed metadata condition lists | Manager Leaderboard request intentionally omits metadata filters | No current product need established |
| `RetrieveAdherences` | Discards parsed metadata condition lists | Silence/hold helper strips moment groups; Opera requests do not add them | No current product need established |
| `RetrieveScorecardStats` | Discards parsed metadata condition lists | Director hook has no caller; Manager Leaderboard uses QA score stats instead | No current product need established |

## Implementation recommendation

1. Keep CONVI-7706 focused on `RetrieveConversationStats`.
2. Implement [CONVI-7708](https://linear.app/cresta/issue/CONVI-7708/support-value-or-missing-metadata-filters-across-assistance-insights) for the nine RPCs already using `generateMetadataFilters`. Extend the common metadata-filter representation/generator to retain overlap as an OR group, then validate every query shape because their base tables and CTE placement differ.
3. Track `RetrieveSmartComposeStats` separately as a broader metadata-filter correctness bug. Its current problem predates value-or-missing: ordinary metadata groups are silently ignored.
4. Do not blanket-enable overlap for Live Assist, Manager, Adherences, or Scorecard Stats. First decide whether metadata moment groups are supported inputs; if not, explicitly reject all such groups rather than silently ignoring value-only requests.
5. Audit non-Director callers of `RetrieveCustomerSnapshotStats` before deciding priority.

## Validation expectations for the Assistance parity work

- Value-only, missing-only, and value-plus-missing produce consistent populations.
- Multiple selected values preserve latest-value semantics.
- Each overlap group is AND-ed with other metadata groups and unrelated exclusions.
- Current and comparison-window calls behave identically.
- Legacy `RetrieveAssistanceStats` and split API aggregation return contract-equivalent filtered populations.
- Smart Compose is excluded until full metadata filtering exists.

## Actions and credentials

- The source audit itself was read-only. Follow-up Linear ticket CONVI-7708 was later created at Medium priority, related to CONVI-7706 and CONVI-7402, to track the nine confirmed Assistance Insights RPCs.
- No product code or production state changed as part of the audit follow-up. No AWS, Okta, Azure, or SSH credentials were read or used.
