# CONVI-7711 Sunbit Performance Insights no data

- Date: 2026-09-18
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Branch/worktree: existing `main`, read-only, HEAD `b2bc3849854d124ed659058f8a8de6813262ddc6`.
- Related source: Director remote main `ec8886f59746a1fd1deedd0216df0d6af6e70472`; live release tree retrieved through GitHub API.
- Domain/subdomain: analytics/performance-insights.
- Request: investigate Sunbit CONVI-7711, compare with RCG CONVI-7674, verify PostgreSQL and ClickHouse first.
- Ticket: https://linear.app/cresta/issue/CONVI-7711/sunbit-performance-insights-returns-no-data-for-one-user-despite#comment-306c5ae9

## Conclusion and confidence

The same conversation-ended-time failure mechanism is reproduced on Sunbit production data. Both affected templates are process type; all 230 relevant scorecards have empty conversation IDs. The ended-time inner join reduces their 131/99 baseline counts to zero/zero. This strongly supports CONVI-7674 as the cause, but Sean's actual failing request is not captured, so user-session root-cause attribution and effective authorization remain unverified.

There is also a distinct submission projection discrepancy affecting 13 rows. All parent scorecard IDs, agents, interaction timestamps, and parent scores agree, but those 13 latest ClickHouse versions predate PostgreSQL submission. Do not summarize this as complete PG/CH parity or as missing scorecards.

## Ticket evidence and limits

The dedicated Linear MCP was not exposed in the available tool catalog. Browser access was stopped at the user's request. Glean MCP retrieved the indexed Linear ticket (updated 2026-09-18 14:16:04Z); its comments array is empty. A Glean Slack notification identifies the linked comment as Jan CC'ing Eleanore, Xuanyu, and Xingwei. No failing HAR/request or authorized impersonation was obtained.

Ticket identifies Sean Hudson, Care use case, Digital Support / Customer Resolutions group filters, Digital Support / Resolutions templates, September 1–14, Daily, scorecard-based calculation. Support sees populated data while Sean sees zero. Ticket assertions about Manager 2nd, Performance Admin, QM Specialist and hierarchy are reported evidence, not independently validated effective ACLs. App DB identifies Sean as user `63f1a4d1504030f`, active, non-dev.

Attachment URLs require authentication (HTTP 401); screenshot contents were not directly inspected. Exact screenshot template resource IDs and timezone therefore remain unconfirmed. All matching Digital Support / Resolutions template families were enumerated to avoid relying solely on titles.

## Connections and credential usage

- Config refreshed through the skill launcher; origin/master `56c7571b7cc5df795d99eb0ef72dcb81a1dd6eb0` at initial placement check; path `configv3/prod/sunbit/us-east-1/config.yaml`.
- PostgreSQL account and cluster: `us-east-1-prod`; database: `sunbit-us-east-1`; skill obtains IAM read-only access through cresta-cli. Queries only; statement timeout 30 seconds.
- ClickHouse host: `clickhouse-conversations.us-east-1-prod.internal.cresta.ai`; database `sunbit_us_east_1`; SELECT/metadata queries only.
- Cleared AWS SSO entries: `us-east-1-prod_dev` / `us-east-1-prod_ro` were read before use; the ClickHouse launcher explicitly uses `us-east-1-prod_dev`; cresta-cli's read-only connection workflow uses the account profiles. No restricted credential used.
- Git config fetch used the cleared `~/.ssh/id_ed25519`, explicitly selected with IdentitiesOnly and isolated SSH configuration. GitHub read-only API used existing gh authentication.
- No credential values were printed or persisted. No database mutations, product edits, reindexing, external messages, or ticket changes.
- Config fetch initially encountered a case-insensitive remote-ref collision; the launcher's retry succeeded. No Git configuration migration performed.

## PostgreSQL / ClickHouse comparison

Window: `[2026-09-01 00:00:00Z, 2026-09-15 00:00:00Z)`, using PostgreSQL `process_interaction_at` and CH `scorecard_time`, customer `sunbit`, profile `us-east-1`, care. Observed interaction timestamps are at 07:00Z. This is a bounded database control, not an exact replay of the unknown browser timezone/group/audience payload.

| Template | ID / revision | PG parents | CH distinct latest parents | Agents | Empty conversation IDs | PG submitted | CH submitted |
| --- | --- | ---: | ---: | ---: | ---: | ---: | ---: |
| Digital Support Template | `019cffb8-041b-7748-837b-ebe698d36c08@bccc9280` | 131 | 131 | 14 | 131 | 121 | 114 |
| Resolutions | `019a87fd-2f52-75ef-b4e5-e5d7f1e50111@a05d998e` | 99 | 99 | 12 | 99 | 96 | 90 |
| Digital Support Assessment 2026 (additional control) | `019d6997-161e-7636-b242-5ffc4f5d601a@db5be4bb` | 21 | 21 | 11 | 21 | 18 | 18 |

All these templates have type 2, confirmed as PROCESS in `cresta-proto/cresta/v1/coaching/scorecard_template.proto:147`. Resolutions Assessment had no rows in this window.

Sorted scorecard-ID MD5 controls agree in PG and CH:

- Digital Support: `114df519b0d8e86e13006e3d56280639`.
- Resolutions: `22ce338b1d82eb3c7a6c48355ac5746e`.
- Digital Support Assessment: `80e0e139d09bd13673da491fd945dccd`.

Full row comparison of the main two templates (230 rows) confirms identical ID sets, agent IDs, interaction times, and parent scores. All parent scores are 100. The assessment control has average 99.4047619 with min 87.5.

Important query correction: `scorecard_d FINAL` alone initially returned 132/101 rows because historical scorecard versions remain. The analytics path groups by scorecard ID and max(scorecard_last_update_time). Deduplicating that way gives 131/99 and exact ID parity. Do not report the initial raw FINAL counts as unique scorecards.

Criterion data uses `score_d`, not the old `scorecard_score_d`. Matching criteria to each latest parent `(scorecard_id, scorecard_last_update_time)` gives 124 Digital Support and 93 Resolutions scorecards with positive-weight, finite, non-N/A criteria. Digital Support's 124 agrees with a reported Support count; Resolutions' current 93 is not claimed equal to the screenshot's 92 because exact audience/window/capture time is unavailable. The preliminary old-table query returned no rows and was corrected after reading getTableName/source usage; it is not evidence of missing analytics data.

## Ended-time reproduction

Query: [convi-7711-ended-time-reproduction.sql](convi-7711-ended-time-reproduction.sql).

ClickHouse query ID: `6acde046-290d-4b46-acf6-a88bc524b8d0`, elapsed 0.305 seconds.

| Predicate mechanism | Digital Support | Resolutions |
| --- | ---: | ---: |
| Interaction-time window, no conversation join | 131 | 99 |
| Same rows inner-joined to care conversations ending in the window | 0 | 0 |

The inner join reproduces the source-level failure. Since all parent conversation IDs are empty, the entire scorable subset is eliminated as well. This test does not exercise Sean's actual ACL or establish that his request includes ENDED_AT.

Relevant source:

- `go-servers/insights-server/internal/analyticsimpl/retrieve_qa_score_stats_clickhouse.go:63-66`: conversation conditions/needsConversationEndTime add the inner join.
- Same file `:544-545`: ENDED_AT enables the join unless time range target is SUBMIT_TIME.
- Director `useQAScoreStatsRequestParams.ts:42` forwards persisted dateRangeTarget as conversationTimeRangeField.
- Director `usePerformanceFilters.tsx:112-113` initializes ENDED_AT; localStateToFilterState at `:183` uses saved state. PR #22772 clears the target for process templates and fixes reconstructed count-chart filters.

## Fix and release check

GitHub live read: PR https://github.com/cresta/director/pull/22772 is merged 2026-09-15 18:59:21Z, merge `f740a0fa253a6c304516502ac933387f09a1eb2c`.

The current `release/director/2026-09-17` tip is `817023bda48a506d16e4929fceb886ee02cc12c2` (newer than the prior knowledge snapshot). GitHub compare relative to the fix merge is diverged, ahead 3 / behind 36. More importantly, direct contents inspection of this exact release tree confirms the process-template reset at lines 194–209 still lacks dateRangeTarget clearing; count-chart reconstructed states also retain the pre-fix path. This release branch lacks the fix. Actual Sunbit deployed build was not independently identified.

## Separate submission-version discrepancy

13 rows (7 Digital Support, 6 Resolutions) are submitted in PG but retain epoch submit time in CH. Their CH last-update times are roughly 0.29–0.55 seconds before PG's final update; both old September 2 and recent September 14–17 submissions are affected. This suggests lost/stale post-submit projection, but the writer/stream root cause was not investigated. It cannot alone explain template-wide zero, because substantial scorable data remains. No reindex was run.

| Scorecard ID | Template | PG submission UTC | CH latest update UTC |
| --- | --- | --- | --- |
| `01a062ba-1bfc-720d-bc17-6a84a16d7604` | Digital Support | 2026-09-02 15:26:00.033439+00 | 2026-09-02 15:25:59.714635 |
| `01a062c1-ce3d-76cc-ab75-e5af56a255d7` | Digital Support | 2026-09-02 15:34:24.495169+00 | 2026-09-02 15:34:24.205699 |
| `01a062fc-f1eb-70c5-8ea9-4307d9f64da2` | Digital Support | 2026-09-02 16:39:00.299716+00 | 2026-09-02 16:38:59.932055 |
| `01a06300-ed09-72da-946d-6776b4f076d1` | Digital Support | 2026-09-02 16:43:21.060726+00 | 2026-09-02 16:43:20.726694 |
| `01a0630c-96b8-7753-9847-4dfa347bd042` | Digital Support | 2026-09-02 16:56:05.411759+00 | 2026-09-02 16:56:05.061319 |
| `01a063c5-0126-751b-940a-37121f6d7be1` | Resolutions | 2026-09-02 20:17:29.843462+00 | 2026-09-02 20:17:29.507102 |
| `01a063d0-1206-7148-b94d-f94a3c0f5903` | Resolutions | 2026-09-02 20:29:35.040577+00 | 2026-09-02 20:29:34.714433 |
| `01a063f0-dabc-763a-ab81-68317a0ce967` | Resolutions | 2026-09-02 21:05:23.605144+00 | 2026-09-02 21:05:23.237137 |
| `01a0a0f2-215b-7097-832f-c9ece7c4ec32` | Digital Support | 2026-09-14 17:23:38.901508+00 | 2026-09-14 17:23:38.356555 |
| `01a0a117-be32-72d9-bf05-306f20b02e05` | Digital Support | 2026-09-14 18:04:43.683048+00 | 2026-09-14 18:04:43.363730 |
| `01a0a620-f619-7796-8c16-7b2cc33cabe5` | Resolutions | 2026-09-15 17:32:53.592038+00 | 2026-09-15 17:32:53.282567 |
| `01a0abff-0549-70e0-b910-ad713e45e9f0` | Resolutions | 2026-09-16 20:53:32.485006+00 | 2026-09-16 20:53:32.164842 |
| `01a0b03e-be55-72a0-9a43-b19dd8bd6fef` | Resolutions | 2026-09-17 16:41:37.433804+00 | 2026-09-17 16:41:37.039282 |

## Next actions

1. Capture Sean's failing RetrieveQAScoreStats payload and compare conversationTimeRangeField, template ID, time range target, users/groups/status with Support's payload. No HAR currently available.
2. Alternatively validate Sean with PR #22772 override or a confirmed deployed build containing the fix, holding audience and dates constant; check both main and auxiliary requests.
3. Investigate effective ACL only if the request omits the incompatible target or the fixed build still fails. The ticket's permission claims are not proof of the effective API scope.
4. Follow up separately on the 13 stale submission projections; retain exact IDs above for scoped investigation.

Raw bounded PG/CH comparison exports remain temporary under `/tmp/convi-7711`; durable evidence is summarized here without credentials.

## Authorized ticket-comment follow-up

The user explicitly requested posting an evidence comment. Prepared [the complete comment](convi-7711-linear-comment.md), including exact PostgreSQL and ClickHouse SQL, actual results, query ID, confidence limits, release check, and the separate submission projection discrepancy. Posting is pending: the configured Linear MCP's tools are not exposed in the current tool catalog; Glean cannot create Linear comments. Browser fallback was not used, respecting the user's request to use Linear MCP. No comment has been posted.

### Comment posted and verified

After Linear authentication was restored, posted the prepared evidence through Linear MCP at 2026-09-18 15:59:11Z. Comment ID `4203ea56-084c-418e-a228-0675f2d0dd69`; [posted comment](https://linear.app/cresta/issue/CONVI-7711/sunbit-performance-insights-returns-no-data-for-one-user-despite#comment-4203ea56). Re-read comments and verified the entire body equals the prepared file. Live issue status was already Done; Xuanyu had separately commented that the fix is in director-beta and scheduled for next week. No issue state or description was changed. This supersedes the earlier posting blocker.
