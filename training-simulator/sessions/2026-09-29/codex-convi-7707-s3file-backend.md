# CONVI-7707 generic S3File backend

- Date: 2026-09-29, America/Toronto
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree/branch: `/Users/xuanyu.wang/repos/go-servers-convi-7707-s3file-backend`, `xw/convi-7707-s3file-backend`
- Commits/PR: `414c716a69`, `d0191114b2`; https://github.com/cresta/go-servers/pull/33043
- Ticket: https://linear.app/cresta/issue/CONVI-7707/be-of-add-image-to-scenario-context

## Inputs and decision

Merged cresta-proto#9995 is present in released `v2.30.26`; current go-servers main used `v2.30.23`. The earlier Training Simulator-specific upload/commit draft duplicated the generic contract. Implement generic upload/commit in S3FileService, using the profile-private bucket from server configuration and a generated customer/profile/namespace/upload-ID/file-name key. Staging keys add a disjoint `tmp/` root. Clients keep returned keys opaque. Legacy S3File and quiz RPCs retain their existing routes and key layout.

## Execution and validation

- Bumped cresta-proto in `go.mod`, `go.sum`, and `deps.bzl`; no local replacement committed.
- Added upload preparation through the cloud-agnostic raw storage client so signed PUT headers and size-limited POST form values are returned unchanged. The caller's positive maximum is passed to the provider; it is not backend domain policy.
- Added request-wide profile/namespace/duplicate validation before commit side effects. Sequential commit returns one status per file: committed metadata, first file failure with `google.rpc.Status`, then not-attempted remainder. Copy success with staging cleanup failure remains committed. No issued-upload binding or idempotent retry was added.
- Full `go test ./apiserver/internal/s3file -count=1` passed. `bazel build //apiserver/internal/s3file:s3file`, Gazelle, and `git diff --check` passed. Tests cover POST instructions, maximum forwarding, cross-profile rejection, duplicate-key rejection, ordered partial results, metadata and cleanup failure.
- Published the branch and opened #33043. Initial CI showed lint/build jobs passing while some checks remained in progress; staging was not exercised.
- Validated CodeRabbit finding that raw provider errors could reach API callers via `Internal` statuses or per-file error results. Logged those errors server-side and returned generic messages at `d0191114b2`. Focused file-lifecycle suite passed, including tests that provider details do not appear in upload or commit responses. Rebased dependent #33045 after this change.

## Follow-up

Review/merge #33043. Scenario persistence and signed reads are stacked separately in #33045; schema-before-code and staging end-to-end validation remain.

Credential use: GitHub HTTPS authentication only. No AWS, Okta, Azure or SSH credentials were selected, read or used.
