# CONVI-7671 ListComments HTML-to-text projection

**Date:** 2026-09-22
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers-convi-7671`
**Related proto worktree:** `/Users/xuanyu.wang/repos/cresta-proto-convi-7671-content-format`
**Branches:** `xw/convi-7671-list-comments-size`, `xw/convi-7671-comment-content-format`

## Decision

Following the offline agreement with Tinglin, scorecard export should request text from `ListComments` rather than request HTML with images removed and convert it again in the caller.

No new protobuf contract is needed. The existing `strip_inline_images` field is reused: false preserves stored HTML, while true asks go-servers to run HTML-to-text conversion before response serialization. The handler includes a comment documenting that scorecard export uses the flag for this purpose.

## HTML-to-text usage review

- Scorecard export previously parsed HTML and called `html2text.FromHTMLNode`; this conversion moves into `ListComments`.
- `action_list_coaching_plans.go` performs a simple `html2text.FromString` conversion but is unrelated to this request.
- `shared/coaching/session.go` enables `PrettyTables`, so it retains its local options.
- conversation conversion sanitizes and caps nesting before conversion.
- language detection and email paths strip domain-specific content and apply fallbacks before conversion.

The new call remains inline because it is a thin wrapper with no additional policy. Specialized pipelines remain local.

## Implementation

- Added a direct `html2text.FromString` call in `ListComments`.
- `ListComments` preserves HTML by default and converts each returned comment when `strip_inline_images` is true.
- Scorecard export enables the existing flag and directly formats returned content into CSV cells.
- Retained nested conversation IDs and the `Comment_CONVERSATION` filter.
- Closed cresta-proto PR #9969 because the explicit content-format field was unnecessary.
- Rebased go-servers onto current `origin/main` (`21557688b8`) and force-pushed PR #32304 at `24cf976403`; main's published proto dependency already contains `strip_inline_images`, so validation requires no local override.

## Validation

- Go: Gazelle updated affected BUILD files.
- Focused Bazel targets passed using the published proto dependency: shared HTML helper, ListComments, and ExportScorecards.
- The response-level test stores a 5 MiB base64 image and verifies the text response reaches the default-size client without the data URI.

## Credential use

- Used the GitHub CLI keyring token over HTTPS to close proto PR #9969 and update the go-servers branch.
- Did not use SSH credentials.
