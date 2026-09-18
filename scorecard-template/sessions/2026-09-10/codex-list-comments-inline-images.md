# ListComments oversized-record investigation

**Date:** 2026-09-10
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** main checkout; read-only code and production-data investigation
**Ticket:** CONVI-7671, split from CONVI-7663

## Question

Identify the comments that made a fixed 250-record `ListComments` page exceed the 4 MiB receive limit and determine whether they contain media, ordinary text, or synthetic test content.

## Inputs reviewed

- Existing CONVI-7663 investigation and Linear ticket.
- `ListComments` implementation, `chat_comments` schema, protobuf conversion, and prior PR #30472.
- Latest `cresta-sandbox-2 / voice-sandbox-2` config metadata.
- Read-only `app.chat_comments` queries against the affected application database.

## Findings

- The profile has 809 comments. Median content size is 92 bytes; p95 is about 433 bytes.
- Exactly seven comments exceed 100 KB, and exactly those seven contain `data:image/png;base64,...` inside HTML.
- Their content sizes are 1,355,333; 1,325,075; 1,269,772; 1,197,795; 805,292; 802,995; and 154,216 bytes, totaling 6,910,478 bytes.
- Removing the embedded image payload leaves only 125-448 characters of normal feedback markup per record.
- All seven are `AI_AGENT_FEEDBACK`. They were created by three authors and describe specific demo-agent errors or mismatches; the screenshots support those reports.
- Metadata is only hundreds of bytes. The inline PNG data dominates each record.
- `convertCommentModelToPb` copies `comment.Content` unchanged to `Comment.content`, so every `ListComments` response carries the full base64 screenshot.

| Comment ID | Created (UTC) | Content bytes | Classification |
|---|---:|---:|---|
| `019f80d1-e02e-7844-a7f8-de28b4e43058` | 2026-07-20 18:37:45 | 1,355,333 | PNG screenshot plus feedback text |
| `019f80b0-7430-70ab-aedd-9869b9090c79` | 2026-07-20 18:01:15 | 1,325,075 | PNG screenshot plus feedback text |
| `019f8071-c7c2-781f-a257-ee12efda86a2` | 2026-07-20 16:52:48 | 1,269,772 | PNG screenshot plus feedback text |
| `019f80d5-07d3-7d16-a15a-2e0eeb336cc7` | 2026-07-20 18:41:12 | 1,197,795 | PNG screenshot plus feedback text |
| `019fa74e-0eb0-7d77-b370-aac2976569e5` | 2026-07-28 05:58:58 | 805,292 | PNG screenshot plus feedback text |
| `019f7fed-51e2-73df-9602-a61d57fe0df5` | 2026-07-20 14:28:07 | 802,995 | PNG screenshot plus feedback text |
| `019ef6ba-5c0f-7328-aaf1-0922ee7fb980` | 2026-06-23 23:04:28 | 154,216 | PNG screenshot plus feedback text |

## Conclusion

The abnormal records are genuine media-bearing comments, not large plain-text test payloads. The affected profile is a sandbox/demo environment, but the usage is a normal feedback workflow: users embedded screenshots in rich-text comments. This validates both an immediate adaptive/byte-aware pagination fix and a longer-term evaluation of storing media by reference.

## Export consumption trace

1. `ExportScorecards` gathers the conversations referenced by the requested scorecards.
2. For conversation templates, `getLinkedCommentsForExport` calls collaboration `ListComments` with the conversation IDs, a fixed page size of 250, and no comment-type filter.
3. The loop ignores every returned comment without a scorecard `criterion`.
4. For a criterion-linked comment, it converts HTML to plain text and stores `[author display name] - text` under conversation ID and criterion ID.
5. CSV construction appends those strings to the existing score comment in the criterion's comment column.

A follow-up read-only query confirmed that all seven oversized records have empty `criterion` and `scoringSubject` values. They cannot contribute to the CSV. Their full base64 images are transferred only because the dependency request is broader than the consumer's needs; the export fails before it can discard them.

This makes request narrowing the first incident-specific mitigation: request only `CONVERSATION` comments, after verifying that scorecard-criterion comments consistently use that type. Byte-aware protection remains useful for a legitimately large criterion-linked comment.

## External actions

- Created [CONVI-7671](https://linear.app/cresta/issue/CONVI-7671/bound-listcomments-responses-containing-inline-images) in the same team, cycle, state, assignee, and label as CONVI-7663.
- Related the tickets, narrowed CONVI-7663 to `ListCurrentScorecardTemplates`, and moved the comment-specific alert and PR attachment to CONVI-7671.

## Credential use

- Used the explicitly reviewed `voice-prod_ro` AWS SSO profile through the read-only customer app-database connection workflow.
- No credential material was printed, persisted, or included in this note.
