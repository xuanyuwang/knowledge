# S3FileService PR #9995 review validation

- Primary source repo: `/Users/xuanyu.wang/repos/cresta-proto`.
- Local worktree/branch: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-s3file`, `xw/convi-7707-s3-file-lifecycle`, local HEAD `325589945f`.
- Live PR head: `d9bc9df1007896fb80a9b4b763bf29bd3ee4e759`; base `029507d21d734620c116370c3e96c00749bd28e0`. The service proto blob is identical to the inspected local version (`adf26660b75d2ad17189351daed7c2cea6e12d64`). No checkout or remote branch mutation performed.
- Request: validate comments on https://github.com/cresta/cresta-proto/pull/9995#pullrequestreview-5298905066 . Review-only; no source fixes, comments, pushes or acknowledgements authorized/performed.

## Finding 1: caller-selected storage authorization

Comment: https://github.com/cresta/cresta-proto/pull/9995#discussion_r4089117939

Verdict: valid authorization requirement/contract gap; the claimed deployed cross-customer access is not established. There are no new S3File handlers on inspected go-servers main. The proto already says the caller must have access but does not specify the server-enforced relation between parent and the requested bucket/prefixes. RPC roles are not a per-location authorization check, and successful cloud IAM access authenticates the backend's credentials, not the frontend caller's right to choose that location.

Evidence: `shared/objectstorage/scoped_client_factory_impl.go:82` simply wraps a provided bucket with the shared storage client; it does not accept or validate a parent. Verified that source matches current remote main by blob hash. S3 presign/copy/delete primitives validate bucket syntax and call the SDK, not caller-to-profile access. Existing generic download has a SUPER_ADMIN role annotation and its direct provided-bucket branch is disabled in `action_generate_presigned_download_uri.go:81`.

Retain the user's explicit bucket/prefix fields. Require server checks that the caller/parent may upload at the staging location and copy/delete/promote at the relevant locations, rejecting violations with PERMISSION_DENIED. A generic parent-scoped allowlist is one option; a registered domain-purpose API is not intrinsically required. FE can still choose application type/size policy and destinations inside its authorized scope. These choices were not changed or implemented during review.

## Finding 2: commit is not bound to issued uploads

Comment: https://github.com/cresta/cresta-proto/pull/9995#discussion_r4089117944

Verdict: valid contract gap. `FileToCommit` has only key/display_name, and the request independently supplies the bucket and both prefixes. Shape checks do not prove the upload/key/configuration was issued by GenerateFileUploadUrls. The existing quiz flow also lacks upload sessions, but fixes staging under a server-chosen tmp/quiz-media prefix; arbitrary caller staging prefixes remove that particular boundary.

A local, synthetic path check used a valid UUID (rather than the review's abc123): existing source `content/training/11111111-1111-4111-8111-111111111111/image.png`, temporary prefix `content/training`, permanent prefix `other-content`, requested permanent key `other-content/11111111-1111-4111-8111-111111111111/image.png`. All documented prefix/UUID/filename shape constraints pass, while the derived staged source points at the existing content object. Copy followed by staging cleanup would relocate/delete that source under a straightforward implementation. No upload was issued. This was a pure string/path simulation, not an actual storage operation or live exploit demonstration. Backend object permissions or a trusted staging-only scope could reject it; neither is presently specified as the binding mechanism.

Recommend an authenticated opaque upload token returned during generation and required at commit, binding the authorized parent (and caller if per-user ownership is intended), bucket, full source/destination keys and expiry. It must include/protect the server-generated unique ID. A signed token can avoid upload-session DB persistence. Issuance must first authorize the location; signing alone does not solve finding 1. One-time/replay and idempotency guarantees remain separate decisions; do not claim a token automatically provides them.

## Validation details

- The prefix/filename subfinding is correct about generated validation: FileStorageConfig checks only non-empty fields; filename checks length 1–255. Neither checks separators, dot segments, controls or prefix equality. These checks may live in handlers; PGV need not encode every invariant. Current comments already describe the intended constraints, so this is an implementation obligation, not proof of a deployed bypass.
- The embedded Buf reports of missing google.api.field_behavior/resource_reference are not supported by actual project checks. Imports are present; current PR's real Lint API, Lint protobuf, Run bazel gen and bazel_test jobs passed (run 36004074762). The matching service proto previously passed local generation/build. No source fix is warranted for those diagnostics.
- Original quiz commit and the provided-bucket factory match current remote go-servers main by hash. New S3File upload/commit handlers are absent in that main directory.
- No cloud operations or credential inspection. Used existing GitHub HTTPS authentication only.

## Recommendation

Accept the two underlying storage-safety requirements while preserving FE-specified bucket/prefix configuration. Scope the work to generic destination authorization, issued-upload binding, and server-side structural validation; do not automatically introduce a domain-policy registry, fixed quiz storage, or upload-session database. This is a recommendation, not an approved implementation change.
