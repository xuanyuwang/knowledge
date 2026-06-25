# Vivint cx-voice — reindex failure: orphan criterion `48191ac7`

Date: 2026-06-19
Source: Slack #job-errors-prod thread (thread_ts 1781832469.714439)
DB: us-west-2-prod / vivint-cx-voice (director schema), read-only

## Symptom

`ConversationReindexer` partial `BatchIndexConversations` failure:

```
failed building score rows for scorecard 019ed710-d8fc-79d9-ba5f-05f0cb038ece:
code = InvalidArgument desc = criterion '48191ac7' for score 019edd65-... not found in template
```

A score row references criterion `48191ac7`, which does not exist in the template
revision the score's scorecard is pinned to. The reindex builder errors out instead
of skipping, failing the whole batch.

## Key IDs

- template_id: `8dc3bef8-b17b-4e8e-a4e6-c4e845ba10b9`
- pinned revision: `f7637a02` (created 2026-01-12, **the current/latest revision**)
- orphan criterion: `48191ac7`

## Criterion `48191ac7` lifetime (from scorecard_templates revisions)

- Added: revision `b3c4c8c2` @ 2025-03-14 17:16:35
- Present through: revision `5ed15f62` @ 2025-03-20 14:35:24
- Removed as of: revision `a7765142` @ 2025-11-10 22:24:57
- Pinned revision `f7637a02` (2026-01-12) does **not** contain it (`has_criterion = f`)

## Data findings (queried 2026-06-19)

| Metric | Value |
|---|---|
| Affected scorecards | 1,607 |
| Stray `48191ac7` score rows | 1,607 (exactly one per scorecard) |
| Other orphan criteria | none |
| Total scores on affected cards | 53,786 (~33 valid scores/card, 35 distinct criteria) |
| Scorecard created_at | 2026-06-17 (180), 2026-06-18 (1,427) |

### Why the "re-pin by created_at" idea was rejected

Original plan: check scorecard `created_at`; if created while `48191ac7` existed,
re-pin to the older revision that had it; else delete + backfill.

Data killed it: **all** affected cards were created 2026-06-17/18 — long after the
criterion was removed (2025-11-10) — and are pinned to the **current** revision with a
full set of ~33 otherwise-valid scores. Re-pinning to a 2025 revision would mismatch
the other 33 criteria (which only exist in `f7637a02`) and corrupt reporting. Deleting
the scorecards would discard legitimate fully-scored cards.

## Root cause (working theory)

The scoring / scorecard-creation path on Jun 17–18 produced a score for a criterion
removed 7 months earlier — i.e. a **stale template / criteria set** at scoring time,
while the scorecard row was (correctly) pinned to the current revision. This is more
than a reindex race; the bad data is written at creation time. Needs follow-up on what
changed around 2026-06-17.

## Recommended fix

1. **Data cleanup (immediate):** delete only the 1,607 stray `48191ac7` rows, keep the
   scorecards + valid scores, then reindex. The reindexer fails *during row-building*
   (before its own delete-before-write cleanup), so it cannot self-heal — the row must
   be removed first.

   ```sql
   -- WRITE connection; expect 1607 rows
   DELETE FROM director.scores s
   USING director.scorecards sc
   WHERE s.customer='vivint' AND s.profile='cx-voice'
     AND sc.customer=s.customer AND sc.profile=s.profile AND sc.resource_id=s.scorecard_id
     AND sc.template_id='8dc3bef8-b17b-4e8e-a4e6-c4e845ba10b9'
     AND sc.template_revision='f7637a02'
     AND s.criterion_identifier='48191ac7';
   ```

2. **Harden reindex builder:** skip scores whose criterion is absent from the pinned
   revision (treat as stale) rather than erroring the whole batch — one orphan row
   should not block 1,600+ conversations.

3. **Find root cause:** investigate the scoring/scorecard-creation path for a stale
   template cache that still served `48191ac7` on 2026-06-17/18.

## Open / not yet checked

- Whether other (customer, profile, template, revision, criterion) combos are similarly
  affected — only this one combo was characterized.
- Reference: go-servers `temporal/ingestion/reindexscorecards/README.md#cleanup-delete-before-write`
