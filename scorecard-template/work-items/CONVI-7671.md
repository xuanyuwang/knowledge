# CONVI-7671: Bound ListComments responses containing inline images

**Status:** active
**Primary domain:** `scorecard-template`
**Primary subdomain:** none
**Official ticket:** [CONVI-7671](https://linear.app/cresta/issue/CONVI-7671/bound-listcomments-responses-containing-inline-images)
**Last updated:** 2026-09-16

## Objective and Impact

- **Objective:** Prevent scorecard export from failing when `ListComments` pages contain large media-bearing comments.
- **Customer/system impact:** A fixed 250-record page serialized to 6,303,757 bytes and exceeded the apiserver client's 4 MiB receive limit.
- **Role:** diagnosed and split from CONVI-7663

## Scope

**In scope**

- Adaptive retry of the same page token with smaller page sizes after `RESOURCE_EXHAUSTED`.
- A deterministic serialized-byte bound for `ListComments` responses.
- Clear handling when one comment alone exceeds the limit.
- Behavioral tests using image-sized comment content.
- Evaluation of moving uploaded media out of inline base64 HTML content.

**Non-goals**

- Relying solely on a larger gRPC receive limit.
- Treating fixed record count as a serialized-byte guarantee.
- The `ListCurrentScorecardTemplates` incident, which remains CONVI-7663.

## Source Context

- **Repos:** `/Users/xuanyu.wang/repos/go-servers`, `/Users/xuanyu.wang/repos/cresta-proto`
- **Worktrees:** `/Users/xuanyu.wang/repos/go-servers-convi-7671`, `/Users/xuanyu.wang/repos/cresta-proto-convi-7671`
- **Branches:** `xw/convi-7671-list-comments-size` in both repositories
- **PRs/commits:** merged [cresta-proto PR #9876](https://github.com/cresta/cresta-proto/pull/9876); dependent [go-servers PR #32304](https://github.com/cresta/go-servers/pull/32304) at `a274bc548a`; go-servers PR #30472 / `a81bcb24ad` is the prior fixed-page-size mitigation

## Current Understanding

The outliers are legitimate screenshot-bearing `AI_AGENT_FEEDBACK`, not synthetic large text. In the affected `cresta-sandbox-2 / voice-sandbox-2` app database, seven of 809 comments contain PNG screenshots embedded as `data:image/png;base64,...` within HTML. They total 6,910,478 content bytes; six are about 0.80-1.36 MB and one is about 154 KB. The profile-wide median is 92 bytes and p95 is about 433 bytes.

`ExportScorecards` intends to ask `ListComments` for comments belonging to the selected scorecard conversations, but it sends the IDs through the deprecated top-level `conversation_ids` field. The current `ListComments` handler reads only `filter.conversation_ids` when building the query and checking access. With a wildcard parent, the intended conversation restriction is therefore ignored and the query can return comments across the entire profile. The exporter also omits a type filter. It later retains only comments with a non-empty scorecard `criterion`, converts their HTML to plain text, formats them as `[author] - text`, and appends them to the relevant criterion-comment CSV cell. All seven oversized screenshot comments are `AI_AGENT_FEEDBACK` with an empty criterion, so the failing export receives their full image data and then discards them. The observed incident is therefore a request-field migration bug plus unnecessary type over-fetch, with byte-unbounded paging as the residual robustness gap.

## Findings and Decisions

- The seven large comments were created by three authors between 2026-06-23 and 2026-07-28.
- Their surrounding text describes concrete demo-agent behavior and the images are supporting screenshots.
- Comment metadata is small (roughly 230-308 bytes for the largest records); `content` is the dominant field.
- All seven oversized records have empty `criterion` and `scoringSubject` fields and cannot contribute to the scorecard CSV.
- The seven image comments belong to seven distinct conversations: `f417690c-09d0-404b-b8b7-9933c966633b`, `108503c2-979e-41a5-b5db-9bda1766a96e`, `8a4493a6-73cf-4b39-ae68-6bb637faa395`, `02a8faef-bcad-411a-b1e0-e8a51b05ad17`, `2ce8d6aa-c585-4f84-bae3-9f2360211d57`, `c494bd8e-f886-4476-97a2-965e4640647a`, and `60c683eb-c9e9-4d1a-be4d-276a3495f0f9`.
- `getLinkedCommentsForExport` populates deprecated `ListCommentsRequest.conversation_ids`, but `ListComments` passes only `req.filter.conversation_ids` into query construction and access checks. The top-level IDs are ignored.
- The existing nil-filter/top-level-conversation-IDs test only asserts that some comments are returned; its fixture does not prove that comments from other conversations are excluded, so it misses this regression.
- The export's actual customer-facing output is plain-text, criterion-linked comments in scorecard CSV columns; inline image bytes are not delivered through that output.
- There is no comment-specific content-size check on create or update. The proto validates only `min_len = 1`, `validateComment` does not inspect content length, and both handlers write the full string to a PostgreSQL `TEXT` column.
- Director's shared TipTap editor explicitly enables base64 images (`allowBase64: true`), including the ordinary conversation-comment popup. A single `CONVERSATION` comment can therefore contain a large inline image, so filtering out `AI_AGENT_FEEDBACK` does not by itself bound a `ListComments` page.
- The immediate resilience fix and the longer-term inline-media storage contract should be separated if the latter requires migration.

## Validation and Rollout

- Read-only production data inspection completed.
- Linear description updated with the exact Director entry points, frontend and backend call locations, customer-facing export flow, prioritized mitigations, and an affected conversation deep link.
- Implementation is prepared in fresh `cresta-proto` and `go-servers` worktrees. The proto adds backward-compatible `ListCommentsRequest.strip_inline_images`; the server removes HTML `<img>` elements only when requested; scorecard export opts in and sends selected IDs plus `CONVERSATION` under the nested filter.
- The full `TestListComments` and `TestExportScorecards` suites pass against locally generated proto bindings. The image-projection integration test stores a 5 MiB base64 image, proving it is removed before the gRPC response reaches the default-size client.
- Proto `buf lint` and `buf build` pass. Repository `mage Lint` could not run because `mage` is unavailable in the local environment.
- Verified and skipped a review suggestion to commit `ListCommentsRequest.json` manually. Proto CI runs generation on pull requests but gates `Commit & Release` to `main` (or an explicitly committing manual dispatch); after merge, the generated commit updates tracked `gen/` artifacts. Existing history confirms this exact schema is maintained by `Auto generate code from protobuf for ...` commits.
- Merged cresta-proto PR [#9876](https://github.com/cresta/cresta-proto/pull/9876). Dependent go-servers PR [#32304](https://github.com/cresta/go-servers/pull/32304) is rebased on current `origin/main`, inherits its newer `github.com/cresta/cresta-proto/v2 v2.24.7`, contains no local replacement, and is mergeable.
- Standalone v2.24.7 validation passes for the full collaboration target and focused scorecard-export tests. The generated binding was checked directly for both `StripInlineImages` and `GetStripInlineImages()`.

## Next Actions

1. Let go-servers PR #32304 CI complete; investigate only failures attributable to this change.
2. Mark the dependent PR ready for review after required checks and review feedback are resolved.
3. Retain adaptive or deterministic byte-aware protection for genuinely large text comments. Define explicit behavior for a single comment above the response budget because pagination alone cannot solve it.
4. Decide whether create/update should reject oversized content and whether media references require a separate migration ticket; creation limits do not repair existing oversized rows.

## Timeline

- 2026-09-10 — Created CONVI-7671, linked it to CONVI-7663, and moved the comment-specific alert and PR reference to the new ticket.
- 2026-09-10 — Confirmed the abnormal comments are base64-embedded PNG screenshots attached to real demo-agent feedback.
- 2026-09-10 — Traced the customer-facing consumer: the export only emits criterion-linked comments as plain text in scorecard CSV cells; all seven oversized records are unrelated AI feedback that would be discarded.
- 2026-09-11 — Recovered the exact comment-to-conversation mapping from the preserved output of the successful read-only query. A live refresh was blocked by an expired `voice-prod_ro` SSO session.
- 2026-09-11 — Identified the immediate root cause: export sends deprecated top-level conversation IDs while `ListComments` reads only the nested filter, turning the wildcard request into a profile-wide comment query.
- 2026-09-11 — Replaced the CONVI-7671 Linear description with the approved detailed call-flow and mitigation scope and added a verified `/director/conversations/closed/...` example link.
- 2026-09-11 — Confirmed there is no per-comment size validation on create or update, while Director enables base64 image content in the shared editor used by ordinary conversation comments. Type narrowing removes the observed AI-feedback outliers but does not eliminate the single-comment overflow case.
- 2026-09-15 — Implemented the backward-compatible image-stripping projection and scorecard-export request narrowing in fresh proto and go-servers worktrees; validated the full affected suites against locally generated bindings, including a 5 MiB image response case.
- 2026-09-15 — Opened cresta-proto PR #9876 for the `strip_inline_images` contract; retained full response content as the backward-compatible default.
- 2026-09-15 — Rebased the go-servers implementation onto current `main`, pushed commit `40ec1dec37`, and opened mergeable draft PR #32304 with the proto release dependency and expected interim CI failure documented.
- 2026-09-15 — Rejected a stale-schema review finding after verifying the release workflow: PR generation checks pass without committing artifacts, while the post-merge `main` run creates the tracked schema/generated-code commit.
- 2026-09-15 — Updated go-servers PR #32304 to published cresta-proto v2.24.3, regenerated Go/Bazel dependency metadata, rebased onto current `main`, and revalidated the affected targets without a local proto replacement.
- 2026-09-15 — Removed the single-use production request-construction helper after review, inlined the `ListCommentsRequest`, and retained only a test helper shared by 22 mock expectations; focused export tests passed.
- 2026-09-16 — Rebased PR #32304 onto current `origin/main`; resolved the proto dependency conflict by dropping the obsolete v2.24.3 bump and retaining main's v2.24.7, which includes the required field. Focused tests passed and the mergeable branch was force-pushed with a lease.
