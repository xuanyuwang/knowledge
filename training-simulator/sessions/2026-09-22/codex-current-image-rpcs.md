# Current Training Simulator image RPC behavior

- Date: 2026-09-22
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Branch/worktree: `main`, `/Users/xuanyu.wang/repos/go-servers`; relevant Training Simulator and S3File directories match remote main `7dc40b61573fc146ad8a27c7bd0ac1cf81de022f` (verified through GitHub API and local origin/main diff).
- Contract: `/Users/xuanyu.wang/repos/cresta-proto`, main checkout. Training Simulator and S3File service proto blobs were verified against remote main `718c0b4d1cf4f6026a7226f3ee263d8c631e51e9`; local service differences are unrelated lesson additions.
- Scope: high-level source investigation to inform possible S3FileService extension. No deployment validation, source edits, or runtime tests.

## Operations

1. `GenerateQuizMediaUploadUrls`: accepts profile parent and 1–20 filename/MIME pairs. Supports PNG, JPEG, WebP; rejects filename path separators. Resolves the profile private bucket, creates unique permanent keys `quiz-media/{profile}/{uuid}/{filename}`, and signs PUTs to corresponding `tmp/quiz-media/...` keys. Returns original filename, MIME, permanent key, signed upload URL, and expiry (default 15 minutes). No bytes uploaded or quiz saved by the RPC.
2. Client HTTP PUT: uploads bytes directly to storage using the signed URL and matching Content-Type. This is a separate client/storage operation, not a Training Simulator RPC.
3. `CommitQuizMedia`: accepts profile and 1–20 permanent keys with optional display names. Checks the profile-specific key prefix; HEADs staging objects to confirm existence and allowed Content-Type; copies each to its permanent key; attempts staging deletion. Returns `QuizQuestionImage` descriptors containing key, display name, MIME, size. Does not save a quiz or generate download URLs.
4. `CommitQuizTemplate`: separately saves the descriptors within quiz questions in a new template/revision and DB transaction. Media commit and DB save are separate operations.
5. `GetQuizTemplate`: loads that quiz revision and questions, resolving the private bucket when images exist, then supplies fresh signed image GET URLs (default 15 minutes).

Upload/commit annotations allow MANAGER, MANAGER_2ND, ADMIN, SUPER_ADMIN, QA_ADMIN. Get also allows AGENT. Both media RPCs are profile-scoped, without a quiz/question owner in the request.

## Important semantics for reuse

- Commit uses storage metadata MIME, not image-byte inspection, and does not enforce a byte-size maximum in the inspected handlers.
- Commit processes files sequentially without rollback: a later error may follow earlier successful copies. Successful staging deletion means replaying the same commit normally fails with missing staging data; it is not an idempotent finalization contract.
- Staging cleanup failure is logged but does not fail a successful permanent copy.
- No standalone media download/delete RPC is present on TrainingSimulatorService main. Download signing occurs within the owning quiz read.
- The proto describes presigned download URLs as output-only/not persisted; this investigation does not assert write-side URL stripping. The image SQL converter directly wraps image messages.
- The CONVI-7707 local draft has GenerateTrainingMediaUploadUrls/CommitTrainingMedia; these are distinct from the current main contract and are not included as existing main operations.

## S3FileService comparison and recommendation

Current S3FileService has GenerateTempUploadURI (single temporary file), GeneratePresignedDownloadURI (SUPER_ADMIN annotation), and integration ListObjects. Upload returns a signed URL, path, storage URI, expiry; default allowed extensions are CSV, JSON, PDF, Parquet, ZIP, excluding quiz images. It uses the scoped storage factory with private storage/fallback and uploader-based keys. There is no generic commit RPC in this service.

An additive S3File extension can own generic upload preparation, staging verification/promotion and a neutral file descriptor. Product APIs should retain attaching files to records, revisioning, and product authorization. Decisions remain on accepted types/size policy, key compatibility, retry/partial-failure semantics and download authorization. Merely allowing image extensions does not replace the quiz lifecycle.

## Source map

- Proto: `cresta/v1/trainingsimulator/training_simulator_service.proto`, `quiz_template.proto`; `cresta/v1/s3file/s3_file_service.proto`.
- Backend: `apiserver/internal/trainingsimulator/action_generate_quiz_media_upload_urls.go`, `action_commit_quiz_media.go`, `storage_helper.go`, `action_commit_quiz_template.go`, `action_get_quiz_template.go`, `converter/converter.go`.
- S3File: `apiserver/internal/s3file/action_generate_temp_upload_uri.go`, `action_generate_presigned_download_uri.go`.
- Used existing GitHub CLI authentication for read-only source freshness verification. No AWS, Okta, Azure or SSH credentials used.

Related: [earlier RPC reuse assessment](codex-convi-7707-rpc-reuse.md). No design decision or implementation scope change was made.
