# CONVI-7707: existing file RPC reuse search

- Date: 2026-09-22
- Domain: training-simulator / training-content
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Task branch/worktree: `xw/convi-7707-scenario-image-reads`, `/Users/xuanyu.wang/repos/go-servers-convi-7707-reads`, HEAD `98c5fa63d2`.
- Cross-check: local go-servers checkout at `6e9999f0ad` (2026-09-21); relevant S3File, document upload, and widget create implementation matches the task worktree.
- Contract source: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707`, HEAD `153723fe64`. These are local source findings, not claims about current deployed services or remote main freshness.
- User request: find existing RPCs that perform similar upload/commit/download jobs before designing another generalized API.

## Search and findings

Searched RPC definitions across `cresta/v1`, `cresta/product`, and `cresta/nonpublic`, then read the matching backend implementations. No general staged-file commit RPC found outside quiz/training in this search.

### 1. S3FileService: closest existing generic service

- Proto: `cresta/v1/s3file/s3_file_service.proto`, `GenerateTempUploadURI` line 20; `GeneratePresignedDownloadURI` line 47.
- Backend: `apiserver/internal/s3file/action_generate_temp_upload_uri.go`, `action_generate_presigned_download_uri.go`.
- Upload is a single-file presigned temporary PUT; returns URL, storage path/URI and expiry. Default extension allowlist: CSV, JSON, PDF, Parquet, ZIP. Images and Word are not on that default list. An environment flag can disable the restriction.
- Storage is selected through the existing storage factory, using private storage with a configured fallback bucket. Keys are uploader-based `uploads/{user}/...`, with temporary storage handling. This differs from the training profile/quiz-media layout.
- Download supports profile-private/shared and integration bucket sources; explicit Cresta bucket option is disabled in the implementation. Public RPC role annotation is SUPER_ADMIN only, and there is no owning-document permission check.
- No permanent commit RPC, durable attachment descriptor, batch upload, or size-limit/method/form-field response in this contract.
- Good candidate to extend additively as the existing domain-neutral API home, but not a drop-in replacement. Preserve existing consumers and current training bucket/paths. Do not broaden its raw-key admin download endpoint into an attachment-access endpoint.

### 2. DocumentReleaseIngestionService: strongest document-upload precedent

- Proto: `cresta/v1/documentreleaseingestion/document_release_ingestion_service.proto`, `GenerateDocumentUploadUrls` line 114, `BatchCreateDocumentReleaseIngestions` line 132.
- Backend: `apiserver/internal/documentreleaseingestion/action_generate_document_upload_urls.go`, `action_batch_create_document_release_ingestions.go`, `storage_helper.go`.
- Up to 50 files; extension-derived PDF, DOC, DOCX, HTML, CSV and TXT MIME types; private profile bucket; default five-minute upload URL lifetime.
- Existing response already exposes HTTP method, ordered multipart form values, and signed headers. Existing request accepts an optional per-file KiB limit (zero means unlimited), passed to shared/objectstorage. This is a mechanism precedent, not a server-enforced product maximum.
- Upload keys are `{profile}/kb-ingestion/{knowledge-base}/{upload-id}/{filename}`. Parent requires a knowledge base; roles are ADMIN/SUPER_ADMIN/VA_ADMIN.
- Follow-up batch creation validates storage URI, expected bucket and object existence, then creates KB ingestion records. It is not a generic staging-to-permanent commit operation.
- Reuse/extract the upload response shape and helper implementation, not its KB-scoped public RPC for coaching.

### 3. AIAgentGatewayService: attachment lifecycle precedent

- Proto: `cresta/v1/aiagentgateway/ai_agent_gateway_service.proto`, Create/Get/DeleteWidgetSessionAttachment lines 226/267/310 in the task proto worktree.
- Backend: `bot-server/internal/aiagentgateway/action_{create,get,delete}_widget_session_attachment.go`; TTL in `bot-server/shared/aiagentgateway/keys/keys.go:119`.
- Create sends bytes through the RPC, allows PNG/JPEG only and caps at 8 MiB. Handler sniffs content MIME, validates session creator, uploads temporary bytes and writes Redis metadata.
- Metadata includes opaque ID, original filename, MIME, byte size, timestamp and session association. Metadata lifetime is 30 minutes; reads return fresh signed URLs capped by remaining lifetime.
- Get/Delete validate session ownership and metadata association. Delete removes the temporary object and metadata; missing metadata is treated as already deleted after caller validation.
- Useful ownership/ID/download/delete design precedent, but transport, image policy, bot-server/session scope and temporary lifetime make direct reuse unsuitable for durable coaching attachments.

### Other matches

- `AIReceptionistService.GenerateKnowledgeBaseSourceFileUploadURI`: agent/tenant-specific KB-source upload; documented same-filename overwrite. Product-specific, not a general attachment contract.
- `UploadUserProfilePicture`: avatar-specific upload.
- KMS gateway DownloadFile/DownloadAttachment RPCs retrieve remote provider content rather than managing user-uploaded private attachments.

## Recommendation

Prefer an additive extension of the existing generic S3FileService (final API naming/ownership still to decide) over automatically creating another generic service. Use document-ingestion's self-describing upload response and shared/objectstorage mechanisms, and extract/generalize the quiz/training commit logic. Keep legacy quiz/training compatibility and their storage configuration. Later product attachment APIs must authorize their owning record before signing downloads or deleting bytes.

This search provides implementation precedents and narrows new work; it does not eliminate the need for generic commit semantics or document authorization. No source edits, tests, PR changes, external messages, or live storage operations. No credentials used; local filesystem/Git inspection only.
