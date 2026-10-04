# S3File issued-upload binding

- Source repo: `/Users/xuanyu.wang/repos/cresta-proto`
- Worktree: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-s3file`
- Branch: `xw/convi-7707-s3-file-lifecycle`
- Work item: CONVI-7707; PR: https://github.com/cresta/cresta-proto/pull/9995

## Finding validation

Treated supplied finding as untrusted review input and checked current local/remote head cf83ee11a2. The finding is valid: upload response has no token and FileToCommit only has a key and display name. Namespace-only backend-owned storage removes arbitrary destination inputs but does not prove the staged upload was issued. No generic handlers were found in the local go-servers checkout. This is a protobuf contract fix, not a runtime enforcement claim.

## Minimal contract change

- Add required nonempty UploadEntity.upload_token = 10 and FileToCommit.upload_token = 3.
- Token is opaque to FE and signed by BE, binding parent, namespace, bucket, staging/permanent prefixes, exact source/destination keys, and expiry.
- Reuse expiration_time as both upload URL/token deadline; commit must validate before it expires. No new expiry field or session database requirement.
- Authorize caller and validate signature, expiry and all bindings against authorized storage configuration before copying/deleting each file. Reject mismatches without mutations to that file; never silently substitute storage configuration or infer alternate paths from a supplied key.
- Existing sequential partial-failure behavior remains. Token does not promise single-use or idempotency. Domain policy and existing quiz RPCs remain unchanged.

Token addition passed scoped Buf lint/build/breaking against PR base, API lint, generated-code Bazel build and diff checks. Generated validators rejected empty tokens. Published 8b11b1bd28 and updated the PR description before the user interrupted to discuss severity.

## Deferral and revert

The user explicitly deferred token binding and requested removal of 8b11b1bd28. Reverted its changes in 9ec3d180a186aed015049b85ba1dbc7f2fd5db94 using a new commit (no rewritten history). The resulting S3File source is byte-identical to cf83ee11a2; Bazel build and diff checks passed. Pushed to #9995 and updated the PR summary to record deferral, preserving its automated appendix with fresh timestamp/body-hash guards. Caller authorization, fixed separate roots, tenant scope and exact-key checks remain backend requirements. No token implementation or production behavior changed. GitHub HTTPS authentication only; no cloud/SSH credentials. CI completion not checked.
