# PR #9995 upload review comments

- Date: 2026-09-28, America/Toronto
- Primary source repo: `/Users/xuanyu.wang/repos/cresta-proto`
- Worktree/branch: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-s3file`, `xw/convi-7707-s3-file-lifecycle`
- Related implementation repo: `/Users/xuanyu.wang/repos/go-servers` local checkout (read-only)
- PR head/base checked: `48169fb3725165e58e02a3b9fe4ee42cef5f4c02` / `029507d21d734620c116370c3e96c00749bd28e0`
- Comments: https://github.com/cresta/cresta-proto/pull/9995#discussion_r4108312730 and https://github.com/cresta/cresta-proto/pull/9995#discussion_r4118843661

## Comment 4108312730: response fields, size limits, bucket

- The bucket portion is already resolved on current head: `bucket_name` was removed from both new upload and StoredFile responses in commit `76fcca55cb`. Older S3File bucket fields still exist, by design.
- Shared `objectstorage.Client.PresignPutObject` returns HTTP method, signed headers and form fields. In the S3 implementation (`shared/objectstorage/s3impl/client_impl.go:166-259`), `SizeLimitInBytes > 0` uses presigned multipart POST with a `content-length-range` policy; zero uses PUT and may require an exact Content-Type header. GCS retains PUT and adds a signed size-range header (`gcsimpl/client_impl.go:175-208`). Returning method/parameters is therefore a real provider contract, not merely speculative.
- Current new `GenerateFileUploadUrlsRequest.FileToUpload` has only filename and MIME, no size or maximum-size field (`s3_file_service.proto:153-203`). Initial plan leaves domain type/size policy to FE. Thus no caller-provided bound currently drives POST. BE could still apply a separately configured generic cap, but none is specified in this proto or implemented yet. An actual byte count is distinct from an enforced maximum. `http_method`, headers and form values would be PUT-focused unless a BE cap/policy is added. This is a valid design gap/clarification, not an observed runtime bug.
- Legacy `ScopedClient.PresignPutObject` returns only URL/key/URI and explicitly rejects `SizeLimitInBytes > 0` (`scoped_client_impl.go:169-184`) because it cannot represent POST/form/header instructions.

## Comment 4118843661: reuse existing temporary upload RPC

- Both methods prepare a temporary upload URL. Existing `GenerateTempUploadURI` accepts one parent+filename and returns presigned URI, temporary S3 file path/URI and expiry (`s3_file_service.proto:367-397`; `action_generate_temp_upload_uri.go:37-77`). It uses a profile-private scoped client with configured fallback bucket, a `tmp/` staging prefix and generated `uploads/{user}/{timestamp}-{uuid}-{filename}` suffix. It derives MIME from extension and, by default, permits CSV/JSON/PDF/Parquet/ZIP but not images (`action_generate_temp_upload_uri.go:21-35,90-107`). It signs PUT with no size limit.
- New `GenerateFileUploadUrls` accepts 1-20 filename/MIME pairs plus `storage_namespace`, returns full upload instructions and a permanent key to echo at commit (`s3_file_service.proto:153-265`). BE would own namespace-scoped staging/permanent keys. No new generic handlers exist in the inspected local go-servers checkout.
- Reuse of the existing RPC is possible in principle, but not drop-in. To support image uploads and the current new contract, one would need to change extension/MIME policy, add namespace or establish an equivalent commit-mapping rule, accept the existing temporary path at commit or change the returned key, handle batching through repeated calls or extend the RPC, and expose method/form/header data if provider-enforced size caps are wanted. Existing Director and Speechlab clients consume the legacy response, so compatibility matters. Existing `GenerateTempUploadURI` can otherwise remain for its current users while new RPCs share lower-level storage helpers.
- Recommended current direction: keep the dedicated new upload RPC for the namespace-scoped batch + commit flow, reuse the underlying `Client.PresignPutObject` and storage operations in BE. Preserve existing API behavior. Clarify that initial implementation returns PUT unless BE chooses a trusted size cap; decide separately whether size enforcement must move from FE to BE before modifying proto. This is a design judgment, not an implementation decision.

## Scope and credentials

Read-only review. No source edits, tests, PR replies/body updates or pushes. GitHub HTTPS authentication used for comment/PR reads; no AWS/Okta/Azure/SSH credentials selected or used.
