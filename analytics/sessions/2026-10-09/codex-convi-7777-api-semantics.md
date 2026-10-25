# CONVI-7777 assistance API semantics verification

**Source repo:** `/Users/xuanyu.wang/repos/go-servers`, fetched `origin/main` at `48d308822e90e17d2ca190987220601601ee9d97`, read only. Related checkpoints: Director `da2e210926b60faefcdef8ede940ff40575d0e50`; cresta-proto `40941b12b272b98c6000b4d528b77e4150fa9f55`.

**Question:** Which assistance actions, conversations, filters, and aggregation semantics do the existing Assistance Insights APIs use, and must they be aligned before the new union metric?

**Evidence and result:** [Full audit](../../deliverables/convi-7777-assistance-api-semantics-audit.md) covers eight assistance RPCs and `RetrieveConversationStats`. I rechecked the highest-risk conclusions against the fetched source: ConversationStats joins `conversation_d` and `message_d` on conversation plus agent, so zero-message conversations are excluded; GenAI roots at `message_d` and joins usage by agent/time group; Summary requires a valid-summary action joined to a copy event; Smart Compose links legacy events by agent/hour. Notes and Guided Workflow switch from the events to conversations ClickHouse cluster under metadata filtering but retain events-cluster type predicates (5 and 4), whereas the conversations-cluster DB enum uses 17 and 14. HintStats has a two-branch `UNION ALL` for sent hints followed by `MAX` in `combined_hint_sent`. SuggestionStats and KnowledgeBaseStats count distinct event IDs while their conversation count covers any filtered event row.

**Decision boundary:** The new API needs one eligibility cohort and one conversation-key union across approved feature signals. Existing RPC aggregates are not composable. The current Assistance page's `RetrieveConversationStats` population is a compatibility target, subject to an explicit zero-message decision. A broad existing-RPC cleanup is not required first; metadata-path defects must be fixed or isolated before using those outputs for parity. This remains source-level; no deployed data checks were performed.

**Validation:** `git diff --check` in knowledge passed. No product code, tests, production queries, or Linear mutations.

**V1 scope update:** The audit now covers seven V1 feature RPCs plus ConversationStats. `RetrieveSmartComposeStats` was removed from the V1 comparison, and SuggestionStats is described only for its chat-use predicates. Chat Smart Compose and all email assistance remain P2 in the feature inventory. The initial eight-RPC investigation above is retained as the historical source check.
