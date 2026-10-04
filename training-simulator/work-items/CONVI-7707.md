# CONVI-7707: Scenario context image backend

**Status:** [cresta-proto#9995](https://github.com/cresta/cresta-proto/pull/9995) merged. [go-servers#33043](https://github.com/cresta/go-servers/pull/33043) implements its generic upload/commit RPCs and is open for review. [cresta-proto#10057](https://github.com/cresta/cresta-proto/pull/10057) adds only scenario image descriptors and is ready for review; draft [go-servers#33045](https://github.com/cresta/go-servers/pull/33045) stacks scenario persistence and signed reads on #33043. The older proto #9918 and go-servers #32494/#32597/#32598 are closed and superseded.
**Primary domain:** training-simulator
**Primary subdomain:** training-content
**Official ticket:** [CONVI-7707](https://linear.app/cresta/issue/CONVI-7707/be-of-add-image-to-scenario-context)
**Parent:** [CONVI-7626](https://linear.app/cresta/issue/CONVI-7626/add-image-to-scenario-context)
**Last updated:** 2026-09-29

## Objective and decision

**Current execution (2026-09-29):** Use S3FileService for storage preparation and commit; FE passes namespace `scenario-images` and retains scenario type/size policy. The backend validates customer/profile/namespace-scoped permanent keys when saving scenario references. Legacy quiz APIs keep their current key paths. The replacement scenario PR requires #10057 merged and released before a normal proto dependency bump and Bazel validation. Additive schema deployment must precede backend rollout; frontend integration and staging end-to-end validation remain pending. See [generic backend session](../sessions/2026-09-29/codex-convi-7707-s3file-backend.md), [scenario session](../sessions/2026-09-29/codex-convi-7707-scenario-replacement.md), and [proto session](../sessions/2026-09-29/codex-convi-7707-image-contract.md). The earlier implementation plan and published stack below are historical.

**Current direction (2026-09-28):** centralize generic upload preparation and permanent-object commit in S3FileService. FE configures a required `storage_namespace` label for storage location, plus parent and file metadata; each file may also carry an optional positive `max_upload_size_bytes` for signing. BE selects the bucket, owns disjoint staging/permanent roots and authorized customer/profile scope, and generates complete keys. FE continues to choose domain type/size limits for this phase; a FE-supplied maximum constrains the signed upload but is not trusted backend domain policy. Namespace grants no access and selects no domain policy. Removed `FileStorageConfig` and bucket identity in the two new responses; BE resolves it from configuration. This supersedes September 23's FE-selected bucket/prefix direction. Domain records and existing quiz APIs remain unchanged. Issued-upload binding is explicitly deferred by the user; its temporary proto addition was reverted. Published `610381e74fdf6b0d6d6ef0cf2ab96858a9acc78d` to [#9995](https://github.com/cresta/cresta-proto/pull/9995); local checks passed, backend implementation and CI remain separate. See [namespace contract session](../sessions/2026-09-24/codex-s3file-namespace.md) and [size-limit change](../sessions/2026-09-28/codex-s3file-upload-size-contract.md).

The implementation and release plan below records the earlier TrainingSimulatorService-based draft. Reconcile that draft with the S3File contract before proceeding with its media API release.

Support scenario-level images as visual reference material for trainees. The September 21 user direction supersedes the earlier decision to expose quiz-specific names: introduce a generalized protobuf, SQL/converter, and upload/commit surface, use it for scenarios, then migrate legacy quiz APIs and protobuf separately. Image pixels are not inputs to Customer AI.

## Source context

- Replacement generic storage: `/Users/xuanyu.wang/repos/go-servers-convi-7707-s3file-backend`, branch `xw/convi-7707-s3file-backend`, head `d0191114b2`, [go-servers#33043](https://github.com/cresta/go-servers/pull/33043), base main. Released proto dependency is `v2.30.26`.
- Replacement scenario contract: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-image-contract`, branch `xw/convi-7707-scenario-image-contract`, commits `0c231aed80` and `52f2c4ddd8`, [cresta-proto#10057](https://github.com/cresta/cresta-proto/pull/10057) ready for review, base main.
- Replacement scenario backend: `/Users/xuanyu.wang/repos/go-servers-convi-7707-scenario-v2`, branch `xw/convi-7707-scenario-images-v2`, head `1922a92b4a`, draft [go-servers#33045](https://github.com/cresta/go-servers/pull/33045), base #33043 branch.
- Historical #9918 and backend #32494/#32597/#32598 are closed; their branches remain intact for traceability. The old source-context bullets below describe that closed stack.

- New generic storage contract: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-s3file`, branch `xw/convi-7707-s3-file-lifecycle`, commit `610381e74fdf6b0d6d6ef0cf2ab96858a9acc78d`, based on main `029507d21d734620c116370c3e96c00749bd28e0`: [cresta-proto#9995](https://github.com/cresta/cresta-proto/pull/9995).

- `/Users/xuanyu.wang/repos/cresta-proto-convi-7707`, branch `xw/convi-7707-scenario-images`: draft [cresta-proto#9918](https://github.com/cresta/cresta-proto/pull/9918).
- Persistence: `/Users/xuanyu.wang/repos/go-servers-convi-7707`, branch `xw/convi-7707-scenario-images`, head `fdd3a31db1`: draft [go-servers#32494](https://github.com/cresta/go-servers/pull/32494), base `main`.
- Media APIs: `/Users/xuanyu.wang/repos/go-servers-convi-7707-media`, branch `xw/convi-7707-media-apis`, head `63be8a2cd8`: draft [go-servers#32597](https://github.com/cresta/go-servers/pull/32597), based on #32494.
- Signed reads: `/Users/xuanyu.wang/repos/go-servers-convi-7707-reads`, branch `xw/convi-7707-scenario-image-reads`, head `98c5fa63d2`: draft [go-servers#32598](https://github.com/cresta/go-servers/pull/32598), based on #32597.
- Complete pre-split implementation preserved locally at `xw/convi-7707-before-split` (`68aacebf7f`).
- At the time of the older stack, backend declared `cresta-proto v2.25.16` and awaited #9918. The replacement S3File branch uses released `v2.30.26`; the scenario branch still awaits #10057's release and dependency bump. Temporary local module/Bazel overrides are validation-only and must not be committed.
- Existing quiz upload/commit/Get behavior and scenario revision/read paths are the implementation precedents. September 18 investigation and validation remain historical evidence; current design is below.

## Implementation and action items

1. **Generalized protobuf.** Add `training_module_image.proto` with `TrainingModuleImage` (object_storage_key, display_name, mime_type, size_bytes, output-only presigned_uri) and `TrainingModuleImageList`. Keep metadata field numbers and JSON shape aligned with the legacy image contract. `TrainingScenario.images = 14` uses `TrainingModuleImage`; it has no quiz proto dependency.
2. **Generalized RPCs.** Add `GenerateTrainingMediaUploadUrls` and `CommitTrainingMedia` with new request/response types in `training_simulator_service.proto`. Routes are `trainingMedia:generateUploadUrls` and `trainingMedia:commit` under the profile parent. Preserve author roles and 1–20 entries per batch, PNG/JPEG/WebP support, and 15-minute defaults.
3. **Shared backend infrastructure.** Implement upload/commit once against the generic contract. Both generic and legacy quiz APIs use the existing profile-configured private bucket and the existing `quiz-media/{profile}/{upload-id}/{filename}` permanent keys plus `tmp/quiz-media/...` staging keys. General names `trainingMediaPrefix` and `trainingMediaTempPrefix` retain these literal values; no new bucket or external storage configuration is introduced. Legacy quiz RPCs remain thin adapters with their existing public types, routes, and upload TTL setting.
4. **SQL schema and models.** Keep nullable `director.training_scenarios.images JSONB`, mapping it to generated `TrainingModuleImageListSQL`. Regenerate GORM; do not change quiz_questions storage. Legacy NULL scenario rows read as no images. Deploy schema before backend code using the new model.
5. **Converters.** Add reusable `TrainingModuleImageListSQL` ↔ image-slice conversions and regenerate goverter. The generalized write converter clones image descriptors and removes `presigned_uri`, preserving caller-owned data. Keep the legacy quiz converters for compatibility.
6. **Scenario persistence and reads.** Batch create/update validate generic permanent keys for the selected profile before VA/DB side effects. Image-only changes create a scenario revision without changing the VA revision. `ListTrainingScenarios` resolves the private bucket only for image-bearing results and generates fresh download URLs after profile/namespace validation. Expanded module/lesson reads inherit this path. Save responses contain stable descriptors.
7. **Generated artifacts and checks.** Generate protobuf Go/SQL/RPC code locally for backend validation, but exclude generated proto artifacts from the PR. Run proto lint, API lint, dependency and compatibility checks, affected Bazel builds, schema/converter generation, and targeted new plus legacy tests. Run Gazelle for build metadata.
8. **Release and rollout.** Publish the additive proto, update backend/client dependencies, apply additive schema before backend rollout, then validate upload → commit → scenario save → trainee lesson reload → URL refresh in staging. Frontend integration and deployment are separate follow-ups.

## Legacy quiz migration plan

- Keep existing `QuizQuestionImage`, `QuizQuestionImageList`, quiz question field types, upload/commit RPC names, HTTP routes, and stored keys during this change. No deprecation/removal deadline is imposed.
- Shared internal upload/commit code prevents behavior from diverging while the contracts coexist.
- A later migration must introduce a compatible quiz read/write surface for `TrainingModuleImage` and coordinate generated Go/TypeScript clients. Existing `quiz-media/` objects stay readable through the shared profile-bound storage helpers. Do not simply change the type of an existing published field: protobuf source compatibility and generated client types are part of the contract even when wire/JSON layouts match.
- Generic endpoints and scenario references accept the same permanent quiz-media keys as legacy quiz APIs, with profile validation. No object movement, JSONB backfill, bucket provisioning, or destructive cleanup is part of this change.

## Behaviors and boundaries

- Commit uses storage metadata to validate MIME and report size, copies staged objects into permanent storage, then best-effort deletes staging objects. It does not persist content metadata.
- Batches retain the existing sequential, non-atomic behavior. Retry after successful temporary-object deletion is not idempotent. Frontend one-image commits reduce partial-batch exposure; response-loss recovery is a separate design.
- No backend byte-size cap or image decoding is introduced; frontend limits must not be presented as backend guarantees.
- Empty images replace/clear the scenario list under existing full-replacement saves. Older editors omitting images must be coordinated before rollout. Removing a reference does not delete objects needed by prior revisions.
- Fresh GET URLs are response-only; clients must refresh them during long sessions. Scenario reads still select the latest revision; historical assignment snapshots are unchanged.
- Auth remains the existing author role set for writes; agent read access follows scenario/module/lesson annotations. Bucket selection remains customer/profile-config scoped and image keys profile scoped.

## Validation and current limits

Proto lint/API lint/dependency checks, scoped Buf compatibility against current main, affected proto Bazel build, GORM/goverter generation, and both focused backend Bazel targets passed. Proto head: `153723fe64`; backend stack tip: `98c5fa63d2`. Each isolated stack stage passed focused Bazel checks with the local proto override. Production files at the tip match the pre-split implementation byte-for-byte; only test organization and its BUILD entry differ. Validation results and source commits are recorded in the [September 21 session](../sessions/2026-09-21/codex-convi-7707-general-media.md). No staging/production storage, database, RPC, release, or deployment validation is claimed. The backend draft cannot compile in ordinary CI until the new proto is published and its dependency is bumped.

## Published review stack

| Order | Draft PR | Base | Review scope | Diff |
| --- | --- | --- | --- | --- |
| 1 | [#32494](https://github.com/cresta/go-servers/pull/32494) | main | Scenario schema/model, SQL converters, write normalization/validation and persistence tests | 13 files, +378/-28 |
| 2 | [#32597](https://github.com/cresta/go-servers/pull/32597) | #32494 branch | Generalized upload/commit handlers, legacy quiz adapters and media tests | 7 files, +333/-150 |
| 3 | [#32598](https://github.com/cresta/go-servers/pull/32598) | #32597 branch | Scenario image presigning, List enrichment and read tests | 4 files, +103/-0 |

The proto release/dependency bump remains a prerequisite for part 1. Its persistence conversion is kept together with the schema/model to avoid an incomplete generated converter mapping. Part 1 uses the existing quiz key helper; part 2 generalizes the helper with the same bucket/paths. New generic methods inherit Unimplemented in part 1 until part 2 implements them. Keep frontend enablement until part 3 completes the backend flow.

The historical plan was to merge in order, rebasing and retargeting dependent PRs onto main after their predecessors. Apply schema before deploying code using the model. The three old PRs were closed on 2026-09-29 after replacement #33045 opened; none merged or deployed.

## Next action

Review #33043 and complete its CI. Merge and release #10057, then bump #33045 to that published proto in `go.mod`, `go.sum`, and `deps.bzl`; run normal focused Bazel tests. Apply the additive schema before backend rollout, then validate upload → commit → scenario save → trainee read with fresh URLs in staging. FE integration and legacy quiz migration are separate.

## Attachment extension assessment (2026-09-21)

**RPC reuse search (2026-09-22):** Existing `S3FileService` provides generic temporary upload and admin-only download; document ingestion already has multi-format batch uploads and self-describing size-limited upload responses. Widget attachments provide a temporary, session-authorized lifecycle precedent. No generic staging commit found outside quiz/training. Consider additive S3FileService extension plus extracted document-upload/quiz-commit internals; final placement remains undecided. See [search evidence](../sessions/2026-09-22/codex-convi-7707-rpc-reuse.md).

**Clarification:** the user asks to generalize only the new infrastructure for future PRD reuse. Recommendation: this bounded extension is reasonable, roughly **2–4 incremental backend engineer-days** for neutral descriptors/upload/commit placement, reusable handlers and file policies, upload-method/headers/form-field support, regeneration and regression checks. Coaching/agent-profile integration remains future work. This supersedes the earlier blanket recommendation to defer generalization. No implementation changes made; see the clarified scope in the linked assessment.

Reviewed the draft [Coaching Attachments & Agent Profile Record PRD](https://docs.google.com/document/d/1RKWHtW_khFTdi00K6rKKkmalXbrbLH2O9XEXj5BbRfQ/edit?tab=t.0). Recommendation pending user decision: keep its implementation separate from CONVI-7707. The current descriptor/storage mechanics can support non-image files, but the PRD adds four attachment owners, cross-use-case agent records, authorized retrieval, deletion and acknowledgement, with restricted visibility also planned. It does not request scenario document ingestion.

A training-only PDF backend extension is roughly 1–3 incremental engineer-days under existing ownership and direct-download semantics, excluding frontend/release work and strict upload-time size enforcement. The broader PRD is multi-week work across several tickets. Existing quiz adapters share the MIME policy and must remain image-only if generalized callers are broadened. No source or PR changes made for this assessment. See [scope analysis and evidence](../sessions/2026-09-21/codex-convi-7707-attachment-scope.md).

## Timeline

- 2026-09-29 — Replaced the obsolete training-specific media stack after #9995 merged. Opened generic S3File backend [#33043](https://github.com/cresta/go-servers/pull/33043) at `414c716a69`; full affected Go package suite, Bazel build, Gazelle and diff checks passed. Fixed a valid review finding by hiding provider error text from API responses at `d0191114b2`, with focused tests. Opened reduced scenario proto draft [#10057](https://github.com/cresta/cresta-proto/pull/10057) at `52f2c4ddd8`; Buf lint/breaking and Bazel build passed, and a Gazelle-only CI failure was fixed. Opened scenario persistence/read draft [#33045](https://github.com/cresta/go-servers/pull/33045), rebased to `1922a92b4a`; focused create/update/list/converter and signed-read suites passed with an uncommitted local proto override. Closed prior backend #32494/#32597/#32598; #9918 had already closed. Ordinary scenario CI, schema rollout, FE integration and staging validation remain. [Sessions](../sessions/2026-09-29/codex-convi-7707-s3file-backend.md).

- 2026-09-28 — Added optional positive per-file `max_upload_size_bytes` to `GenerateFileUploadUrls` in #9995 so the backend can request a bounded signed upload. S3 signing uses POST with form fields for a positive maximum and PUT when absent; response method/headers/form values remain necessary. FE chooses the maximum, so it is not trusted backend domain policy. Published `610381e74fdf6b0d6d6ef0cf2ab96858a9acc78d`; scoped lint, breaking check, formatting and S3File Bazel build passed. Backend handler work remains. [Session](../sessions/2026-09-28/codex-s3file-upload-size-contract.md).

- 2026-09-28 — GitHub org search found four handwritten `GenerateTempUploadURI` workflow entry points plus one shared Director wrapper, among 68 matching files. Reusing the legacy RPC must preserve these consumers; no source changes. [Session](../sessions/2026-09-28/codex-s3file-org-usages.md).

- 2026-09-28 — Validated #9995 comments on POST/PUT, size-limit inputs and possible reuse of `GenerateTempUploadURI`. Bucket removal is complete; current new proto has no size-bound input. Existing temp RPC is not a drop-in for the namespace-scoped batch/commit flow. Review only, no source changes. [Session](../sessions/2026-09-28/codex-s3file-pr-9995-upload-review.md).

- 2026-09-25 — Added ordered per-file `Status status` results to `CommitFiles` in #9995, reporting committed metadata, first file error and not-attempted remainder. Published `48169fb372`; local proto, dependency and S3File Bazel checks passed. Backend implementation and idempotent retry remain. [Session](../sessions/2026-09-25/codex-s3file-per-file-commit-status.md).

- 2026-09-24 — Removed `bucket_name` from new upload and committed-file responses at user request; BE resolves bucket from configuration. Published `76fcca55cb` to #9995; scoped Buf/API lint, breaking check, formatting and S3File Bazel build passed. Existing RPCs unchanged. [Session](../sessions/2026-09-24/codex-s3file-remove-bucket.md).

- 2026-09-24 — User deferred token binding; reverted `8b11b1bd28` with `9ec3d180a1`, restoring the exact namespace-only contract from `cf83ee11a2`. Build/diff checks passed and PR description updated.

- 2026-09-24 — Removed unnecessary reservations for unpublished `storage_config`, assigning `storage_namespace = 3` in both requests; pushed `cf83ee11a2`. Scoped lint/compatibility and Bazel build passed.

- 2026-09-24 — Updated #9995 to expose only `storage_namespace` as storage configuration, reserving removed request field 3/name and keeping bucket metadata in responses. BE owns bucket/key layout and tenant scope. Published `7a8e98cda4`; Buf/API lint, compatibility, dependency/import/path checks, formatting and S3File Bazel build passed. Updated PR body while preserving automated warnings. [Session](../sessions/2026-09-24/codex-s3file-namespace.md).

- 2026-09-23 — Clarified that FE must choose the bucket and full storage paths. Revised #9995 to use shared `FileStorageConfig` on upload/commit and include `bucket_name` in upload results and `StoredFile`. Removed implicit profile-private bucket and generated quiz/profile prefix choices. Published `325589945f`; local lint, compatibility, dependency and Bazel checks passed. PR description updated with timestamp/body-hash guards; automated codegen warning and unchecked acknowledgement preserved.

- 2026-09-23 — Created independent draft [proto#9995](https://github.com/cresta/cresta-proto/pull/9995) for `GenerateFileUploadUrls`, `CommitFiles` and `StoredFile`. Initial domain configuration/restrictions are enforced by FE; profile/key scope remains a server responsibility. Existing quiz/training drafts unchanged. Buf/API lint, compatibility, dependency/import/path checks, formatting, Gazelle and `bazel build //cresta/v1/s3file:all` passed. Evidence: [session](../sessions/2026-09-23/codex-s3file-lifecycle-proto.md).

- 2026-09-18 — Investigated live ticket and current source; confirmed quiz precedent, scenario revision/read integration, and retry/URI boundaries. Evidence: [session](../sessions/2026-09-18/codex-convi-7707-scenario-images.md), [daily log](../log/2026-09-18.md).
- 2026-09-18 — User agreed to reuse `QuizQuestionImage`, `QuizQuestionImageListSQL`, and existing converters, accepting their quiz-specific names across the coupled layers. Revised the plan to remove duplicate scenario image types and reuse download signing. Implementation remains pending.
- 2026-09-18 — User agreed to reuse `GenerateQuizMediaUploadUrls` and `CommitQuizMedia` as well. Removed proposed scenario media RPCs and key prefixes; added an ordered definitions/preparation checklist covering protobuf, shared docs, generated artifacts/dependencies, SQL, DB models, converters, normalization, and rollout ordering.
- 2026-09-18 — Implemented preparation in dedicated worktrees and opened draft [proto#9918](https://github.com/cresta/cresta-proto/pull/9918) / [backend#32494](https://github.com/cresta/go-servers/pull/32494). Schema/model/converter generation and local focused tests passed; published dependency bump and signed reads remain follow-ups. Evidence: [implementation session](../sessions/2026-09-18/codex-convi-7707-preparation.md).
- 2026-09-21 — User superseded quiz-name reuse with generalized training media infrastructure. Revised both drafts to introduce general types/RPCs/SQL converters, share upload/commit internals with legacy adapters, and integrate scenario signing. See [session](../sessions/2026-09-21/codex-convi-7707-general-media.md).

- 2026-09-21 — Accepted `TrainingModuleImage` / `TrainingModuleImageList` in `training_module_image.proto`. A separate file avoids cycles because `training_module.proto` imports both scenario and quiz definitions. Updated dependent SQL wrapper/converters and tests; RPC names, fields, and storage paths are unchanged.

- 2026-09-21 — User requested retaining the existing external storage setup. Reused the existing private bucket and quiz-media permanent/staging paths for the generalized APIs, restored an explicit temporary-prefix constant under a general name, and removed the unnecessary per-endpoint namespace parameter.

- 2026-09-21 — Executed the approved three-PR split: persistence [#32494](https://github.com/cresta/go-servers/pull/32494), media APIs [#32597](https://github.com/cresta/go-servers/pull/32597), and signed reads [#32598](https://github.com/cresta/go-servers/pull/32598). Verified each draft/base/head, focused tests at each stage, and production equivalence at the stack tip.
