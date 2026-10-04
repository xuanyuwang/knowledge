# S3FileService per-file upload maximum

- Date: 2026-09-28, America/Toronto
- Source: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-s3file`, branch `xw/convi-7707-s3-file-lifecycle`
- PR: [cresta-proto#9995](https://github.com/cresta/cresta-proto/pull/9995)
- Commit: `610381e74fdf6b0d6d6ef0cf2ab96858a9acc78d`

Added `optional int64 max_upload_size_bytes = 3` to `GenerateFileUploadUrlsRequest.FileToUpload`, annotated optional and validated as positive when present. Omission requests no signed size limit. A supplied value is an FE request for signing, not a trusted backend domain size policy. The existing response already carries HTTP method, signed headers and form values; these are needed because the shared S3 client signs a positive size limit with POST/form fields and signs an unbounded upload with PUT. The backend handler is not implemented in this PR.

Validation: Google clang-format and `git diff --check`; scoped Buf lint/build/breaking against the PR base; scoped API lint; `bazel build //cresta/v1/s3file:all`. Generated PGV validation was inspected and checks `> 0` only when the optional field is present. The commit was pushed, the remote PR head verified, and the PR description updated while preserving the automated `go generate` warning and unchecked acknowledgement. No backend source, legacy S3File RPC or quiz API changed. GitHub HTTPS authentication was used; no AWS, Okta, Azure or SSH credential was selected or used.
