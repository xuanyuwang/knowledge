# CONVI-7777: Assistance API historical scan and time-semantics investigation

**Local implementation follow-up:** Requested-range bounds and the Notes/Workflow conditional source/payload switch are implemented locally and pass focused Go/Bazel verification. This audit retains the pinned pre-change findings below. See [follow-up and concrete examples](convi-7777-time-source-parity-follow-up.md). No push or deployment.

**Evidence date:** October 21, 2026 (America/Toronto). **Pinned main:** `b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb`. **Pinned PR #33993:** `0d1b75da684ee0a9237020d6d500e117db6f7b4e`. **Scope:** ten existing assistance/adjacent APIs, both Hint implementations, the new seven-family evidence path, and its shared ConversationStats population.

The existing assistance APIs inspected have bounded source scans; all seven new Assistance Usage evidence families omit timestamp bounds. The larger parity problem is that equal SQL intervals do not guarantee equal evidence, source history or bucket attribution. The scan baseline is the requested [S,E) interval used by existing assistance APIs; timestamp-copy inconsistencies are separate correctness findings.

## Contract and evidence limits

For an authorized request [S,E), eligible conversations must remain governed by the existing population and request filters. Feature lookup must return qualifying use only for those conversation/agent pairs. Scan bounds must control physical work without silently redefining eligibility or discarding supported evidence. This investigation does not change product code, feature predicates, ticket status, deployment or production data.

Fresh local evidence: 83 generated SQL variants per reader configuration; 381 synthetic SQL executions with zero query errors; 107 explicit result/mechanism assertions. All probes use ClickHouse 24.2.3.70 on a disposable local container. Memory tables reproduce query semantics; a separate 180,000-row single-node MergeTree fixture measures scan pruning. These are not production latency, distributed transfer, deployed schema, customer prevalence or retention evidence. GenAI/Summary differing main and PR readers were separately compiled and compared on focused scenarios; the other inspected feature readers/helpers are identical at the pinned revisions.

The Go diagnostic uses a compiler overlay of pinned sources and leaves all source worktrees untouched, including concurrent edits. Real API filter/query builders produce the captured SQL. Most semantic probes execute generated SQL directly, not the full RPC; a manually supplied eligible-pair table isolates feature lookup. The missing-message and version-selection probes execute the actual population query separately. Synthetic states show behavior when the state exists; they do not show customer frequency.

## Cross-API scan matrix

| Feature | API | Sources | Clock | Existing scans | New API | Remaining issue |
|---|---|---|---|---|---|---|
| KB Search | RetrieveKnowledgeBaseStats | Conversations: conversation_event_d | Copied conversation start | Exact [S,E) | No time bounds; final pair intersection | Evidence copied outside the request can disappear in legacy; raw usage/event units differ from distinct conversation use. |
| Chat and email Suggestions | RetrieveSuggestionStats | Conversations: conversation_event_d | Copied conversation start | Exact [S,E) | No time bounds; V1 uses types 4/5/10 only | Email types 25/27 share this bounded reader but remain excluded from the new V1 union. |
| Notes | RetrieveNoteTakingStats | Events normally; Conversations with metadata | Copied conversation start on both conversation and event | Exact [S,E) on both sources and metadata | Events only; both sources unbounded | A missing Events conversation copy suppresses valid events; adding matching metadata can switch legacy results back on. |
| Guided Workflow | RetrieveGuidedWorkflowStats | Events normally; Conversations with metadata | Copied conversation start on both conversation and event | Exact [S,E) on both sources and metadata | Events only; both sources unbounded | Same dual-copy/cluster issue as Notes. Sessions and document payloads remain feature-specific. |
| GenAI | RetrieveKnowledgeAssistStats | Conversations: message, action and moment annotations | Copied conversation start on all three | Exact [S,E) on all three and metadata | Only qualifying use moments; unbounded | Legacy usage is anchored to message group/bucket, not conversation ID; usage-only buckets disappear. |
| Summary | RetrieveSummarizationStats | Conversations: conversation, generation action, use event | Copied conversation start on all three | Exact [S,E) on all three and metadata | Generation + copy sources unbounded | Exact timestamp equality is still required inside the feature; legacy result also anchors on conversation buckets. |
| Behavioral/Checklist/KB/Workflow hints | RetrieveHintStats | Conversations: moments, actions, events | Copied conversation start on all branches | Exact [S,E), including nested action-mode moment subquery | Followed moments + hint-use events unbounded | Both sent implementations are bounded; sent/followed rows can still use different copied buckets or agents. |
| Chat Smart Compose (P2) | RetrieveSmartComposeStats | Events: conversation_d + analytics_event_d | Conversation copy uses start; analytics event uses actual event_time | Both clocks use exact [S,E) | Excluded from V1 | Agent/hour intersection misses later-hour insertion unless that agent starts another qualifying conversation in that hour; it does not prove conversation attribution. |
| Legacy combined assistance | RetrieveAssistanceStats | Conversations: messages, moments, events | Copied conversation start | Exact [S,E) on all sources and metadata | Separate legacy endpoint | Message aggregate anchors output; a profile deprecation flag may skip the API entirely. |
| Adherence / related hint views | RetrieveAdherences | Conversations: moment_annotation_d, scanned twice | Copied conversation start | Exact [S,E); parsed metadata lists are discarded | Separate adjacent API | Nonmatching metadata does not narrow the reproduced result. Positive behavior adherence alone does not prove a delivered assistance use. |
| Conversation population | RetrieveConversationStats shared builder / new API denominator | Conversations: conversation/message, optional labels and metadata | Copied conversation start for this new API | Exact [S,E) | The final cohort also gates all seven evidence queries | No message means no eligible pair. Filtering before latest-row selection can retain an old in-range version after a newer version moved outside. |

## Findings

### T1 — Seven new evidence families scan historical data before eligibility intersection

**Classification:** Confirmed scan behavior; production cost unknown. **Action:** Address in CONVI-7777 before broad rollout.

**Trigger:** One-day request, one eligible conversation/agent, months of feature history for the same agent.

**Accepting/missing guard and trace:** The evidence constructor supplies only an agent predicate. Feature rows are reduced to pairs before the ordinary join with assistance_usage_population. The final join excludes other conversations from the answer but cannot be assumed to prune all preceding work.

**Observed result:** A single-node synthetic MergeTree with 180,000 history rows returned the identical eligible pair with unbounded read_rows=180,002 and exact request bounds=2,050. Counts include the two tiny lookup tables.

**Impact and limit:** More history can increase source reads, pair reduction, memory and transfer even for a short request. This is demonstrated locally, not a measured production regression.

**Fix boundary:** Apply the requested [S,E) interval consistently to every physical evidence source, following the existing assistance APIs, and retain the final eligible-pair intersection. Audit copied timestamps and internal matching separately from source-scan alignment.

**Regression:** Seed selected-agent history far outside the request, in-range evidence, out-of-cohort pairs and skewed copied timestamps. Assert exact results and inspect each physical read/remote plan; benchmark representative deployed data.

### T2 — Copied timestamps can disagree under the same requested interval

**Classification:** Verified existing time-semantics difference; prevalence unknown. **Action:** Track separately from requested-range scan alignment.

**Trigger:** Conversation start changes after an evidence row copied the earlier value, or independent projections are refreshed at different times.

**Accepting/missing guard and trace:** Events and Conversations writers copy the conversation start field, independently of event creation/ingestion time. The upsert-message path can move StartedAt earlier based on a message timestamp. No maximum correction delta is enforced in that inspected loop.

**Observed result:** Synthetic evidence with copied start outside [S,E) is excluded by existing APIs but retained by the current new pair-based lookup when its cohort pair is supplied. Late arrival with an unchanged in-range copied start remains included in both.

**Impact and limit:** Late arrival and copied-time disagreement are different states. Applying the same request interval aligns source bounds but does not make independently copied timestamps agree. The current unbounded new lookup and existing bounded APIs can therefore return different evidence.

**Fix boundary:** Use the existing requested-range behavior as the scan baseline. Document and investigate timestamp-copy, correction and reindex behavior as a separate correctness question; no fixed-day expansion is proposed.

**Regression:** Test copied timestamps before/inside/at the end of the request, unchanged copied time with late creation, and a conversation moved into/out of the request. Keep synthetic cases distinct from observed production data.

### T3 — Summary still loses use on an internal exact timestamp join

**Classification:** Reproduced on main reader and PR reader; inherited behavior. **Action:** Resolve explicitly; a wider scan alone cannot fix it.

**Trigger:** Generation action at copied start October 1 00:00 and a qualifying copy at October 1 01:00, with the same conversation/agent. Both timestamps are inside the request.

**Accepting/missing guard and trace:** The feature joins valid generation to use USING (conversation_id, conversation_start_time, agent_user_id), before the new API can apply its pair intersection.

**Observed result:** Legacy used_summarization_count=0; new Summary evidence returns no pair. Generation count remains 1. Expanding source ranges cannot make unequal join keys equal.

**Impact and limit:** A conversation can lose Summary-use credit despite having both qualifying records. Aligning source bounds cannot resolve unequal internal join keys.

**Fix boundary:** Choose whether timestamp equality is an intended feature contract. If it is removed, preserve conversation/agent identity and decide whether generation-to-copy linkage needs a stronger annotation/session key. Keep any legacy behavior correction separate from a scan-only change.

**Regression:** Same pair, differing copied times inside or outside the interval; unrelated generation/copy; wrong agent and duplicate generations.

### T4 — Time-bucket anchors can hide usage even when both source scans cover it

**Classification:** Reproduced on main and PR legacy readers; new evidence lookup differs. **Action:** Cross-API parity decision and regression coverage.

**Trigger:** Two-day request. Conversation/message copy is October 1; usage annotation or generation/copy pair is October 2; no message/conversation aggregate exists for that agent on October 2.

**Accepting/missing guard and trace:** GenAI left-joins usage aggregates to message aggregates by agent/time bucket. Summary left-joins summary aggregates to conversation aggregates. Neither join proves conversation identity across those buckets.

**Observed result:** Legacy daily GenAI use is 0 and Summary use is 0, even with a two-day source interval that contains all rows. The new GenAI and Summary evidence queries return the eligible pair; new output buckets use population time. Notes/Workflow instead emit their event-copy day.

**Impact and limit:** Identical source scan ranges do not guarantee matching totals or trend placement. The legacy combined assistance API has the same message-bucket anchoring pattern.

**Fix boundary:** State whether the product groups adoption by population conversation start or feature-source copy. For conversation adoption, carry the pair plus chosen population time through aggregation; do not borrow legacy aggregate totals as the numerator.

**Regression:** Same pair copied across midnight; UTC and non-UTC day/week boundaries; missing anchor bucket; include/exclude metadata; Agent and Team totals.

### T5 — Notes and Workflow depend on cluster choice and a second conversation copy

**Classification:** Reproduced existing dependency and cross-API parity gap. **Action:** Include source coverage in CONVI-7777 validation.

**Trigger:** Conversations contains the eligible conversation and matching typed evidence. Events has the JSON usage event but lacks its conversation row; matching metadata is also available.

**Accepting/missing guard and trace:** Without metadata, legacy Notes/Workflow read Events and inner-join events to Events conversation_d by conversation ID. With metadata, legacy switches to Conversations. New Notes/Workflow always use the Events branch, even when the cohort was filtered with metadata in Conversations.

**Observed result:** Legacy no-metadata and new evidence queries return no Notes/Workflow usage. Adding a matching metadata filter causes both legacy endpoints to return 1 via Conversations.

**Impact and limit:** The same date range can cover different stored histories and return different usage merely because the selected source changes. Padding timestamps cannot restore a missing Events conversation row.

**Fix boundary:** Verify historical coverage, ingestion/retention/reindex behavior for both projections. Decide whether the new endpoint should require a second Events conversation record after cohort eligibility is already established. If removing that join, review source/agent/metadata gates explicitly.

**Regression:** One cluster missing conversation or event; copied times disagree; matching metadata changes source; wrong-agent conversation rows; typed versus JSON payloads.

### T6 — The eligible population still filters copies before selecting the latest version

**Classification:** Reproduced inherited population behavior; not a new evidence-query bug. **Action:** Track separately from source-scan optimization.

**Trigger:** An older conversation row starts October 1; a newer row for the same pair starts October 3; request is October 1. An in-range message copy remains.

**Accepting/missing guard and trace:** The population applies exact time bounds in the conversation source WHERE, then ORDER BY update_time DESC LIMIT 1 BY pair. The newer out-of-range row is removed before version selection. Message copies are also independently range-filtered.

**Observed result:** The population still returns the October 1 pair. A separate missing-message probe returns no population despite feature evidence.

**Impact and limit:** Calling this cohort globally current would overstate its contract. Widening only feature scans cannot recover a pair excluded by a stale message copy or correct an old version retained by prefiltering.

**Fix boundary:** Keep this separate from CONVI-7777 scan-only changes unless latest-state semantics are explicitly required. Define bounded candidate discovery versus global latest-state selection and preserve frozen ConversationStats behavior until reviewed.

**Regression:** Newest version moves outside/inside the range; message and conversation disagree; same pair with multiple versions; new agent assignment; labels ON/OFF paths.

### T7 — Whole-second SQL formatting changes fractional request boundaries

**Classification:** Reproduced shared boundary behavior. **Action:** Separate shared filter correction.

**Trigger:** Requested interval is [October 1 00:00:00.500, 00:00:01.500).

**Accepting/missing guard and trace:** The common timestampFormat is 2006-01-02 15:04:05. Both request endpoints are formatted without fractions before source conditions are built.

**Observed result:** A KB event at 00:00:00.250 is included although it precedes the request; one at 00:00:01.250 is excluded although it is inside. The query uses [00:00:00,00:00:01).

**Impact and limit:** Exact DateTime64 membership at API boundaries is not preserved. Normal whole-day UI requests do not trigger this specific case; direct fractional requests can.

**Fix boundary:** Use typed timestamp arguments or a format retaining required precision consistently across source and metadata filters. Do not fold this shared behavior correction into an unreviewed window change.

**Regression:** Fractional lower/upper endpoints, zero-width ranges, microsecond-before/at end, and equivalent metadata-filter requests.

### T8 — Adjacent APIs have different attribution and filter contracts

**Classification:** Smart Compose source-observed; adherence metadata gap reproduced. **Action:** Document exceptions and track separate corrections.

**Trigger:** A Smart Compose insert occurs in a later hour than conversation start, or an adherence request contains nonmatching metadata.

**Accepting/missing guard and trace:** Smart Compose filters analytics_event.event_time and joins to agent/hour of conversation starts. RetrieveAdherences discards the parsed metadata inclusion/exclusion lists.

**Observed result:** All inspected scans remain time-bounded. Smart Compose attribution cannot be equated to conversation-level membership from these joins. The adherence fixture survives a nonmatching metadata filter with the same response as no filter.

**Impact and limit:** A uniform historical scan range cannot make event-time metrics, conversation-start metrics and metadata semantics equivalent. Silence/Hold or P2 features need explicit contracts before admission into the new union.

**Fix boundary:** Keep Smart Compose and email additions outside V1 as scoped. When revisiting them, require conversation-linked actual-use evidence and explicit filters. Define whether adherence metadata should be honored or rejected, then address that API separately.

**Regression:** Long conversation with later-hour insert; another conversation in that hour; same text twice; nonmatching metadata for adherence; missing delivered-action linkage.

## Implementation boundary and acceptance criteria

1. Agree on four distinct contracts: exact eligible conversation range; evidence source scan range; chosen output bucket time; and historical availability/retention by source. The same request should use a consistent scan policy across all participating feature sources, with justified exceptions for different clocks. A common scan span is not itself proof of equal results.
2. Use the requested [S,E) interval for evidence source scans, matching existing assistance APIs, and keep the population interval unchanged. Apply bounds to annotations, events, required conversation copies, and supporting metadata reads where applicable. An eligible-agent predicate or final cohort join is not a substitute for physical source bounds.
3. Investigate copied timestamp disagreement, ingestion/reindex behavior, retention and missing-copy prevalence separately. These checks explain cross-API completeness differences; they do not introduce an additional fixed-day scan window.
4. Keep a semantic matrix for every family: aligned time; late arrival with unchanged copy; copied-time disagreement before/inside/after the interval; exact boundaries; generation/use disagreement; population/message disagreement; cross-day/week buckets; metadata include/exclude/both; missing cluster copy; wrong agent; duplicates; source/bot/dev-user gates; and Agent/Team/overall attribution. Validate rare cases that feature-internal joins can still drop.
5. Validate source bounds in generated SQL and actual physical/remote plans. On deployed ClickHouse/version/data, record read_rows, read_bytes, memory, result identity and network/remote work for representative 1-day, 7-day and 30-day requests, narrow and broad agent audiences, and concurrent calls. Do not treat fewer returned pairs or green semantic tests as production performance proof.
6. Preserve legacy RPCs in the scan-only change. T2–T8 are inherited semantics or separate parity decisions; migrate/fix them only with explicit scope and independent regression evidence. Add the findings to follow-up tracking without claiming that separate Linear tickets or owners already exist.

## Known related issues

The existing assistance issue register remains relevant: metadata exclusion with non-nullable LEFT JOIN defaults (AA-01), Summary payload-type/copy semantics (AA-02), Notes metadata projection/event-enum mismatch, per-agent attribution and denominator contracts. The current exclusion fixtures again return empty results under join_use_nulls=0 despite no excluded row. This inherited issue remains independent of source-scan alignment. The synthetic Notes metadata fixture deliberately uses the query’s expected type 5 to isolate the cluster dependency; production projection enum parity is a separate known concern.

## Non-blocking maintainability

Pass requested-range bounds consistently through all seven source builders. Keep population eligibility and evidence-source filtering explicit. Document each source clock and cluster selection beside its query contract. Keep these naming/design suggestions separate from demonstrated defects.

## Recommendation

Align all seven evidence families with the existing assistance APIs by applying the requested [S,E) interval to every participating source. Validate the resulting behavior and benchmark the deployed query shape. Track timestamp-copy, join, bucket and historical-availability gaps as separate correctness findings. The current new API is logically cohort-filtered but does not yet have a bounded evidence-scan contract. Existing feature APIs avoid that particular unbounded-scan shape but have reproduced time/copy/bucket inconsistencies that need separate decisions.

## Sources

- [Shared interval bounds](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/insights-server/internal/analyticsimpl/common_clickhouse.go#L320): The default source interval is start-inclusive and end-exclusive. The formatter at line 37 keeps whole seconds.
- [All seven new evidence queries](https://github.com/cresta/go-servers/blob/0d1b75da684ee0a9237020d6d500e117db6f7b4e/insights-server/internal/analyticsimpl/action_retrieve_conversation_assistance_usage_stats_clickhouse.go#L27): Only eligible agents are applied inside feature sources; eligible pairs are joined after feature reduction. No source timestamp bounds are supplied.
- [Notes cluster switch](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/insights-server/internal/analyticsimpl/retrieve_note_taking_stats_clickhouse.go#L91): Metadata requests switch from Events to Conversations, including a different payload projection.
- [Workflow cluster switch](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/insights-server/internal/analyticsimpl/retrieve_guided_workflow_stats_clickhouse.go#L82): Metadata requests switch from Events to Conversations.
- [Summary exact timestamp join](https://github.com/cresta/go-servers/blob/0d1b75da684ee0a9237020d6d500e117db6f7b4e/insights-server/internal/analyticsimpl/common_summarization_usage_clickhouse.go#L5): Generation and use must agree on conversation, agent and copied start time. This is preserved from main.
- [GenAI aggregate joins](https://github.com/cresta/go-servers/blob/0d1b75da684ee0a9237020d6d500e117db6f7b4e/insights-server/internal/analyticsimpl/retrieve_knowledge_assist_stats_clickhouse.go#L48): Usage joins message aggregates by requested grouping keys, including time buckets when requested.
- [Population source and version selection](https://github.com/cresta/go-servers/blob/0d1b75da684ee0a9237020d6d500e117db6f7b4e/insights-server/internal/analyticsimpl/common_conversation_population_clickhouse.go#L35): Both message and conversation copies receive exact bounds. Conversation time filtering occurs before latest-row selection at lines 79-94.
- [Smart Compose hour matching](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/insights-server/internal/analyticsimpl/retrieve_smart_compose_stats_clickhouse.go#L38): Actual event time is filtered; attribution uses agent/hour instead of a conversation ID.
- [Adherence metadata discarded](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/insights-server/internal/analyticsimpl/retrieve_adherences_clickhouse.go#L83): The reader discards parsed include/exclude metadata lists. Source timestamps still have bounds.
- [Earlier messages can change start time](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/apiserver/internal/conversation/action_batch_upsert_conversation_messages.go#L1954): The inspected loop moves start time to an earlier message timestamp without a local maximum-delta guard.
- [Events copy conversation start](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/shared/clickhouse/events/event.go#L540): ConversationStartTime comes from the supplied conversation, while CreateTime is ingestion wall clock.
- [Conversations projection copies start](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/shared/clickhouse/conversations/conversation.go#L2505): Annotations and events copy conversation.StartedAt; their own creation time is a different column.
- [Legacy combined API](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/insights-server/internal/analyticsimpl/retrieve_assistance_stats_clickhouse.go#L217): Message, moment and event scans receive time bounds; results are anchored on message aggregates.
- [Conversation source sort key](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/shared/clickhouse/testing/schemas/ch_conv_schema.sql#L27): Base source tables are time-led. Do not assume every base table has monthly partitions; several by-conversation/metadata views do.
- [Events source sort key](https://github.com/cresta/go-servers/blob/b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb/shared/clickhouse/testing/schemas/ch_events_schema.sql#L18): Events source keys begin with copied conversation start time; analytics_event uses event_time instead.

## Reproduction artifacts

All source snapshots, generator overlays, captured queries, SQL runners, results and verification are saved in `/Users/xuanyu.wang/repos/knowledge/analytics/sessions/2026-10-21/convi-7777-assistance-scan-audit`. Entry points: `generate_scan_queries_test.go`, `generate_scan_queries_main_readers_test.go`, `run_sql_probes.py`, `run_scan_pruning.py`, `verify_probe_results.py`, `generated-queries.json`, `query-scan-inventory.json`, `probe-results.json`, `scan-pruning-results.json`, and `verification.json`. SQL probes use only synthetic rows in uniquely named local databases and clean up those databases. The local container uses default local authentication; no AWS, Okta, Azure or SSH credential/profile was selected or read. GitHub reads used gh/git self-authentication.

Report validation: the original report rendered and its controls passed browser checks. After the requested-range revision, static HTML/JavaScript, two displayed scan variants and companion links pass; all fifteen pinned anchors refer to saved sources. Visual rechecking of the revised file was blocked by the browser file-URL policy. See report-qa.json in the evidence directory.
