# CONVI-7671 implementation

**Date:** 2026-09-15
**Source repos:** `/Users/xuanyu.wang/repos/cresta-proto-convi-7671`, `/Users/xuanyu.wang/repos/go-servers-convi-7671`
**Branch:** `xw/convi-7671-list-comments-size` in both repositories
**Ticket:** CONVI-7671

## Implemented contract

- Added backward-compatible `ListCommentsRequest.strip_inline_images`. Its default is false, preserving full rich content for existing callers.
- When enabled, `ListComments` removes HTML `<img>` elements after authorization and model conversion but before constructing the serialized response.
- The tokenizer-based transform preserves all non-image HTML bytes and strips both normal and self-closing image tags.

## Scorecard export narrowing

- Moved selected conversation IDs from deprecated top-level `conversation_ids` to `filter.conversation_ids`.
- Added `filter.types = [CONVERSATION]`, excluding AI feedback that cannot be created as criterion-linked feedback through the supported UI.
- Enabled `strip_inline_images` because export ultimately converts criterion comment HTML to plain text and does not consume images.
- Constructs the request inline at the single production call site; a test-only helper keeps the same expected request readable across 22 export test cases.

## Validation

- Generated temporary local Go bindings from the proto worktree and used a temporary go.mod replacement solely for cross-repository testing; neither generated code nor the local replacement remains in the intended diffs.
- `buf lint --path cresta/v1/collaboration` passed with only the repository's existing deprecated-category warning.
- `buf build --path cresta/v1/collaboration` passed.
- Full `TestListComments` suite passed.
- Full `TestExportScorecards` suite and the focused request-construction test passed.
- The response-level integration test inserts a comment containing a 5 MiB base64 image and successfully receives the stripped response, exercising removal before the default gRPC client receive boundary.
- `mage Lint` was unavailable because the `mage` binary is not installed in this environment.

## Dependency sequencing

[cresta-proto PR #9876](https://github.com/cresta/cresta-proto/pull/9876) merged as `b1c442393b`, and published version `v2.24.3` contains `ListCommentsRequest.strip_inline_images`. The go-servers branch now consumes that version in `go.mod`, `go.sum`, and `deps.bzl`; no absolute local `replace` or generated proto code is present. After rebasing onto current `main` and inlining the single-use production request helper, [go-servers PR #32304](https://github.com/cresta/go-servers/pull/32304) is at `f32e79333a`.

Standalone validation against v2.24.3:

- `go mod tidy`, `bazel run //:gazelle-update-repos`, and `bazel run //:gazelle` completed successfully.
- The published binding contains both the `StripInlineImages` field and `GetStripInlineImages()` accessor.
- Full `//apiserver/internal/collaboration:collaboration_test` passed.
- Focused `TestExportScorecards|TestNewScorecardExportListCommentsRequest` tests in the coaching target passed.
- An unfiltered coaching-target run built successfully, then failed only in unrelated ClickHouse-container suites because the local Colima Docker socket was unavailable.

## Residual risk

Image stripping protects the scorecard-export caller from a single image-bearing comment and request filtering removes the observed profile-wide over-fetch. It does not bound arbitrarily large plain text. Byte-aware response handling and explicit single-record-over-budget behavior remain defense-in-depth follow-up work.

## Generated-schema review verification

A review suggestion asked for manually regenerating `gen/cresta-proto-schemas/schemas/cresta.v1.collaboration/ListCommentsRequest.json` in PR #9876. The suggestion was skipped after current-code and current-CI verification:

- `.github/workflows/lint_and_buf_gen.yaml` runs on relevant pull requests and `main` pushes.
- Pull-request jobs successfully run both buf and Bazel generation, but `Commit & Release` is skipped because its condition permits commits only on `refs/heads/main` or an explicitly committing manual dispatch.
- After a human protobuf commit lands on `main`, the workflow replaces generated outputs and creates an `Auto generate code from protobuf for <sha>` commit containing `gen/` and `groundcover_fields.json`.
- Commit `cd97260ea3` is direct historical evidence for this exact schema path: it updated `ListCommentsRequest.json` in an automatic generated commit.

Therefore the feature branch remains proto-definition-only. The generated schema will gain `stripInlineImages` through the normal post-merge generation/release path.
