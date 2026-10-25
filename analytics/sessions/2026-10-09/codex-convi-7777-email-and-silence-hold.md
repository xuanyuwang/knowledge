# CONVI-7777 email and Silence/Hold continuation

- Date: 2026-10-09 (America/Toronto)
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`; read-only fetched `origin/main` at `04955d3a64aba40f88eac984d8f7d68315fb9f71` (local checkout on `main` was behind).
- Related snapshots: `/Users/xuanyu.wang/repos/director` `origin/main` at `2e8b714bac4385ea036a56581e22a9ed8a6ec1b0`; `/Users/xuanyu.wang/repos/cresta-proto` `origin/main` at `40941b12b272b98c6000b4d528b77e4150fa9f55`.
- Linear: CONVI-7777 In Progress, updated October 6, no comments. No code, production data, or ticket mutation.

## Findings

1. `RetrieveSuggestionStats` counts email used piecemeal and full-reply events in the conversations-cluster event table, with both ordinary and metadata-filter paths (`retrieve_suggestion_stats_clickhouse.go:55-92`). Director's `useAssistanceStatsWithSplitAPIs.ts:72-122,383-390` includes these counts. This closes the API-coverage question for these two email categories, subject to live data coverage.
2. `EMAIL_USED_SMART_COMPOSE` and `EMAIL_INSERTED_SEARCH_RESULT` are conversation-linked proto events and accepted by both PostgreSQL and events-cluster writes (`action_batch_create_conversation_events.go:98-116,139-157`). No insights-server stats query counts them. The conversations-cluster indexer imports only chat Smart Compose from the events cluster (`clickhouse.go:28-32`); its conversion switch lacks email event cases (`shared/clickhouse/conversations/conversation.go:3044-3066`). Treat email Smart Compose/search as an explicit scope expansion, not as existing stats coverage.
3. Director's Silence/Hold adapter chooses policies configured with a HINT action, then derives followed/sent counts from `RetrieveAdherences` SDX/DDX (`useGetSilenceHoldHintStats.tsx:25-136`). `RetrieveAdherences` does not join individual sent actions (`retrieve_adherences_clickhouse.go:17-76`). Actual delivered-hint linkage must be established before inclusion in a metric that excludes sent-only or unaided behavior.
4. Public Smart Compose proto documentation says edited inserted text can yield `used_text_length = 0`; a positive-length predicate is an exact-match lower bound. The final definition still requires product agreement (`cresta/v1/conversation/event.proto:1052-1273,1343-1360`).

## Next

- Resolve Notes and Smart Compose usage definitions; keep the email scope to partial/full Suggestions already in stats unless explicitly expanded.
- Trace a real Silence/Hold occurrence from policy action through action and moment annotation before counting it.
- Validate representative voice/chat/email event coverage, agent attribution, and history before committing to API/query design.

## PM inventory handoff

The user requested a full list for PM review before further implementation work. Created [the proposed feature inventory](../../deliverables/convi-7777-pm-feature-inventory.md) from the Assistance Insights tabs, split stats adapter, prior detailed audit, and this source refresh. It lists 13 candidate qualifying categories, current exclusions, and five explicit PM decisions. This is a proposal, not approval.

## API semantics follow-up

The user asked for the exact current API rules before designing the unified metric. Fetched go-servers `origin/main` `48d308822e90e17d2ca190987220601601ee9d97` and Director `origin/main` `da2e210926b60faefcdef8ede940ff40575d0e50`; read eight assistance RPC handlers/ClickHouse queries, shared filter/group parser, Director split adapter, ConversationStats, and relevant SQL goldens. [Detailed audit](../../deliverables/convi-7777-assistance-api-semantics-audit.md) records source predicates, numerator grains, conversation populations, filter behavior, grouping, and recommended alignment. Major source-level findings: no common denominator, variable bot/dev/source constraints, Notes/GW metadata enum mismatches, HintStats sent `UNION ALL` followed by `MAX`, and legacy Smart Compose agent/hour approximation. No production query or code change; deployed effect remains unverified.
