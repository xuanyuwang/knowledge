# Remove bucket name from new S3File responses

- Date: 2026-09-24
- Source repo: `/Users/xuanyu.wang/repos/cresta-proto`
- Worktree: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-s3file`
- Branch: `xw/convi-7707-s3-file-lifecycle`
- PR: https://github.com/cresta/cresta-proto/pull/9995

## Decision

User requested removing `bucket_name` from the new response messages. Bucket selection belongs to BE/Config Service. Existing bucket-change concerns are explicitly out of scope. Keep the object's permanent key relative to the server-configured bucket. Keep existing legacy S3File contracts untouched.

## Work

- Confirmed local and remote PR head were `9ec3d180a186aed015049b85ba1dbc7f2fd5db94` and the source worktree was clean.
- Removed `UploadEntity.bucket_name = 9` and `StoredFile.bucket_name = 5`, updating the associated key comments. These fields were not released in production, so no reservations were added.
- Scoped Buf lint/build, breaking check against PR base main `029507d21d734620c116370c3e96c00749bd28e0`, API lint, clang-format and diff check passed. Bazel build passed (including regenerated S3File Go code), as did `git diff --check`.
- Published `76fcca55cb26f4b2d5bedd93627c567294ebaab5` to existing PR #9995 by fast-forward push. Guarded PR description update against latest timestamp and body hash, verified remote head and preserved the automated appendix and unchecked acknowledgement.
- No backend implementation, existing S3File RPC, or quiz RPC was changed. CI completion is pending. GitHub HTTPS authentication only; no cloud or SSH credentials.
- No AWS/Okta/Azure/SSH credentials selected or used; validation used a cached Docker image with `--pull=never` and no registry access.
