# S3FileService upload/commit proto PR

- Primary source repo: `/Users/xuanyu.wang/repos/cresta-proto`.
- Worktree: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-s3file`.
- Branch: `xw/convi-7707-s3-file-lifecycle`, created from freshly fetched main.
- Ticket: CONVI-7707, verified live through Linear MCP.
- User authorized creation of a cresta-proto PR for a generic S3FileService storage lifecycle. Initial phase keeps domain configuration/restrictions in FE; domain-service integration and existing quiz changes are follow-ups.
- This direction supersedes the earlier proposed requirement for a server-side domain-policy layer in the initial implementation. Profile/bucket isolation and structural key validation remain storage responsibilities.
- Scope: additive batch temporary-upload preparation and permanent-file commit contracts, generic descriptors, profile-private storage with caller-selected namespace, and existing S3File upload role set. Existing quiz and S3File RPCs unchanged. Earlier draft #9918 and backend stack are not modified in this task.
- Precedents: current quiz upload/commit, S3File single-file upload, and document-ingestion upload method/header/form-field response.
- Read repo API standards, development guide, PR template, workspace operating model and existing CONVI-7707 work item. Generated client code must remain out of the PR.
- Authentication so far: existing GitHub HTTPS credentials and Linear connector. Git SSH disabled on fetch; no AWS, Okta, Azure or SSH credentials used.

## Contract and validation

- Base main: `a52824040e4e541cc61b33b4d92bc3d485bb31ca`.
- Added `GenerateFileUploadUrls`, `CommitFiles`, and a four-field `StoredFile` descriptor in the s3file package. Both new methods retain the existing S3File upload role set and 1–20-file batch bounds from quiz media.
- Caller selects `storage_namespace` (one lowercase path segment; `tmp` reserved) and per-file MIME. Bucket/profile come from the parent and server config. Namespace is organization, not permission. Permanent/staged keys retain `{namespace}/{profile}/{upload-id}/{filename}` and `tmp/` layouts, compatible with `quiz-media`.
- File-type/size domain policy remains in FE; no size-limit or backend policy-registry fields added. Upload responses carry method, headers and ordered form fields following the existing document-ingestion precedent, so clients do not hardcode PUT semantics.
- Commit describes metadata-only verification/reporting, sequential non-atomic processing, best-effort staging cleanup and non-idempotent retry behavior. Domain records, attachment association and download URLs are outside commit.
- Existing S3File and Training Simulator methods/messages remain unchanged. No generated client artifacts included; no changes to `public/proto`.
- Passed repository-wide Buf lint through the cached codegen image, scoped Buf compilation/API lint, scoped breaking check against an archived base descriptor, dependency checks, import/HTTP-path checks, clang-format and diff checks.
- Gazelle regenerated only the s3file BUILD source list. `bazel build //cresta/v1/s3file:all` passed, including generated Go, gRPC, gateway and validation code (116.799 seconds in the cold worktree).

## Tooling notes

- The first API-lint invocation mistakenly requested an uncached amd64 variant of the codegen image, which started an automatic ECR pull. Stopped that invocation and used the cached arm64 image explicitly with `--pull=never` for all subsequent Docker checks. No AWS profiles/keys were inspected or deliberately selected; the initial registry attempt used Docker's existing registry authentication.
- Git operations use existing GitHub HTTPS authentication with SSH disabled. Linear was read through its connector. No Okta, Azure or SSH credentials used.
- First API lint found a commit route suffix mismatch and a comment mistaken for a field requirement; corrected the new route to `s3file:commit` and clarified the header comment. Scoped API lint then passed.


## Published result

- Draft PR: https://github.com/cresta/cresta-proto/pull/9995
- Title: `CONVI-7707 Add generic S3File upload and commit contracts`.
- Head: `d35e318ed21b1982bbe9cafb464947d6dcd40bd6`; base: `main`.
- Verified remote OPEN/draft state, exact head and expected three-file diff (+261/-2). Source worktree is clean. CI started queued/pending; no CI completion, merge or release is claimed.
- Handwritten changes are limited to `s3_file_service.proto`, new `stored_file.proto`, and generated BUILD metadata. No changes to the earlier proto/backend drafts, domain implementation, public export, or generated client artifacts.
- Updated project README/project state, canonical work item, daily log and worktree registry. Knowledge changes remain uncommitted alongside existing work.
- Next: review contract, implement the S3File handlers and FE adoption, and reconcile earlier domain-media drafts before release.

## Caller-selected bucket and paths clarification

- User clarified that FE must specify where files are stored, including the S3 bucket, so the generic APIs do not rely on quiz storage infrastructure. The first draft did not meet this: it exposed a namespace but retained profile-private bucket resolution. This section supersedes that first-draft configuration above.
- Replaced unpublished `storage_namespace` with shared `FileStorageConfig` on both requests: required `bucket_name`, `temporary_key_prefix`, `permanent_key_prefix`. Staging and permanent objects use the same explicitly selected bucket. Prefixes may contain nested folders, must be valid relative paths and must differ. The service generates only a unique upload-ID/file-name suffix; it does not prepend profile/customer/quiz segments or infer a staging prefix.
- `parent` remains authorization/audit context. Storage credentials are server-side; callers must have access to their chosen bucket/prefixes. No domain-policy registry or customer infrastructure provisioning added.
- Added bucket identity to upload instructions (`bucket_name = 9`) and committed `StoredFile` (`bucket_name = 5`), so an object key can be persisted without inferring its bucket.
- New APIs have no image-only allowlist or quiz metadata dependency. Generic HEAD/copy/delete mechanics, ordered 1–20-file batches, unique suffix generation, metadata reporting and documented non-atomic retry behavior remain. No Go implementation was removed or migrated: this is still a proto-only PR.
- Validation passed: repository-wide Buf lint, scoped API lint, descriptor compilation and breaking check against base main, dependency/import/HTTP-path checks, clang-format, diff check and `bazel build //cresta/v1/s3file:all` (1.870 seconds). A final comment-only clarification allows nested staging/permanent prefixes while requiring distinct prefixes; no behavior-bearing changes followed validation.
- Published commit `325589945f95aa791eb7881830dfe00ff892cb68` to existing draft #9995. Updated only the three affected summary bullets with remote updated_at/body-hash guards; verified exact body round-trip and byte-for-byte preservation of the automated go-generate warning/unchecked acknowledgement. Existing quiz APIs and older drafts are unchanged.
- Credential use this follow-up: GitHub HTTPS authentication only. Docker checks explicitly used the cached image with `--pull=never`; no registry credentials or AWS/Okta/Azure/SSH credentials used this follow-up.

## Notification converter CI failure attribution

- Investigated the user-reported failing job: https://github.com/cresta/cresta-proto/actions/runs/35942126642/job/107457445781?pr=9995 . Read-only investigation; no source changes, CI reruns, PR comments/body edits or merge acknowledgements.
- Job tested PR head `325589945f95aa791eb7881830dfe00ff892cb68` through merge commit `27a6317d4f487e6f31cadcd20b1fa33fcb446bd7`, with exact proto base `a52824040e4e541cc61b33b4d92bc3d485bb31ca`. It checked out go-servers main at `e721aaec85da9b9b2bd47badc23bea24a3fe93ea`.
- Changed proto package was only `cresta/v1/s3file`; the workflow resolved its consumer to the root go-servers module `.` and ran `go generate ./...` with the entire proto checkout as a module replacement. That reaches notification converters even though notification protos were not changed by this PR.
- `chime_sound_enabled = 24` was introduced by CONVI-7717, merged proto PR #9956, commit `c3ecaa473b99015042f11cad490389b79d2860e1` (2026-09-22T23:11:51Z). Verified this commit is an ancestor of the exact PR base.
- Notification proto blob `e19a142a7c4d7dbddfc84b5bbe2b4f4bfc4aeb3f` and generated notification Go blob `c2849868e8ca65ab43cee3f563d62917b99704f2` are byte-identical on the exact base and PR head.
- At the exact CI go-servers commit, `notifications-server/internal/sql-schema/gen/notifications/model/global_preferences.go` lacks ChimeSoundEnabled. `notifications-server/internal/converter/converter.go:39` maps the DB model to the API message without supplying or ignoring that field. Both were verified via GitHub Contents API at the CI SHA, rather than relying on the older local checkout.
- The original #9956 CI already logged the identical `Cannot match the target field with the source entry: "ChimeSoundEnabled" does not exist.` error at 2026-09-22T23:18:25Z: https://github.com/cresta/cresta-proto/actions/runs/35794346762/job/106975829812 . This is historical execution evidence in addition to exact-base source equality; no new full base CI rerun was performed.
- Conclusion: this specific converter failure comes from the inherited notification proto/backend mismatch, not the S3File contract changes. It belongs to the CONVI-7717 notification persistence/converter follow-up. This finding does not declare the whole PR CI green or acknowledge merge readiness.
- Authentication used: existing GitHub HTTPS authentication only; no cloud or SSH credentials.
