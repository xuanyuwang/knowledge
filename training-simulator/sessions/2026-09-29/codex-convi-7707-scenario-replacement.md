# CONVI-7707 scenario backend replacement

- Date: 2026-09-29, America/Toronto
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree/branch: `/Users/xuanyu.wang/repos/go-servers-convi-7707-scenario-v2`, `xw/convi-7707-scenario-images-v2`
- Commits/PR: `19d812238b` (initial), `1922a92b4a` (rebased head); https://github.com/cresta/go-servers/pull/33045
- Ticket: https://linear.app/cresta/issue/CONVI-7707/be-of-add-image-to-scenario-context

## Replacement

Started on generic S3File backend branch #33043, applied the prior persistence work, and carried over signed scenario reads while omitting the obsolete Training Simulator upload/commit RPCs and legacy quiz adapters. The old three-part go-servers stack #32494/#32597/#32598 was closed after draft #33045 opened. Old branches remain intact.

Scenario create/update now accepts only well-formed permanent keys under the matching customer/profile and `scenario-images` namespace before VA/DB effects. Legacy quiz keys remain valid for legacy quiz APIs, but are not scenario keys. Nullable JSONB and `TrainingModuleImageListSQL` store stable metadata; the converter strips transient URLs without mutating requests. Image-only edits preserve the VA revision. List reads sign fresh image URLs and leave stored descriptors unchanged.

## Validation and limits

- Focused create, update, list, key-scope, signed-read and converter suites passed with a temporary `-modfile` replacing the proto module with locally generated #10057 source. This local override was not committed. Gazelle and `git diff --check` passed.
- Normal CI/Bazel compilation is blocked until #10057 merges, releases, and its dependency is bumped in `go.mod`, `go.sum`, and `deps.bzl`. Draft #33045 is stacked on #33043.
- Rebased #33045 onto #33043 after the provider-error review fix and updated its remote branch with an exact force-with-lease guard; no scenario source change was needed.
- Additive schema deployment must precede backend rollout. FE integration and upload → commit → save → trainee read staging validation remain pending. No live storage, DB, deployment, or staging changes were made.

Credential use: GitHub HTTPS authentication only. No AWS, Okta, Azure or SSH credentials were selected, read or used.
