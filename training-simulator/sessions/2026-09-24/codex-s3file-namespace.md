# S3File namespace-only storage configuration

- Date: 2026-09-24
- Source repo: `/Users/xuanyu.wang/repos/cresta-proto`
- Worktree: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-s3file`
- Branch: `xw/convi-7707-s3-file-lifecycle`
- Work item: CONVI-7707; PR: https://github.com/cresta/cresta-proto/pull/9995

## Decision and scope

User now requests only namespace to be configurable. This supersedes the prior FE-selected bucket and prefixes decision. Backend selects the bucket from server configuration, owns disjoint staging/permanent roots and authorized customer/profile scope, and generates upload IDs/complete keys. FE supplies a required simple storage namespace plus existing parent and file metadata. The namespace organizes objects and is not an authorization policy or domain registry. Keep bucket identity as response metadata. Existing quiz and S3File APIs remain unchanged. Issued-upload binding from the earlier review remains a separate follow-up; no token/session mechanism is added in this scoped edit.

## Execution

- Refreshed PR head `d9bc9df1007896fb80a9b4b763bf29bd3ee4e759`; fast-forwarded the clean worktree, preserving the remote main merge. Base is `029507d21d734620c116370c3e96c00749bd28e0`.
- Replaced both `storage_config` requests with required `storage_namespace = 4`; reserved field 3 and its old name. Removed `FileStorageConfig`. Namespace permits 1–63 lowercase ASCII letters/digits/underscores/hyphens starting with a letter. No exact key layout is exposed as a client contract: clients treat generated keys as opaque. Updated bucket response documentation.
- Passed repository Buf lint, scoped compilation and breaking comparison against base `029507d21d`, scoped API lint, `mage checkAllProtoDependencies`, import/HTTP-path checks, Google clang-format and `git diff --check`. `bazel build //cresta/v1/s3file:all` passed; inspected generated PGV validators for both namespace fields. No new test scaffolding or generated clients committed.
- Initial validation harness attempts needed correction: uv used the configured private index and returned 401; reused a local formatter binary instead. Docker image uses buf as its entrypoint; overrode it for shell commands. Colima does not share macOS /tmp, so moved snapshots into the home cache. Added the initially omitted protoc options directory to the archived source snapshot. Full corrected checks passed.
- Published `7a8e98cda49c15f535bbc885629a73c617395f87` to existing #9995 with a normal fast-forward push. PR body update first stopped on a changed timestamp caused by the push; re-read and confirmed the body was unchanged and head matched, then applied the summary with fresh timestamp/body-hash guards. Verified exact description and byte-for-byte preservation of the automated codegen appendix/unchecked acknowledgement. New CI completion, merge or deployment is not claimed.
- Credential use: GitHub HTTPS authentication. No AWS/Okta/Azure/SSH credentials were selected or read; cached Docker checks used `--pull=never` with no registry access. The initial uv attempt contacted the configured CodeArtifact index and failed authentication; no credential retrieval followed.
- Updated work item, project README/project.yaml and daily log directly in knowledge; preserved unrelated uncommitted work. No knowledge commit or push.

- Final read-back confirmed remote head matches and `isDraft=false`; this turn did not change PR readiness. Source worktree is clean.

## Unpublished-field cleanup

User confirmed storage_config has not shipped. Removed the unnecessary number/name reservations in both new requests and assigned storage_namespace field 3. Published cf83ee11a2 to #9995; scoped Buf lint/breaking against main and the S3File Bazel build passed. Updated the PR description with timestamp/body guards and preserved the automated appendix. GitHub HTTPS authentication only; no cloud/SSH credentials used. This supersedes the reservation choice recorded above.
