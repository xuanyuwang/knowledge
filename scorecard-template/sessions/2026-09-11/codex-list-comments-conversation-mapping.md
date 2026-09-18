# CONVI-7671 image-comment conversation mapping

**Date:** 2026-09-11
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** main checkout; read-only investigation
**Ticket:** CONVI-7671

## Question

Identify the exact conversations containing the seven oversized inline-image comments.

## Evidence and method

- Rechecked the `voice-prod_ro` AWS profile's own configuration entry before use; it is not marked restricted.
- Used the read-only customer app-database workflow against `cresta-sandbox-2 / voice-sandbox-2`.
- The live query could not refresh because the AWS SSO session had expired. No credential material was printed or persisted.
- Recovered the exact mapping from the preserved output of the successful 2026-09-10 read-only query against `app.chat_comments`.

## Exact mapping

| Conversation ID | Comment ID | Content bytes |
|---|---|---:|
| `f417690c-09d0-404b-b8b7-9933c966633b` | `019f80d1-e02e-7844-a7f8-de28b4e43058` | 1,355,333 |
| `108503c2-979e-41a5-b5db-9bda1766a96e` | `019f80b0-7430-70ab-aedd-9869b9090c79` | 1,325,075 |
| `8a4493a6-73cf-4b39-ae68-6bb637faa395` | `019f8071-c7c2-781f-a257-ee12efda86a2` | 1,269,772 |
| `02a8faef-bcad-411a-b1e0-e8a51b05ad17` | `019f80d5-07d3-7d16-a15a-2e0eeb336cc7` | 1,197,795 |
| `2ce8d6aa-c585-4f84-bae3-9f2360211d57` | `019fa74e-0eb0-7d77-b370-aac2976569e5` | 805,292 |
| `c494bd8e-f886-4476-97a2-965e4640647a` | `019f7fed-51e2-73df-9602-a61d57fe0df5` | 802,995 |
| `60c683eb-c9e9-4d1a-be4d-276a3495f0f9` | `019ef6ba-5c0f-7328-aaf1-0922ee7fb980` | 154,216 |

All seven records are `AI_AGENT_FEEDBACK` with an inline PNG data URI, empty `criterion`, and empty `scoringSubject`.

## Root-cause implication

The seven records belong to seven distinct conversations, which initially appears inconsistent with the failing request containing one scorecard name. Current code resolves the inconsistency:

- `getLinkedCommentsForExport` places selected IDs in deprecated top-level `ListCommentsRequest.conversation_ids`.
- `ListComments` builds conditions and checks access using only `req.filter.conversation_ids`.
- With the wildcard parent and no nested filter IDs, the query is scoped only by customer and profile, so it can return comments from unrelated conversations.
- The existing test for top-level IDs with a nil filter asserts only that at least one comment is returned. It does not insert or assert exclusion of an unrelated conversation, so the regression is not detected.

This request-field migration bug is the immediate cause that allows one-scorecard export to collect profile-wide comments. The missing comment-type filter and lack of byte-aware pagination are additional narrowing and resilience gaps.

## Link format

Director's closed-conversation route encodes the full resource name under `/conversations/closed/:conversationName`. The derived resource is:

`customers/cresta-sandbox-2/profiles/voice-sandbox-2/conversations/<conversation-id>`

An attempted browser validation redirected to the Director login page, so the route shape is code-confirmed but was not authenticated in this session.

## Follow-up

- Fix export to populate `filter.conversation_ids` and add a negative cross-conversation regression test.
- Add a `CONVERSATION` type filter after verifying criterion-linked comment types.
- Refresh `voice-prod_ro` SSO if a current database recheck or additional conversation metadata is needed.

## Comment-size guardrail follow-up

- `Comment.content` has only a protobuf `min_len = 1` rule; there is no maximum-length or maximum-byte rule.
- Backend `validateComment` checks required and output-only fields but does not inspect content size. Both `CreateComment` and `UpdateComment` assign the full content string to the database model.
- `app.chat_comments.content` is PostgreSQL `TEXT` with no application-level length constraint.
- Director's shared TipTap editor configures its image extension with `allowBase64: true`. The ordinary `CreateCommentPopup` uses this editor, so a large pasted image is possible for `CONVERSATION` comments as well as the observed AI feedback.
- Therefore, filtering scorecard export to `CONVERSATION` removes the seven known outliers but is not a complete serialized-size bound. An export-specific image-exclusion option plus byte-aware response handling is still warranted. Pagination must define behavior for one comment larger than the budget; a future create/update limit would prevent new records but not address existing data.

## External action

- With explicit user approval, replaced the CONVI-7671 Linear description with the detailed UI-to-export-to-`ListComments` flow, code locations, prioritized mitigations, and acceptance criteria.
- Added the user-verified affected-conversation link using the correct `/director/conversations/closed/...` route.
- Reloaded the ticket and confirmed that the formatted sections and conversation hyperlink persisted.
