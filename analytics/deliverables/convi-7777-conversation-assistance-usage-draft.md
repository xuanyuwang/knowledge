# CONVI-7777: Conversation-level assistance usage — investigation and implementation draft

**Scope update, October 9:** Chat Smart Compose and **all email use cases are P2**, outside v1. Current SmartComposeStats cannot assign its legacy events to a specific conversation, but newer stored `USED_SMART_COMPOSE` events do contain conversation IDs and payload. Partial/full email Suggestions are conversation-linked and already counted by current stats, while email Smart Compose/search need new stats work. Silence/Hold is a conditional v1 candidate guarded by `enableSilenceHoldHintsTab`; its qualifying-use signal remains unresolved. The exploratory design and test notes below are not an approved inclusion; use the [PM inventory](convi-7777-pm-feature-inventory.md) for current scope.

Date: 2026-10-05. Status: **provisional; product decisions and data validation remain**. No product code changes or production queries performed.

## 2026-10-09 source refresh: email and Silence/Hold

Linear remains In Progress with no comments (`updatedAt` 2026-10-06). This refresh read fetched `origin/main` snapshots, not the older local `main` checkouts: go-servers `04955d3a64aba40f88eac984d8f7d68315fb9f71`, Director `2e8b714bac4385ea036a56581e22a9ed8a6ec1b0`, cresta-proto `40941b12b272b98c6000b4d528b77e4150fa9f55`.

- **Email Suggestions are in the existing stats path, but P2 for the new metric.** `RetrieveSuggestionStats` counts DB event types 25 (`EMAIL_USED_PIECEMEAL_SUGGESTION`) and 27 (`EMAIL_USED_FULL_REPLY_SUGGESTION`) in both metadata and ordinary branches (`retrieve_suggestion_stats_clickhouse.go:55-92`). Director's split adapter adds both to per-group `totalAssistanceCount` and the overall count (`useAssistanceStatsWithSplitAPIs.ts:72-122,383-390`). This confirms current API/UI coverage, not live customer event coverage. If added after v1, count conversation membership from the underlying used events; do not add these event counts to a numerator.
- **Other email assistance does not have current stats coverage.** The public proto defines conversation-linked `EMAIL_INSERTED_SEARCH_RESULT` (31) and `EMAIL_USED_SMART_COMPOSE` (32). Both are accepted into PostgreSQL and the events cluster (`action_batch_create_conversation_events.go:98-116,139-157`), but neither appears in insights-server stats queries. The email Smart Compose payload has `used_text_length` and the same exact-match limitation as chat Smart Compose (`event.proto:1343-1360`); the events converter retains it (`shared/clickhouse/events/converters.go:297-307`). The conversations-cluster indexer reads only chat `USED_SMART_COMPOSE` from the events cluster (`apiserver/internal/conversation/clickhouse.go:28-32`); its events-cluster conversion switch also has no email cases (`shared/clickhouse/conversations/conversation.go:3044-3066`). Thus adding email Smart Compose or email search requires explicit projection/query work and a product predicate; event ingestion alone is insufficient. EA prefers all email use cases in P2.
- **Silence/Hold remains unproven as a qualifying hint use.** Director selects active silence policies *configured* with a HINT action, then maps `RetrieveAdherences`' SDX and average adherence into hint sent/followed counts (`useGetSilenceHoldHintStats.tsx:25-136`). The backend adherence query counts SDX and DDX moments without joining a sent action (`retrieve_adherences_clickhouse.go:17-76`). A positive behavior can therefore contribute to that page metric without proving the hint was delivered. A new metric must use a per-occurrence sent-action link or exclude this category; aggregate `totalHintFollowedCount` from this adapter is not suitable.
- **Positive retained-text length remains a product choice.** `USED_SMART_COMPOSE` event documentation explicitly allows a zero-length exact match after the agent edits inserted text (`event.proto:1052-1273`); `used_text_length > 0` is a conservative lower bound, not a complete definition of assisted composition. The same issue applies to email Smart Compose. Do not silently treat zero as unused until the metric's intended interpretation is approved.

No production coverage, backfill feasibility, or deployment state was verified in this refresh.

## Goal and confirmed requirements

Add a new column inside `data-testid="assistance-used-leaderboard"` reporting the percentage of conversations with at least one qualifying assistance use. This is not a pooled followed/sent rate. A hint being sent alone does not count. Support standard filters and conversation-linked voice/chat/email usage; calculate in the backend. The ticket's feature list is provisional.

Confirmed formula: `distinct qualifying conversations / distinct eligible conversations`. The denominator is **all filtered conversations**, including those not offered assistance. Both Agent and Team tabs are confirmed.

### Follow-up requirement decisions (2026-10-05)

Use **assistance**, not hints, as the metric scope and terminology. User accepts the inventory/definitions in principle but wants Notes and Smart Compose docs before finalizing those predicates; `usedTextLength > 0` is not approved yet. Customer Snapshot remains out of v1. This earlier pass considered partial/full email Suggestions because current stats already count them; EA's subsequent preference moves **all email use cases to P2**. Silence/Hold inclusion remains conditional on actual sent-and-followed evidence; unlike Reminder, these have tracked behavior adherence. Later sections retain the initial draft proposals, subject to the scope update at the top.

Feature docs for user review:
- [Smart Compose and Suggestions](https://docs.cresta.com/documentation/agent-assist/chat-agent-assist/key-features/smart-compose-and-suggestions): inline autocomplete, Right Arrow for next word / Tab for whole completion.
- [Atomic Voice Application — Automatic note-taking](https://docs.cresta.com/documentation/agent-assist/voice-agent-assist/voice-application/atomic-voice-agent-application): extracted names/emails/phone numbers, click to copy, double-click to edit.
- [Notes card](https://docs.cresta.com/documentation/insights/assistance-insights/notes-card-voice): broader interaction description; conflict with copy-only analytics remains explicit.
- [Hold Events](https://docs.cresta.com/documentation/opera/navigating-opera-rules-page/creating-a-new-opera-rule/1.-define-opera-rule/track-a-behavior/hold-events): e.g. hint at 90 seconds before 2-minute missed-behavior threshold.

Follow-up code evidence: `useGetSilenceHoldHintStats.tsx:25-136` selects policies configured with hints but maps generic adherence aggregates to sent/followed counts; configuration-level hint presence is not occurrence-level delivery proof. Email partial/full use events are conversation resources (`cresta/v1/conversation/event.proto:98-106,349-365`) and are queried from conversation events in `retrieve_suggestion_stats_clickhouse.go:63-65,85-87`; technical linkage exists, but coverage remains unverified.

Example: 100 eligible conversations, hints followed in 20, summaries copied in 15, with 10 conversations in both => 25%, not 35%. Multiple uses/features in the same conversation count once.

## Hint versus assistance

- Product concept: **assistance** includes Opera hints plus GenAI answers, summaries, searches, workflows, entity Notes, Suggestions, Smart Compose, etc. A hint is one delivery mechanism, not a synonym for every feature.
- `RetrieveHintStats` reports sent/followed counts for hint categories, policy breakdowns, and rates/averages. Its followed sources are positive action-linked adherence moments and hint document-chip use events.
- `RetrieveAssistanceStats` is a legacy API; current implementation is NOT a universal cross-feature conversation-usage API. It queries followed hints and a message-based conversation population. It can return empty under `DeprecateRetrieveAssistanceStats`.
- The target table uses `useAssistanceStatsWithSplitAPIs`: Suggestion, SmartCompose, KnowledgeBase, NoteTaking, GuidedWorkflow, Summarization, Hint stats. GenAI arrives separately through KnowledgeAssistStats. ConversationStats and AgentStats supply other columns.
- Existing `overallEngagement` pools four hint types + summary + GenAI used/generated counts. Preserve it unchanged.
- Existing `convosPoweredByAARate` uses `agentAssistUsedOnly` / `has_agent_assistance` labels, based on agent online-window overlap rather than actual feature usage. Preserve it unchanged and avoid reusing its name/flag for this metric.

## Verified inventory and recommended predicates

These are source-code observations, not proof of complete customer/historical event coverage.

| Feature | Current usage evidence | Conversation linkage | Draft treatment |
|---|---|---|---|
| Behavioral / Checklist hints | `moment_annotation_d`: positive adherence (`adherence_type=2`) linked to a nonempty `adherence_action_annotation_id`; Opera policy source | Yes | Include; validate actual eligible sent action, not merely successful behavior |
| KB / Guided Workflow hints | `conversation_event_d`: `USED_HINT_DOCUMENT_CHIP` with associated action/policy | Yes | Include document-chip use; workflow completion is not required |
| Reminder hints | Sent volume; no tracked followed behavior | Yes, but no qualifying use | Exclude as requested |
| GenAI answers | `KA_GEN_AI_REPLY_USED` moment (`moment_type=38`), nonempty knowledge session | Yes | Include existing detected-use semantics, not shown/viewed/copy alone |
| Summary | Valid generated summary + `USED_SUMMARIZATION` copy-all or copy-summary payload; current API counts distinct conversations | Yes | Include copied generated summary; generation/edit alone does not qualify |
| Notes | Automatically extracted transcript entities (names, phone numbers, email), not arbitrary manual notes. Current query counts CopyEntity | Yes | Include entity copy. GitBook also says click/edit, conflicting with query and migration doc; recommend query/migration definition |
| Searches | `USED_KB_SEARCH`; counts searches, not successful answers or article clicks | Yes for conversation event path | Include explicit in-conversation search; exclude standalone searches without linkage |
| Guided Workflow (standalone) | `USED_KB_DOCUMENT`, document type Guided Workflow, document ID; current count dedups session ID | Yes | Include opening a workflow; distinct from following a pushed workflow hint, but dedup conversation across both |
| Suggestions | Current query uses `USED_SUGGESTION`, `USED_HINT_SUGGESTION`, `USED_SEARCH_SUGGESTION` | Yes, often associated agent message | Include chat suggestions; they are not inherently unlinked |
| Smart Compose | Client SDK emits `USED_SMART_COMPOSE` on sent message with textLength/usedTextLength; events may have usedTextLength=0 | Yes in modern client path | P2. A future query needs coverage and projection work; never infer conversation from agent/hour. |
| Silence/Hold hints | Separate tab adapts `RetrieveAdherences` into hint-shaped stats; SDX/DDX behavior adherence, not necessarily sent-action proof | Conversation moments exist | Conditional v1 candidate guarded by `enableSilenceHoldHintsTab`; qualifying-use signal pending. The flag does not prove hint delivery. |
| Customer Snapshot | New page tab; API measures panel display duration from presented/closed events | Yes | Explicit product decision. Do not automatically count display as usage under the sent-only exclusion |
| Email Suggestions / Smart Compose / search insertion | Separate conversation event types exist; SuggestionStats counts partial/full Suggestions | Yes in contracts | All are P2 per EA. Partial/full Suggestions already have stats coverage; Smart Compose/search need new query/projection work and a use predicate. |

Other event types (e.g. external URL hint clicks, GenAI rationale clicks, workflow automation) exist in event contracts. Event existence is not a reason to count them all as assistance usage. This is an audited page/API inventory, not a claim of an exhaustive taxonomy of every Agent Assist capability. Compliance hints are a product label in configuration docs; their exact mapping to behavioral/checklist predicates requires follow-up rather than adding a second counting category. Kudos/reinforcement are likewise not automatically qualifying usage.

## Important implementation findings

### Current hint grain is already fixed in this checkout

`retrieve_hint_stats_clickhouse.go:315-350` counts distinct adherence action IDs. `retrieve_assistance_stats_clickhouse.go:23-35` documents the same fix. The older knowledge warning about followed-moment inflation is historical, not current checkout behavior. Deployment status was not checked.

### Smart Compose has real producer + ingestion, but incomplete projection

- Glean code search returned full `client-sdk/ui-components/client-sdk-smart-compose-overlay/src/smart-compose-event-manager/smart-compose-event-manager.ts`: listens for inserts and the subsequent agent message, emits conversation-named events, tests whether inserted text remains at the expected position, sets usedTextLength to full length or zero.
- Thus **an event named USED_SMART_COMPOSE is not by itself proof of retained text**. The proto calls this an exact-match lower bound; edited/partial usage can be missed.
- `apiserver/internal/conversation/action_batch_create_conversation_events.go:139-146` supports writing these events to the events ClickHouse cluster.
- `apiserver/internal/conversation/clickhouse.go:29-32,70-89` reads Smart Compose events for conversation indexing.
- `shared/clickhouse/conversations/conversation.go:2892-2961` maps event types and conversation/message attribution but drops Smart Compose payload. `ConversationEventRow:203-237` has no used-text-length column.
- Existing `retrieve_smart_compose_stats_clickhouse.go:39-99` still reads legacy `analytics_event_d` inserts and joins agent/hour cohorts. This cannot satisfy precise conversation-level filtering/deduplication.

### Two clusters / multiple enum domains

`conversation_event_d` means different schemas in the conversations and events clusters. Do not copy integer predicates across them. Example Notes is public/DB type 17 but events-cluster type 5; Smart Compose is public/DB 22 but events-cluster 6. Summary has legacy and newer payload encodings with different copy/edit numeric mappings.

Notes and GuidedWorkflow currently use events cluster without metadata filters and conversations cluster when metadata is present. Their metadata branches retain event-cluster numeric predicates even though the conversation projection uses DB enum values: an apparent compatibility issue that requires focused tests/data verification, not behavior to preserve blindly.

The conversation projection already has flattened Notes copy, KB document type/session, and summary payload columns for PG-backed events. Raw events imported from events cluster zero those generic payload columns. Smart Compose needs payload-preserving projection for a positive-used-length definition, or a carefully bounded cross-cluster membership plan.

### Attribution and denominator need explicit rules

Raw events cluster writer currently uses the conversation's agent; the conversation projection can prefer associated-message agent. Transfer attribution is therefore not guaranteed identical across sources. Need tests on multi-agent chat/transfer cases and a clearly chosen grain.

Recommended grain: `(customer, profile, conversation_id, agent_id)` for agent metrics. Team row should dedup conversation IDs among selected team members; do not blindly add agent counts for transferred/shared conversations. Overlapping groups may each contain the same conversation; response-wide totals must not sum overlapping groups.

Use the existing ConversationStats eligibility semantics as reference (time basis, source, dev-user exclusion, permissions, filters), but compute numerator and denominator inside the same new API. Do not use per-feature `total_conversation_count` fields as a common denominator; their source populations differ.

## Draft implementation

### 1. Confirm semantics and validate data coverage first

- Denominator and both tabs approved; Snapshot excluded. Chat Smart Compose and all email use cases are P2. Guard any Silence/Hold contribution with `enableSilenceHoldHintsTab` and resolve its qualifying-use signal.
- Approve copy-only Notes, copied generated Summary, search/open as active usage, GenAI detected use, and Smart Compose retained-text rule.
- Read-only sample validation for representative voice/chat tenants: event types, linked IDs, dates/retention, positive vs zero Smart Compose lengths, transfer-agent identity, metadata-filter parity, index freshness.
- Establish historical coverage policy. Missing tracking is not equivalent to zero use. Decide rollout date / availability treatment rather than silently publishing an incomplete universal metric.

### 2. New analytics API (preferred)

Working name: `RetrieveConversationAssistanceUsageStats` (not yet an approved proto contract).

Reuse standard customer/profile parent, analytics filters, ACL/user resolution, agent-only option, grouping conventions. Return exact counts per agent or group:

- `total_conversation_count`
- `conversations_with_assistance_used_count`

Optionally include a derived ratio; counts remain authoritative. Zero denominator should render N/A, nonzero denominator with zero uses should render 0%. Keep missing/unavailable/error distinct from zero.

Support aggregate table requests first; frequency support can follow existing conventions if needed, but ratios must be computed from interval totals, not averages of daily percentages. No new chart required for this ticket.

Why new API: cross-feature semantics and dedup grain differ from HintStats and legacy AssistanceStats; existing `agentAssistUsedOnly` means something else. An explicitly named extension to ConversationStats could work but must be opt-in and not silently alter shared existing callers.

### 3. Backend computation

Conceptual query (not runnable SQL):

```sql
eligible_conversations = authorized, filtered conversation cohort
qualifying_usage = UNION ALL of qualifying conversation/agent keys from
  followed sent hints, GenAI used moments, summary copies,
  entity copies, searches, workflows, chat suggestions, approved chat Smart Compose
used_conversations = DISTINCT qualifying_usage joined to eligible_conversations
result = count eligible keys, count eligible keys with any qualifying usage
```

No summing per-feature aggregate API responses. A workflow opening that also follows a hint still yields one qualifying conversation. Use a semi-join/one-row-per-key to prevent join multiplication. Include zero-use conversations via a denominator-rooted left join.

Preferred data path: consolidate required qualified usage evidence in the conversations cluster and run dedup/aggregation there, reusing current annotation/event tables. Add minimal payload projection for Smart Compose used length (or a documented/versioned qualified-use flag), extend schema/writer/reader conversion tests, and backfill within available raw-event retention. Evaluate a dedicated normalized usage projection only if existing tables cannot meet correctness/latency; do not invent a large new pipeline by default.

Alternative: fetch bounded distinct usage keys from events cluster into a conversations-cluster external table, then join to the eligible cohort. This can avoid schema changes but introduces high cardinality, consistency, memory and latency risk; benchmark before choosing. Never union SQL across two clients as if tables share a database, and never fallback to approximate agent/hour mapping.

Standard filter acceptance includes date/timezone (recommend conversation-start basis to match Assistance), selected use case/channel, agent/team/group and permissions, deactivated/agent-only, duration, metadata inclusion/exclusion and value-or-missing. Shared parser currently rejects value-or-missing for unsupported paths; CONVI-7708 is related. Verify capabilities explicitly.

### 4. Director

Target prefix:
`/Users/xuanyu.wang/repos/director/packages/director-app/src/components/insights/assistance/assistance-insights-statistics/assistance-used-page/`

- `AssistanceUsedLeaderboardContainer.tsx`: new API requests for agent and group results using the same page filters.
- `assistance-used-leaderboard/AssistanceUsedLeaderboard.tsx`: pass new results.
- `.../types.ts`: counts + optional derived ratio.
- `.../hooks/useAssistanceUsedByAgentLeaderboard.tsx` and `useAssistanceUsedByTeamLeaderboard.tsx`: join results by resource identity, preserve zero-use rows, use backend group counts.
- `.../utils.tsx`: percentage column, tooltip defining distinct conversations, sorting, CSV header/value.
- `.../hooks/useVisibleColumnsForAssistanceUsedLeaderboards.tsx`: visible even when rate is 0; independent rollout flag if needed, not the existing voice-only powered-by-AA gate.
- API wrapper/types/query factory, i18n, configurable column menu and export parity.
- Proposed label: **Conversations with assistance used (%)**. Preserve Overall engagement and Conversations powered by Agent Assist.

### 5. Validation

- Hint sent-only/negative/no-opportunity does not qualify; positive behavior without sent hint does not qualify.
- Every included feature alone can qualify; ignored/edited-only Summary and zero-length Smart Compose cannot.
- Duplicate events/reindex rows, repeated usage and feature overlap count once.
- 0/0 => N/A; 0/N => 0%; N/N => 100%; counts always satisfy numerator <= denominator.
- Time-boundary/post-call summary copy belongs to conversation cohort, not event-day cohort.
- All standard filters and ACLs apply equally to numerator/denominator; standalone or orphan events cannot count.
- Two simultaneous chats for one agent; transfer/multi-agent team dedup; overlapping groups.
- Enum and payload conversion tests for both clusters, including positive/zero Smart Compose lengths and old/new Summary copy encodings.
- FE sorting, zero vs unavailable, menu visibility, CSV and existing columns unchanged.
- Representative ClickHouse latency/cardinality benchmarks and bounded historical coverage audit before rollout.

## Sources and checkpoints

Local source snapshots (`main` checkouts, read-only):
- go-servers: `19cd65cbb574e14f6e0ad367b0eec30c8086a852`
- director: `f47845f4f7f17d4da582d8d480dfec7b2ac1c1b6`
- cresta-proto: `a52824040e4e541cc61b33b4d92bc3d485bb31ca`

Backend primary root: `/Users/xuanyu.wang/repos/go-servers/insights-server/internal/analyticsimpl/`.
Relevant files: `retrieve_hint_stats_clickhouse.go:315-350`; `retrieve_assistance_stats.go:15-116`; `retrieve_assistance_stats_clickhouse.go:23-203`; `retrieve_knowledge_assist_stats_clickhouse.go:182-198`; `retrieve_summarization_stats_clickhouse.go:162-190`; `retrieve_note_taking_stats_clickhouse.go:65-173,185-219`; `retrieve_guided_workflow_stats_clickhouse.go:145-188,205-241`; `retrieve_suggestion_stats_clickhouse.go:52-96`; `retrieve_smart_compose_stats_clickhouse.go:39-99`; `retrieve_customer_snapshot_stats_clickhouse.go:17-60`; `retrieve_adherences_clickhouse.go:17-76`.

Glean sources consulted (search excerpts, not independent deployment verification):
- [Assistance Insights](https://docs.cresta.com/documentation/insights/assistance-insights)
- [Assistance Insights Updates Overview](https://docs.superhuman.com/d/_doM9F-e3jUe/_supZOU1Z) — migration definitions; Notes copy-only, searches during conversation, Suggestions sent reply; warns of legacy-client coverage.
- [Summary Card](https://docs.cresta.com/documentation/insights/assistance-insights/summary-card)
- [Notes Card](https://docs.cresta.com/documentation/insights/assistance-insights/notes-card-voice) — broader click/edit definition conflicts with code/migration doc.
- [GenAI Answers Card](https://docs.cresta.com/documentation/insights/assistance-insights/genai-answers-card) — conveying information, not exact wording.
- [Smart Compose and Suggestions](https://docs.cresta.com/documentation/agent-assist/chat-agent-assist/key-features/smart-compose-and-suggestions)
- [Smart Compose producer](https://github.com/cresta/client-sdk/blob/master/ui-components/client-sdk-smart-compose-overlay/src/smart-compose-event-manager/smart-compose-event-manager.ts) — full file in Glean code results, plus tests with zero used length.
- [Hint configuration overview](https://docs.cresta.com/documentation/opera/navigating-opera-rules-page/creating-a-new-opera-rule/1.-define-opera-rule/send-the-agent-assistance/hint-configuration-overview)
- [Hold Events](https://docs.cresta.com/documentation/opera/navigating-opera-rules-page/creating-a-new-opera-rule/1.-define-opera-rule/track-a-behavior/hold-events)
- [Customer Snapshot design](https://docs.superhuman.com/d/_d9CPCLe1mII/_sup7JNMR)

No claim of production deployment, data coverage, performance, or passing tests is made in this draft.
