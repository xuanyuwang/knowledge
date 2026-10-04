# CONVI-7707 attachment scope assessment

- Date: 2026-09-21
- Primary domain: training-simulator; primary subdomain: training-content
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Inspected worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7707-reads`, branch `xw/convi-7707-scenario-image-reads`, HEAD `98c5fa63d2` (clean).
- Related contract evidence: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707`, HEAD `153723fe64`.
- Request: assess incremental effort and whether the new attachments requirement should expand CONVI-7707. Assessment only; no implementation or PR changes authorized by this request.

## Product evidence

Read the complete Glean-returned content of [PRD: Coaching Attachments & Agent Profile Record](https://docs.google.com/document/d/1RKWHtW_khFTdi00K6rKKkmalXbrbLH2O9XEXj5BbRfQ/edit?tab=t.0), owner Eleanore An, draft, indexed update time 2026-09-21T18:08:19Z.

- P0: attachments on coaching plans, coaching sessions including group coaching, and individual scorecards/evaluations; attachment deletion.
- P0: agent-level documents viewable across use cases, authorized retrieval/download, acknowledgement (requires further discovery).
- P0.5: restricted visibility independent of coaching permissions; submitting supervisor and authorized management; further discovery required.
- P1: audit trail, agent history and bulk export. P2: full-text search, lifecycle/expiry alerts.
- Journeys mention pictures and PDFs; discovery quotes mention Word documents. Exact allowed file types and sizes remain open questions, not finalized requirements.
- Training Simulator certification appearing on the agent profile is explicitly a later-phase journey. The PRD does not require scenario document ingestion, AI document understanding, or certification implementation in CONVI-7707.

## Source-confirmed reuse and gaps

1. `training_module_image.proto` already carries file-neutral information: key, display name, MIME, byte size, response-only signed URI. No image dimensions/decoding are required by this descriptor. A document descriptor can reuse this shape.
2. `action_generate_training_media_upload_urls.go` and `action_commit_training_media.go` accept PNG/JPEG/WebP. Commit uses HeadObject content type and length, then copy and best-effort staging deletion. It has no byte-content validation or size cap.
3. Legacy quiz upload/commit adapters call these same helpers. Broadening the shared allowlist alone silently broadens legacy quiz behavior. A document extension needs separate allowed-type policies for quiz and generalized callers, with regression coverage.
4. `training_scenarios.images` stores `TrainingModuleImageListSQL`; converters clone descriptors and strip signed URIs. This storage mechanism can hold file metadata, but the published API and JSON list names have image semantics. Renaming new draft types/fields must also update SQL type annotations, generated GORM/goverter/BUILD artifacts, tests, and clients. Renaming JSON fields after persistence or publication needs migration/compatibility planning.
5. Storage mechanics (presigned upload, staging, commit, signed download) are reusable. Keep the existing configured bucket and quiz-media paths for current work; no bucket provisioning or object migration is needed for a training-only format extension.
6. `shared/objectstorage` already supports `ContentDisposition` for downloads. It also supports upload size limits, but S3 switches to presigned POST with form fields when the limit is set. The current training RPC returns a PUT URL only and discards method/form fields. A strict upload-time size limit therefore needs an API/client change; commit-time HeadObject rejection is smaller but does not prevent oversized staging uploads.
7. Current endpoints and key validation are customer/profile scoped with training author roles. They do not model attachment owner/parent authorization, uploader, timestamps, document category, visibility, acknowledgement, or customer-wide agent listing. Group coaching associations and cross-use-case access require domain-specific authorization and persistence design.
8. Commit returns descriptors without creating an attachment DB resource. Scenario revisions own references. Removing a reference does not delete an object, which may remain used by older revisions. The PRD's delete operation requires an explicit detach/delete/retention contract; it cannot simply reuse staging cleanup.

## Recommendation and estimates

Recommendation, pending user decision: keep the coaching/agent-record feature in separate tickets and keep CONVI-7707 focused on scenario images. Reuse low-level storage operations later; do not make coaching depend on TrainingSimulatorService or store restricted agent records as training images.

Rough incremental estimates for one backend engineer, based on inspected code (not delivery commitments; exclude frontend, review/release wait, and unresolved product discovery):

| Change | Incremental effort | Boundary |
| --- | --- | --- |
| Generalize the new draft image descriptor/field naming only | About 0.5–1 day | Mechanical changes across proto, SQL annotations, generated converters/models, stack and checks; no new feature behavior |
| Support PDF attachments within existing training ownership/read paths | About 1–3 days total | Descriptor naming if desired, allowlist separation, commit-size validation, download behavior and focused tests; assumes direct download, settled type/size policy, no previews/extraction, and no strict upload-time size rejection |
| Implement coaching and agent-profile PRD | Multi-week, multiple tickets | New attachment persistence/ownership, four integrations, authorized listing/download/deletion, cross-use-case semantics and acknowledgement; restricted visibility and later priorities add separate work |

The full PRD estimate is deliberately coarse because permission and acknowledgement requirements are unresolved and those domain implementations were not exhaustively audited. A neutral shared descriptor is an optional preparatory change, not necessary to unblock the current image feature. If choosing it, select a domain-neutral package after dependency review; changing only `Image` to `Attachment` under `trainingsimulator` does not make it appropriate for coaching consumers.

## Validation and handoff

### Clarified scope: generalize the foundation, not implement the PRD

The user clarified that the intended delta is making the new upload/commit infrastructure reusable for the PRD, while leaving all coaching/agent-profile feature integration for later. The earlier recommendation to defer the entire attachment feature did not answer that narrower question.

Revised recommendation: generalizing the foundation now is reasonable. Budget roughly **2–4 additional backend engineer-days**, excluding review/release wait and frontend adoption, for neutral proto/API placement, reusable implementation, file policies, dependent regeneration and validation across the existing stack. This is a planning estimate, not measured work. Simple file-type/name changes alone remain smaller, but do not remove the Training Simulator service dependency.

Proposed scope:

- A neutral file/attachment descriptor and list wrapper, retaining the five existing metadata fields. Place them in a dependency-appropriate shared package; do not invent ownership/acknowledgement fields in the binary descriptor.
- Neutral upload/commit contracts and shared backend helpers, with training and legacy quiz adapters where compatibility requires them. Separate service wiring and RPC authorization from reusable storage operations.
- Configurable, server-selected allowed-type/size policies; preserve legacy quiz image-only behavior. Keep scenario image rendering semantics unchanged unless separately requested.
- Preserve existing buckets/keys. Pass validated storage scope into helpers rather than baking training ownership into the reusable implementation. Future cross-use-case attachment ownership and permission checks belong to the product integration.
- Expose upload method, signed headers and ordered form fields in the new generic upload response so existing storage-provider size-limit support is usable. Commit still checks metadata; download helpers support attachment disposition.
- Update SQL wrapper annotations and regenerate models/converters/build metadata; verify old quiz behavior, new file types/limits, signed URLs and scenario persistence. Independent attachment tables are not needed for this foundation.

Future callers must authorize the owning document/parent before signing downloads or allowing deletion. A generic upload/commit result is file metadata, not a grant of access to a coaching/HR document. Cross-use-case agent records can later reference the file independently of its physical storage scope, subject to an authorized resolver.

Additional source check: existing `cresta.v1.common.media` contains WebM segment types, not an attachment descriptor. Existing `S3FileService` already exposes generic temp upload and restricted download, but has no corresponding media commit contract. Review reuse/extension of that service before adding another generic service; its broad upload and SUPER_ADMIN-only download annotations are not the future coaching permission model.

- Read-only local code assessment plus Glean document retrieval. No tests run because production code was unchanged; earlier implementation test results are not new validation of attachments.
- No external updates, PR edits, merges, releases, or deployments.
- Used the configured Glean connector; no AWS, Okta, Azure, or SSH credentials used.
- Next: user decides whether any limited naming preparation belongs in CONVI-7707. Separate attachment design should first settle allowed types/sizes, ownership across use cases, visibility, deletion and acknowledgement semantics.
