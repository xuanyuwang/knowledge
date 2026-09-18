# PR #31335 adversarial correctness review — 2026-09-08

Subsequent implementation: all four findings were fixed in separate local commits, with a fifth test-cleanup commit. See the [validated fix session](../sessions/2026-09-08/codex-convi-7402-review-fixes.md). The review below preserves the findings against its original snapshot.

## Actionable findings

All four findings have high confidence. P2 means a correctness issue that should be addressed; findings 2–4 distinguish existing helper limitations from the new code that incompletely rejects or exposes them. No production fixes were made.

### 1. [P2] Apply the submit-time correction to QA conversations too

**Location:** [retrieve_qa_conversations_clickhouse.go:328](/Users/xuanyu.wang/repos/go-servers-convi-7402/insights-server/internal/analyticsimpl/retrieve_qa_conversations_clickhouse.go:328), specifically the `req.FilterByTimeRange` argument at line 329.

**Trigger:** `RetrieveQAConversations`, `time_range_filter_target=SUBMIT_TIME`, January window, one metadata group with `moments=[A]`, `excluded_moments=[A]`, selected value `X`. A conversation starts December 28, has A=`Y`, and its scorecard is submitted January 5.

**Expected:** Reject that conversation: A exists and its value is not X. **Actual:** Both metadata CTEs filter `conversation_start_time` to January. The existing annotation disappears from both, so the missing branch admits the conversation. The function already computes `conversationTimeRange=nil` at lines 300–303 but does not pass it to the moment parser. The corresponding score-stats call does use it. This is the same new over-inclusion mode the prior fix addressed, left in the sibling endpoint.

**Reproduction actually run:** `TestReview7402SubmitTimeSQL` mirrors both production builders' parser arguments and exports their full SQL. ClickHouse 26.8.2.7 executed both against identical three-conversation fixtures. Conversations returned `missing`, `old-selected`, and **`old-other`**; stats returned weight sum 2 and conversation/scorecard counts **2**. The disallowed third row should not appear. See `execute-submit-conversations.sql`, `execute-submit-stats.sql`, and matching results in the evidence directory.

**Why tests can pass:** Existing submit-time golden cases have no overlap; overlap goldens use conversation time. The new nil-time parser test covers the helper, not whether both endpoints pass nil. Both QA suites passed while this bug remained.

**Durable test/fix direction:** Add an endpoint-level submit-time plus overlap regression containing conversations outside the start-time window, then execute its SQL. Pass the effective conversation time range consistently to both moment parsers. Do not apply submit time to annotations themselves; it is a scorecard axis.

### 2. [P2] Reject all unsupported shared overlap shapes before discarding group structure

**Location:** [common_clickhouse.go:631](/Users/xuanyu.wang/repos/go-servers-convi-7402/insights-server/internal/analyticsimpl/common_clickhouse.go:631), singleton guards at lines 632–633.

**Trigger:** A shared-parser endpoint such as Conversation Stats receives `moments=[A]`, `excluded_moments=[A,B]`, metadata value X. ES defines this as `(A=X OR missing(A)) AND missing(B)`.

**Expected:** Preserve that predicate, or explicitly reject this unsupported shape. **Actual:** The newly added rejection guard requires exactly one include and one exclusion. The existing lowering then produces only the A=X include and drops both exclusions. With includes `[A,B]` and exclusion `[A]`, it produces only `missing(A)`. With `[A,B]` on both sides, it produces **no moment filters at all**. Thus the new rejection is incomplete; larger instances of the same unsupported OR still silently return incorrect results. These larger-shape errors predate the PR, and this finding concerns the partial shared-helper repair, not a claim that all were newly introduced.

**Reproduction actually run:** `TestReview7402SharedMultiOverlap` has three expected-rejection cases. All failed: observed include/exclude filter counts `(1,0)`, `(0,1)`, `(0,0)`, respectively, with no error.

**Why tests can pass:** The added shared-parser test exercises only `[A]/[A]`. QA's stricter parser tests do not cover the different shared parser or its consumers. An SQL golden built from already-dropped filters cannot recover the missing identity/group semantics.

**Durable test/fix direction:** Parameterize cardinality, intersecting IDs, and types in shared-parser rejection tests; include `[A]/[A,B]`, `[A,B]/[A]`, and `[A,B]/[A,B]`. Validate mixed groups before lowering to separate include/exclude lists. Reject unsupported overlap across the full ID sets; only support it later with a representation that retains OR.

### 3. [P2] Preserve numeric-bin endpoint flags in the newly supported overlap path

**Location:** New call at [common_clickhouse.go:1109](/Users/xuanyu.wang/repos/go-servers-convi-7402/insights-server/internal/analyticsimpl/common_clickhouse.go:1109); existing converter at [common_clickhouse.go:1559](/Users/xuanyu.wang/repos/go-servers-convi-7402/insights-server/internal/analyticsimpl/common_clickhouse.go:1559).

**Trigger:** Same-ID metadata overlap, `numeric_bin={from_value:5,to_value:5,to_value_is_inclusive:true}`; conversations with A=5 and missing A. Another case is `(5,10]` using both endpoint flags.

**Expected:** `[5,5] OR missing` selects A=5 and missing A. `(5,10] OR missing` excludes 5 and includes 10. **Actual:** Conversion always uses `>= from AND < to`. The singleton becomes unsatisfiable, leaving only missing values; `(5,10]` becomes `[5,10)`. This is an existing converter defect newly exposed to overlap requests that previously returned an error. ES's `ConvertNumericBinToESRange` honors both flags; Director explicitly normalizes singleton upper bounds to inclusive.

**Reproduction actually run:** Generated SQL from the real parser/helper was executed in ClickHouse. Singleton returned only `missing`. Exclusive/inclusive interval returned `five, missing, six`, whereas the fixture expectation was `missing, six, ten`. Raw parser assertion on the singleton also failed.

**Why tests can pass:** Added numeric golden bins use the default `[from,to)` flags and nonzero width. No singleton or nondefault-boundary case is checked.

**Durable test/fix direction:** Choose operators from both flags. Add fixtures at both endpoints, just inside/outside, and a singleton, with and without the missing branch. Compare with ES range construction on the same bin. Full live ES execution was not performed in this review.

### 4. [P2] Reject an empty metadata value before building the overlap CTE

**Location:** [common_clickhouse.go:1092](/Users/xuanyu.wang/repos/go-servers-convi-7402/insights-server/internal/analyticsimpl/common_clickhouse.go:1092); empty predicate originates at lines 1566–1584.

**Trigger:** Same-ID metadata overlap with `metadata_value_attributes=[{metadata_value:{}}]` (the value oneof is unset).

**Expected:** Explicit `InvalidArgument` for unsupported input. **Actual:** The new parser checks only that the attribute slice is nonempty; the converter adds an empty condition. SQL contains an empty tuple in its boolean expression. ClickHouse returns **Code 43 / ILLEGAL_TYPE_OF_ARGUMENT: Illegal type (Tuple()) of 2 argument of function and**. The new overlap path accepts an input that previously failed the mixed-group guard. The converter's include-only validation gap already existed.

**Reproduction actually run:** `TestReview7402ExportOverlapSQL/empty-oneof` failed its expected-rejection assertion; `execute-generated-empty-oneof.sql` failed in ClickHouse with Code 43.

**Why tests can pass:** Existing cases use populated string/number values or reject different IDs/types. Neither parser nor SQL golden coverage includes a present MetadataValue message with an unset value.

**Durable test/fix direction:** Validate actual oneof variants and ensure a nonempty predicate before accepting the overlap. Add empty message, empty oneof, and mixed valid/invalid attribute cases; require validation errors before database access.

## Additional validation failure

The broad focused run also fails **`TestRetrieveQAScoreStatsClickhouseQuery_FilterByConversationOutcomeMomentGroup`**, at `retrieve_qa_score_stats_test.go:258`. Its `require.Contains` expects six tabs before `moment_annotation_d`; extracted `buildQAMomentFilterQuery` now emits four. This is a test-maintenance regression, not evidence that outcome SQL is invalid. Keep the test, but normalize whitespace or assert structure. The query extraction changed formatting without adapting this assertion. CI was still running when last checked; it is not valid to call the entire relevant test set green.

## Reviewed revisions and rebase

- Remote PR head verified twice: `1b2a95bd9a21c2453cd70609ebd20c36f0365ed7`.
- Fetched `origin/main`: `98583180ee54f812a19390fbd356afa87cc24fce`.
- Initial source worktree was clean. Rebased the local named PR branch onto fetched main, with **no conflicts**. New local head: `12117463df3318998e944551b63913c6110ba6b6`.
- `git range-diff 40e5619fbf..1b2a95bd9a origin/main..HEAD` marked **all ten patches equivalent**. No PR-patch semantic changes were introduced by conflict resolution (none was needed). Latest main adds independent analytics fixes and newer dependencies. Findings 1–4 exist in the remote head as well; the implicated common/conversations files are identical between reviewed local and remote heads.
- No pushes, comments, review submissions, or thread resolutions. Temporary source tests were copied to the evidence directory and removed; final source worktree is clean. The local rebased history is intentionally retained.

## Semantic contract and coverage matrix

Let `V(A)` mean selected latest metadata value and `M(A)` mean no annotation for template A. Missing means annotation absence, not an empty string, false, numeric zero, or a SQL NULL-valued field. Separate groups are AND; selected values inside a group are OR. Metadata's special overlapping include/exclude encoding means `V(A) OR M(A)`. Remaining non-overlapping exclusions are AND-ed outside that OR in ES. QA deliberately supports only one metadata ID per mixed group.

| Case | Expected / intended behavior | Inspected or actually tested |
|---|---|---|
| No groups | No moment constraint | Parser/query source; existing tests |
| String include-only; multiple selected values | Latest A in selected set | Existing parser tests; full SQL fixture |
| Missing-only A | No annotation for A | Existing parser tests; source |
| Singleton metadata overlap | `V(A) OR M(A)` | Existing tests; full SQL, passes basic string fixtures |
| Overlap + independent include + independent exclusion | `(V(A) OR M(A)) AND V(B) AND M(C)` | Both full committed goldens executed; fixture passes |
| Multiple overlap groups | `(V(A) OR M(A)) AND (V(B) OR M(B))` | CTE indexing and AND construction inspected; no dedicated runtime fixture |
| Present disallowed value | Excluded, never treated as missing | Full SQL fixture passes on normal time axis; submit-time conversations fails |
| Latest value changes; duplicate rows | Latest value wins; no duplicate counts | Two update directions and duplicate annotation executed; passes |
| Equal latest timestamps | Needs deterministic semantic winner | Source/schema risk only; not proven in production |
| Boolean, numeric scalar, empty string | Literal values, distinct from missing | Conversion/source inspected; no dedicated full runtime fixture |
| Numeric default intervals | Lower inclusive, upper exclusive | Committed full SQL fixture passes |
| Inclusive/exclusive endpoints and singleton | Honor both flags | Parser + generated SQL executed; finding 3 |
| Same-ID mixed group with no metadata attrs | Reject in QA | Guard inspected |
| Same-ID mixed group with empty value oneof | Reject | Parser + ClickHouse executed; finding 4 |
| Mixed groups with different IDs / actual outcome moments | Explicit rejection in QA | Existing rejection tests pass; ES supports broader mixed groups |
| Multi-ID mixed shared groups | Preserve ES boolean contract or reject | Three parser repros fail; finding 2 |
| Unsupported include-only multi-ID / nonmetadata shapes | Should reject or retain semantics | Existing QA/shared parser can ignore them; pre-existing limitation, not fixed here |
| Flag `CLICKHOUSE_ENABLE_EXCLUDED_MOMENTS=false` | Existing rollout behavior disables exclusions | Source inspected: overlap reduces to include-only; no flag-off runtime test |
| Conversation start window | `[start,end)` on annotation conversation start | Source + full golden execution |
| Conversation end window | Raw annotation table, end-time column | Existing fallback parser test passes; generated raw CTE executes (unbounded fixture) |
| Scorecard submit window | Scorecard submit membership; no conversation-time restriction on moments | Both full generated queries executed; finding 1 |
| Submit-time plus ended-at | Reject incompatible axes | Existing option test passes |
| Metadata/CLO MV routing | Metadata MV for start; raw for end; CLO MV additionally feature-gated | Parser tests and source inspection; raw metadata CTE executed |
| Join NULL setting | Overlap works with empty defaults or NULL via `ifNull` | Default-setting runtime tests only; independent exclusions retain pre-existing empty-string dependency |
| Aliases, CTE names, placeholders, aggregation | Unique overlap IDs; args follow SQL order; no fanout | Full golden execution and builder inspection; no ambiguity reproduced |
| Pagination | Stable page membership required | Existing ordering inspected; ties lack unique score ID, pre-existing residual risk |

Time axes: event/message time belongs to the shared endpoint's event tables; annotation `create_time` (raw) and `update_time` (MV) choose the latest value; neither is scorecard submit time. Score data uses mapped `scorecard_time`/conversation-end columns or explicitly `scorecard_submit_time`. The submit-time correction should remove annotation conversation-time bounds, not filter annotation ingestion/update time to the submission window.

## Shared-helper blast radius and execution trace

QA paths: request normalization, parent/user/ACL/template filtering → QA common/scorecard/score/conversation parsers → moment parser → per-group source representation → CTE builders → bound ClickHouse query → row scanning/aggregation. Both QA endpoints are ClickHouse-backed; an invalid filter or query error is returned, not transparently rerun in ES. Respect-template score stats calls the same reader per template group. The score-stats routing now considers overlap filters when choosing the metadata-capable query. Value joins and existence sets use DISTINCT, avoiding annotation fanout before sums/counts. Conditions use positional `?` parameters; no placeholder numbering exists here. Builder argument order was checked and full SQL was executed after the repository's test interpolation.

`parseMomentConditionsForQAAttribute` has exactly two production consumers: QA score stats and QA conversations. `buildQAMomentFilterQuery` is used for ordinary and overlap includes in both, explaining why the extraction also affects ordinary outcome tests.

`parseClickhouseFilter` has **16 production callers**. Eleven use its moment filter lists: agent, conversation, hint, suggestion, guided-workflow, note-taking, knowledge-base, assistance, summarization, knowledge-assist, and customer-snapshot stats. They lower separate include/exclude lists through metadata builders (or endpoint equivalents), preserving AND between lists but having no OR-group representation. Five discard the returned moment lists: manager stats, scorecard stats, live-assist stats, smart-compose stats, and retrieve-adherences. All encounter the new singleton rejection before that discard. General metadata support on the latter five is a pre-existing separate gap. The shared guard's blast radius therefore exceeds the two QA APIs.

`convertMetadataMomentGroupFilter` and excluded metadata conversion are consumed by the QA parser and shared parser; outcome conversion is used by QA. The zero-time guard makes unbounded moment lookup possible for submit-time stats. Metadata MV routing does not fall back to raw on a missing/stale MV; the config-read fallback comment concerns outcome MV selection.

## Commands/tests actually run

All Go runs were in `/Users/xuanyu.wang/repos/go-servers-convi-7402`, with dependency-network use disabled for subsequent runs (`GOPROXY=off GONOPROXY=none GOTOOLCHAIN=local`). The first run fetched the newer proto dependency through the configured Git transport before compiling.

| Command / check | Result |
|---|---|
| `git fetch origin main convi-7402-bswift-users-getting-qa-score-stats-errors` with explicitly selected cleared SSH key | Success |
| `gh pr view`, `gh api repos/cresta/go-servers/pulls/31335/comments` | Head, reviews/comments and checks read; no external writes |
| `git rebase origin/main`; range-diff above | Success; no conflicts; ten equivalent patches |
| `go test ./insights-server/internal/analyticsimpl ./insights-server/internal/analyticsimpl/elasticsearch -run 'Test(ParseMomentConditionsForQAAttribute\|ParseClickhouseFilter\|RetrieveQAScoreStatsClickhouseQuery\|.*Overlap)' -count=1` | Analytics fails the whitespace assertion above; this regex selected no ES tests |
| `go test ./insights-server/internal/analyticsimpl -run '^(TestParseMomentConditionsForQAAttribute.*\|TestParseClickhouseFilter.*\|TestQATimeRangeFilterTargetOptions\|TestRetrieveQAScoreStatsClickhouseQuery_FilterByMetadataMomentGroups_OverlapValuePlusNoValue)$' -count=1` | Pass, 1.474 s |
| `go test ./insights-server/internal/analyticsimpl/elasticsearch -run '^TestConvertConvoMomentGroup_IncludedAndExcludedSameMoment$' -count=1` | Pass, 1.013 s; parser structure test, not live ES |
| `env -u TEST_DATABASE_URL ... go test ./insights-server/internal/analyticsimpl -run '^(TestRetrieveQAConversations\|TestRetrieveQaScoreStats)$' -count=1 -timeout=3m` | Pass, 17.978 s; embedded PostgreSQL and mocked ClickHouse |
| Temporary `TestReview7402SharedMultiOverlap` / `TestReview7402ExportOverlapSQL` | Five expected-contract assertions fail: three multi-ID cases, singleton bound, empty value |
| Temporary `TestReview7402SubmitTimeSQL` | Pass; exports full queries mirroring both production paths |
| `clickhouse local --multiquery --queries-file <evidence SQL>` via saved fixture scripts | Both exact goldens execute; basic fixture passes; submit-time discrepancy, numeric errors, empty-value runtime failure reproduced |
| `git diff --check`; final source `git status --short` | Pass; clean source worktree |

No production query, live ES differential run, entire analytics package suite, Bazel run, or distributed cluster load test was performed. Actual ClickHouse was **26.8.2.7**, newer than the repository test-container version **23.8.16.40**. The tests using Memory tables validate SQL/name resolution and logical results, not replicated/distributed/MV behavior. Final observed GitHub checks: insights-server Bazel build/test and lint still running; Codeowner pending, CodeRabbit successful.

## Uncertain risks requiring production schema or data

- **Unbounded submit-time scans:** stats now scans metadata history without conversation-time bounds in three CTEs per overlap. Checked-in metadata MV is partitioned/ordered primarily by conversation start time, so template-only conditions may scan broad history; no eligible-conversation semijoin is pushed into these CTEs. Measure rows/bytes/read latency for production-sized tenants before claiming acceptable cost.
- **Raw/MV latest-value parity:** raw uses create_time; MV uses update_time and a ReplacingMergeTree key based on conversation/template/hour. Reindex order, equal timestamps and background merges may change which historical value is considered latest. Production frequency and severity were not measured.
- **Freshness/retention/deletion:** an absent MV row is interpreted as absent metadata. Lag, omitted backfill, partial retention, or stale deleted annotations can change membership; no live tenant parity/freshness check was performed.
- **Settings/version:** independent exclusions still compare join keys directly to `''`, which assumes `join_use_nulls=0`. Distributed join settings, older ClickHouse analyzer behavior and production schema migrations were not validated by local Memory tables.
- **Ordering:** conversation listing orders by conversation ID and scorecard update time, not a unique score/criterion key. Equal-key pagination instability predates this change and was not load-tested.

## Credentials and retained artifacts

Credentials used after individual inspection: (1) SSH `/Users/xuanyu.wang/.ssh/id_ed25519` for Git access, with directory/config/public-comment documentation inspected; fetch explicitly used `IdentitiesOnly=yes`, `IdentityAgent=none` and that key. (2) GitHub CLI account `xuanyuwang`, keychain service `gh:github.com`, after inspecting hosts/config and that keychain entry's metadata/comments; environment token overrides were unset for gh calls. (3) Repository embedded-PostgreSQL test account `cresta`, after reading its credential entry in `shared/db/testing/embeddedpgx.go`; `TEST_DATABASE_URL` was unset. Local `clickhouse local` is in-process and used no database credentials. The QA suites used mocked ClickHouse connections.

[Reproduction source, full SQL and outputs](../sessions/2026-09-08/convi-7402-review-evidence/) are retained with synthetic data only. Source tests were temporary and are not left in the PR worktree. No fixes or external review actions were performed.
