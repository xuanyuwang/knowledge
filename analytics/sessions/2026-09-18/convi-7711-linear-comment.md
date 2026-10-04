Investigated Sunbit production PostgreSQL and ClickHouse on September 18. **The same conversation-ended-time failure mechanism as CONVI-7674 reproduces on these process scorecards.** This strongly supports the same cause, but we still need Sean's failing request payload or a fixed-build test to confirm attribution to his session.

The database control uses customer `sunbit`, profile `us-east-1`, Care use case, and September 1–14: `[2026-09-01 00:00:00Z, 2026-09-15 00:00:00Z)`. It does not apply Sean's exact group/ACL filters or reproduce the unknown browser timezone.

### 1. PostgreSQL: process scorecards exist

Database: `sunbit-us-east-1`, read-only connection.

```sql
SELECT
    t.title,
    s.template_id,
    s.template_revision,
    s.usecase_id,
    count(*) AS scorecards,
    count(*) FILTER (WHERE s.conversation_id = '') AS no_conversation,
    count(*) FILTER (WHERE s.submitted_at IS NOT NULL) AS submitted,
    count(DISTINCT s.agent_user_id) AS agents,
    min(s.process_interaction_at) AS min_time,
    max(s.process_interaction_at) AS max_time
FROM director.scorecards s
JOIN director.scorecard_templates t
  ON (s.customer, s.profile, s.template_id, s.template_revision)
   = (t.customer, t.profile, t.resource_id, t.revision)
WHERE s.customer = 'sunbit'
  AND s.profile = 'us-east-1'
  AND s.template_id IN (
    '019cffb8-041b-7748-837b-ebe698d36c08',
    '019d6997-161e-7636-b242-5ffc4f5d601a',
    '019a87fd-2f52-75ef-b4e5-e5d7f1e50111',
    '019e1909-b9f3-75b8-ab9e-407c1e0a0863'
  )
  AND s.process_interaction_at >= '2026-09-01'
  AND s.process_interaction_at < '2026-09-15'
GROUP BY 1, 2, 3, 4
ORDER BY 1, 3;
```

Results (timestamps UTC):

| Template | Template ID / revision | Use case | Scorecards | Empty conversation ID | Submitted | Agents | Min interaction | Max interaction |
| --- | --- | --- | ---: | ---: | ---: | ---: | --- | --- |
| Digital Support Assessment 2026 | `019d6997-161e-7636-b242-5ffc4f5d601a@db5be4bb` | care | 21 | 21 | 18 | 11 | Sep 4 07:00 | Sep 14 07:00 |
| Digital Support Template | `019cffb8-041b-7748-837b-ebe698d36c08@bccc9280` | care | 131 | 131 | 121 | 14 | Sep 1 07:00 | Sep 14 07:00 |
| Resolutions | `019a87fd-2f52-75ef-b4e5-e5d7f1e50111@a05d998e` | care | 99 | 99 | 96 | 12 | Sep 1 07:00 | Sep 14 07:00 |

Template inspection confirms type `2` (`SCORECARD_TEMPLATE_TYPE_PROCESS`) for these templates. Resolutions Assessment has no rows in this window. The additional assessment templates were checked because exact screenshot resource IDs were unavailable.

### 2. ClickHouse: the ended-time join changes 131/99 to 0/0

Database: `sunbit_us_east_1`, conversations service in `us-east-1-prod`.

The query below deduplicates parents by scorecard ID and latest `scorecard_last_update_time`, then compares the interaction-time population with the same population inner-joined to conversations ending in the selected window. `FINAL` alone is insufficient to remove all historical scorecard versions.

```sql
WITH sc AS (SELECT scorecard_id,scorecard_template_id,argMax(conversation_id,scorecard_last_update_time) AS cid
FROM scorecard_d
WHERE customer_id='sunbit' AND profile_id='us-east-1' AND usecase_id='care' AND scorecard_template_id IN ('019cffb8-041b-7748-837b-ebe698d36c08','019a87fd-2f52-75ef-b4e5-e5d7f1e50111') AND scorecard_time>='2026-09-01' AND scorecard_time<'2026-09-15'
GROUP BY 1,2), cv AS (SELECT DISTINCT conversation_id
FROM conversation_d
WHERE customer_id='sunbit' AND profile_id='us-east-1' AND usecase_id='care' AND conversation_end_time>='2026-09-01' AND conversation_end_time<'2026-09-15')
SELECT 'interaction time / no join' AS mode,countIf(scorecard_template_id='019cffb8-041b-7748-837b-ebe698d36c08') AS digital_support,countIf(scorecard_template_id='019a87fd-2f52-75ef-b4e5-e5d7f1e50111') AS resolutions
FROM sc
UNION ALL
SELECT 'conversation-ended / inner join',countIf(scorecard_template_id='019cffb8-041b-7748-837b-ebe698d36c08'),countIf(scorecard_template_id='019a87fd-2f52-75ef-b4e5-e5d7f1e50111')
FROM sc GLOBAL INNER JOIN cv ON sc.cid=cv.conversation_id FORMAT TSVWithNames;
```

Actual result:

```text
mode                              digital_support  resolutions
interaction time / no join        131              99
conversation-ended / inner join   0                0
```

Query ID: `6acde046-290d-4b46-acf6-a88bc524b8d0`; elapsed: 0.305 seconds.

A separate full comparison of the 230 main-template parents confirms that PG and CH have identical scorecard ID sets, agent IDs, interaction timestamps, and parent scores (all parent scores are 100). Sorted-ID MD5 controls also agree:

```text
Digital Support: 114df519b0d8e86e13006e3d56280639
Resolutions:     22ce338b1d82eb3c7a6c48355ac5746e
```

Every one of these parents has `conversation_id = ''`, so the conversation inner join eliminates all of them. This is the same mechanism identified in CONVI-7674. In the backend, `CONVERSATION_ENDED_AT` sets `needsConversationEndTime` (unless the time target is SUBMIT_TIME), and `qaScorecardQuery` adds `JOIN conversation ON scorecard.conversation_id = conversation.conversation_id`.

### 3. Scope of the conclusion and next validation

This confirms the mechanism on Sunbit data, **not yet the field actually sent by Sean's session**. The ticket's role/hierarchy description has not been independently verified as effective API authorization. The decisive next check is:

- Compare Sean's and Support's `RetrieveQAScoreStats` requests, especially `conversationTimeRangeField`, `timeRangeFilterTarget`, exact template IDs, dates, and user/group/status filters.
- If Sean sends `TARGET_FIELD_FOR_TIME_RANGE_CONVERSATION_ENDED_AT`, test the same state with [Director PR #22772](https://github.com/cresta/director/pull/22772)'s fix, which clears the incompatible field for process templates and also normalizes auxiliary count-chart requests.

PR #22772 merged September 15. As checked September 18, `release/director/2026-09-17` tip `817023bda48a506d16e4929fceb886ee02cc12c2` still lacks the fix by both ancestry and effective code inspection. Sunbit's actual deployed build was not independently identified.

**Separate data-consistency finding:** 13 parents (7 Digital Support, 6 Resolutions) are submitted in PG but retain older, unsubmitted latest versions in CH. PG/CH submitted counts are 121/114 and 96/90 respectively. Thus parent population/score parity does not imply submission-state parity. This deserves a separate synchronization follow-up and does not explain the complete loss of both template populations. No database repairs or reindexing were performed.
