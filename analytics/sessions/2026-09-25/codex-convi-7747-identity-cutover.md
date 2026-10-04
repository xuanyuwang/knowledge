# CONVI-7747 Comcast Active Days identity-cutover evidence

- Date: 2026-09-25 (America/Toronto)
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Source context: clean main checkout at `772c0f2590b45803acd07092a9460dc15cdb61f9`
- Config: `/Users/xuanyu.wang/repos/config`, fetched `origin/master` `aa9f2cafce47ebfbf6cbf54a109b8bfdb556dbf5`
- Profile/use case: `comcast/prod/acquisition` (voice), cluster `comcast-prod`
- Read-only stores: app PostgreSQL `comcast`; conversations ClickHouse `comcast_prod`; events ClickHouse `auth_comcast`
- Ticket: https://linear.app/cresta/issue/CONVI-7747/comcast-residential-agent-leaderboard-active-days-shows-0n-a-for-all
- Independent merge record: https://linear.app/cresta/issue/CE-7255/comcast-resi-username-migration-reindex-recovery-for-timed-out-merge

## Conclusion

The sampled August 19 false-negative Active Days are caused by a **user-identity cutover**, not the Bill West online-session candidate-window bug. The UPN-to-NTID merge moved app conversations and messages from email-style user IDs to short-username IDs. Existing `conversation_with_labels_d` rows and historical Agent Assist online events remained under the email-style IDs. The Agent Stats query groups its conversation and active-label populations by `agent_user_id`; current short-username IDs therefore show conversations but no active labels on August 19. On August 20, the online events and label rows use the short IDs, reproducing the reported switch to 1.

CE-7255 documents merge job `01a01de1-3fc8-70eb-9f54-70a5d3dc456d`: 7,918 users and 965,386 conversations moved during the August 19-20 cutover. The merge timed out before indexing; subsequent conversation and scorecard reindex jobs repaired those projections. The label table is written by `cron/sync-users/internal/conversation-agent-assistance/task.go`; the reindex stages did not re-label it. This explains how the mismatch survived the prior recovery. The exact mapping and affected label-row count still need a cohort-wide audit.

## Exact ticket examples

The ticket's apparent conversation IDs are Comcast platform IDs. PostgreSQL `app.chats.platform_id` resolves them to UUID conversation IDs, all `profile_id=prod`, `usecase_id=acquisition`, and `source=0`:

| Agent | Platform ID suffix | Conversation UUID | Current PG agent ID | UTC start | Agent messages |
| --- | --- | --- | --- | --- | ---: |
| Steven Ebert | `1786405014266646` | `019fee09-4d29-764f-af4d-28813bccc5ae` | `44ba6115dde5053c` | Aug 10 23:36:54 | 65 |
| Steven Ebert | `1786406322942959` | `019fee1d-44b3-7843-a5f1-5514a2d7aa7e` | `44ba6115dde5053c` | Aug 10 23:58:43 | 89 |
| Steven Ebert | `1786408479617162` | `019fee3e-2d6a-77bf-b7b5-deb53108b964` | `44ba6115dde5053c` | Aug 11 00:34:40 | 41 |
| Nick DiVeglia | `1787145698863529` | `01a01a2f-4110-764e-bb51-9b308935d1a7` | `a6ff8ad1d26904d` | Aug 19 13:21:39 | 70 |
| Matthew Nadolski | `1787172936281664` | `01a01bce-dbe4-7dc5-b521-0c14c98cebf9` | `304d475cdbadccda` | Aug 19 20:55:36 | 38 |

For each exact UUID, ClickHouse `conversation_with_labels_d FINAL` already has a `has_agent_assistance=true` message row, written on August 11 or 19 **before the overlap fix**. Its `agent_user_id` and `message_agent_user_id` are the old email-style user ID, while the corresponding current PostgreSQL message and conversation carry the short user ID:

| Agent | Old label/event ID | Current conversation/message ID | Old user name | Current user name |
| --- | --- | --- | --- | --- |
| Steven Ebert | `a7803d2d623a4396` | `44ba6115dde5053c` | `sebert200@cable.comcast.com` | `sebert200` |
| Nick DiVeglia | `96c06d69575fd643` | `a6ff8ad1d26904d` | `ndiveg379@cable.comcast.com` | `ndiveg379` |
| Matthew Nadolski | `f2b430ea795af19c` | `304d475cdbadccda` | `mnadol200@cable.comcast.com` | `mnadol200` |

The exact message IDs in the label rows still exist in PG, now assigned to the current short ID. `conversation_d FINAL` has both old and new `agent_user_id` rows for each sampled conversation: the old version was written at conversation end, and the new version was written August 21. The table's replacement key contains the agent, so `FINAL` does not collapse these identities.

## August 19 to 20 reproduction

Using America/New_York daily windows, source-0 Acquisition conversations, `conversation_d FINAL`, and true message-level labels in `conversation_with_labels_d FINAL`:

| Local date | Current short ID | Conversation count | AA-labeled count on short ID | AA-labeled count on old ID |
| --- | --- | ---: | ---: | ---: |
| Aug 19 | Steven `44ba...` | 20 | 0 | 18 |
| Aug 19 | Nick `a6ff...` | 13 | 0 | 11 |
| Aug 19 | Matthew `304d...` | 17 | 0 | 16 |
| Aug 20 | Steven `44ba...` | 15 | 15 | 0 |
| Aug 20 | Nick `a6ff...` | 20 | 19 | 0 |
| Aug 20 | Matthew `304d...` | 10 | 9 | 0 |

The old-ID August 19 label counts are fewer than conversations because not every conversation has a qualifying message label. The decisive point is zero old-to-new identity match before the cutover, then positive matches on the short IDs on August 20. This reproduces the ticket's zero-to-one daily Active Days pattern for three sampled agents across the two named teams.

Events ClickHouse `auth_comcast.user_event_d` independently confirms qualifying application-type-0 online rows on August 19 under the old IDs (Nick 36 rows, Steven 36, Matthew 63) and none under the short IDs. On August 20, the short IDs have online rows (Nick 38, Steven 31, Matthew 41). An exact Nick old-ID session spans `2026-08-19 13:27:07.726Z`–`13:43:26.679Z` and covers his labeled `13:29:28.407Z` message. Matthew's old-ID session spans `20:58:06.078Z`–`21:13:22.362Z` and covers his `21:02:25.665Z` message. Qualifying sessions were present and the labeler found them; the Bill West candidate-window failure is not this sampled cause.

The source-8 ingestion exclusion does not explain these samples: all five exact PG conversations are source 0. The checked-in Comcast insights config does not set `excludeIngestionPipelineFromAgentStats`, though live effective flag state was not read.

## Repair implications

A normal re-run of `cron-label-conversations` against current PG conversations/messages would use short IDs, while pre-August-20 online events remain under old IDs. It would not reconstruct those true historical matches. A repair needs the approved old-to-new merge mapping and must migrate or recompute the label rows with that identity mapping, then clean old rows so old/new keys do not coexist. Bound the job by the merge cohort and affected date/ID set, verify the distinct conversation/message keys, and re-run the same Agent Stats daily comparison afterward. No repair job was submitted and no data was mutated.

## Credential handling

Used the individually reviewed ordinary `comcast-prod_ro` and `comcast-prod_dev` AWS SSO profile entries via the read-only DB skills. The ClickHouse skill uses `comcast-prod_dev` for its configured secret; only read-only SELECT queries were sent. The ordinary GitHub SSH key was used for config fetch. The continuation also attempted the individually reviewed `us-east-2-prod_ro` profile for auth DB access, but validation failed before connecting. No other AWS, Okta, Azure, or SSH credential was used.

## Expanded audit and backfill preparation

Source review confirms `RetrieveAgentStats` computes `COUNT(DISTINCT agent_user_id)` separately from `conversation_d` and `conversation_with_labels_d`, then full-joins on grouping keys. Its response uses the active-label count when the AA-count flag is enabled and conversations exist. `temporal/ingestion/mergeusers/workflow.go` runs ES and ClickHouse conversation/scorecard reindex stages, with no label replay stage. ClickHouse schema `conversation_with_labels` uses `(toStartOfHour(conversation_end_time), agent_user_id, conversation_id, customer_id, profile_id, message_id)` as its replacement key, so `FINAL` preserves old and new identities.

A read-only CH join of true Acquisition message labels with latest conversation agent IDs by exact conversation UUID produced: Aug 18 (1,668 matched conversations, 1,668 mismatches, 413 distinct old IDs), Aug 19 (5,988/5,988, 474), Aug 20 (4,407/12, 9), Aug 21 (4,862/0), Aug 22 (2,652/0), using America/New_York message days. For Aug 18–19 together, 560 observed old IDs each matched exactly one latest conversation ID; 51 had just one observed conversation. [Exact read-only SQL](convi-7747-label-identity-audit.sql) is retained for reproduction. The latest CH identity is not a substitute for the complete merge input or full PG validation. The earlier same-day conversation-start-bound query returned 5,984 Aug 19 matches; allowing cross-midnight conversations returned 5,988.

The auth PostgreSQL schema has `auth.users.migrated_to_user_id`, an authoritative old-to-new source. The read-only auth DB skill resolved `auth-comcast-prod` but `cresta-cli` rejected cluster validation (no clusters returned), so no auth query ran. The backfill [job specification](../../deliverables/convi-7747-identity-backfill-job.md) leaves mapping extraction and full historical inventory as submission gates. A dedicated skill was drafted at `/Users/xuanyu.wang/repos/coaching-qm-skills/diagnose-active-days/SKILL.md`. No mutation ran. This continuation used individually reviewed ordinary `comcast-prod_ro`, `comcast-prod_dev`, and `us-east-2-prod_ro` AWS entries and the ordinary GitHub SSH key for config fetch; the auth credential attempt did not reach the database.

## Impact and customer-feedback assessment

On September 25, a full pre-August-20 read-only Acquisition label audit used the same local-day basis as Agent Stats: `message_time` for message rows, `conversation_start_time` for legacy blank-message rows, with an America/New_York cutoff at `2026-08-20 04:00 UTC`. All 221,041 label rows joined by exact conversation UUID to the latest `conversation_d FINAL` row (latest `agent_user_id` by `update_time`, source-0, non-dev). [Reproduction query](convi-7747-impact-audit.sql).

| Scope | Stale label rows | Conversations with stale labels | Current agent IDs | Agent-days with stale true labels |
| --- | ---: | ---: | ---: | ---: |
| All pre-Aug-20 Acquisition | 220,476 / 221,041 | 220,472 / 221,037 labeled conversations | 1,006 | 16,171 |
| Aug 8–19, the reported window | not separately counted | 50,177 true-labeled conversations | 846 | 4,343 |

Of the full stale rows, 648 are false labels and 16,427 are legacy blank-message rows. Restricting to true labels leaves 219,824 mismatched conversations out of 220,384 true-labeled conversations. The full mismatch spans 33,337 true-labeled conversations in June, 106,693 in July, 29,617 on Aug 1–7, and 50,177 on Aug 8–19. Monthly/distinct-agent counts are 360, 597, 509, and 846 respectively; agents overlap periods.

At the daily display grain, no affected current agent-day had another true Acquisition label under its current ID. Of 16,171 stale-label agent-days, 16,062 across 1,004 agents also had a source-0, non-dev Acquisition conversation starting on that same New York day, making them strong candidates for an incorrectly displayed 0. For Aug 8–19, this is 4,325 agent-days across 846 agents. The other 109 full-range agent-days could be N/A or affected by conversation-time/other request semantics; they should not be called proven wrong zeros without exact API replay. The live effective AA-count flag and team membership/visibility for all 1,006 agents were not independently checked. These are data-level impact estimates, not a count of people who viewed the metric.

A post-cutover read-only comparison of true Acquisition labels to latest conversation IDs found 12 mismatched conversations in the Aug 20–22 slice and zero across each subsequent week from Aug 23 through Sep 25 (36,695, 44,469, 37,043, 39,193, and 32,286 matched true-labeled conversations by week). This supports a historical reporting repair decision rather than an ongoing mismatch, within the exact joined population checked.

The candidate mapping inferred from exact old-label/current-conversation pairs has 1,006 old IDs and 1,006 current IDs, no old ID with multiple observed targets, and 141 old IDs supported by only one observed conversation. A self-join found zero existing target label keys for the 220,476 stale rows at the same `(conversation_id, message_id, target_agent_user_id)`. This simplifies the mechanical repair, but the original CE-7255 mapping or auth migration state must still validate all pairs. CE-7255 reports 965,386 conversations moved across the broader merge; its 90-day parameter selected users, while their entire histories moved. That broader count is not the Active Days impact count.

[CONVI-7747](https://linear.app/cresta/issue/CONVI-7747/comcast-residential-agent-leaderboard-active-days-shows-0n-a-for-all) says Comcast saw the same pattern across multiple supervisor teams, reports onboarding between mid-June and mid-July, and describes reporting/coaching visibility impact with no live-call impact. It links support ticket 23359; its own comments contain no subsequent customer answer about historical correction priority or desired date scope. Zendesk was not accessible, so no direct support-thread feedback was reviewed. The prudent next step is to ask the customer-facing owner whether Comcast needs all history since June corrected, only Aug 8–19, or an explanation without backfill. No job was implemented or run during this assessment.
