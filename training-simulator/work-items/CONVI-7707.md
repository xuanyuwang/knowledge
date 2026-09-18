# CONVI-7707: Scenario context image backend

**Status:** definitions/persistence preparation implemented in two draft PRs; published proto dependency and signed-read integration pending. Linear: In Progress as of 2026-09-18.
**Primary domain:** training-simulator
**Primary subdomain:** training-content
**Official ticket:** [CONVI-7707](https://linear.app/cresta/issue/CONVI-7707/be-of-add-image-to-scenario-context)
**Parent:** [CONVI-7626](https://linear.app/cresta/issue/CONVI-7626/add-image-to-scenario-context)
**Last updated:** 2026-09-18

## Objective and impact

Support images attached to individual scenarios and available to trainees during voice simulation, mirroring quiz-question media. Role: investigated, designed, and implemented the definitions/persistence preparation. Both source PRs are draft; no Linear updates, releases, or deployments made.

The parent description explicitly says images are stored at scenario level. CONVI-7707 itself has no description or comments. Working interpretation is visual reference material for the trainee; feeding image pixels to Customer AI is not established by the ticket and would require additional runtime design.

## Source context

### Implementation worktrees and drafts

- `/Users/xuanyu.wang/repos/cresta-proto-convi-7707`, branch `xw/convi-7707-scenario-images`, commit `01efcfb856`: [cresta-proto#9918](https://github.com/cresta/cresta-proto/pull/9918), draft.
- `/Users/xuanyu.wang/repos/go-servers-convi-7707`, branch `xw/convi-7707-scenario-images`, commit `7783ba6558`: [go-servers#32494](https://github.com/cresta/go-servers/pull/32494), draft; based on `1ae17fe508`.
- Backend still declares proto `v2.25.16`. It requires the released version containing #9918 before merge. Local verification uses `/tmp/convi-7707-go.mod` (temporary replacement) and a Bazel repository override against the generated proto worktree; neither local override is committed.
- Runtime signing in `ListTrainingScenarios` remains follow-up work. Upload/commit methods and storage prefixes are reused unchanged.

### Investigation snapshots

- Primary source: `/Users/xuanyu.wang/repos/go-servers`, local `main` checkout; inspected `origin/main` at `2995bc56b2b487fb0f4442d425ed716149266798`.
- Contract: `/Users/xuanyu.wang/repos/cresta-proto`, inspected `origin/main` at `ac81eebac8390e7b5b07057e24b00863d9cb7191`.
- Consumer evidence only: `/Users/xuanyu.wang/repos/director`, inspected `origin/main` at `ec8886f59746a1fd1deedd0216df0d6af6e70472`.
- All three refs checked against live GitHub main during the investigation. The main source checkouts were not switched or modified during investigation; implementation worktrees were created afterward as listed above.
- Close precedents: [go-servers#31193](https://github.com/cresta/go-servers/pull/31193) adds upload/commit; [go-servers#31303](https://github.com/cresta/go-servers/pull/31303) adds quiz Get/download signing; [cresta-proto#9587](https://github.com/cresta/cresta-proto/pull/9587) adds the quiz Get contract.

## Current conclusion

The quiz lifecycle is reusable. This is a bounded contract, storage, and response-enrichment change in `cresta-proto` and `go-servers`. The image belongs on `TrainingScenario`, not `TrainingModule`. Existing scenario creation/update handlers already serve module saves, and scenario listing already feeds expanded module and lesson reads.

Quiz flow: generate presigned PUT → upload bytes directly to temporary object storage → commit media and receive metadata → save metadata in content revision → generate fresh presigned GET on read.

## Proposed implementation

Reusing the existing quiz image types, upload/commit RPCs, and storage paths is agreed with the user. The contract, SQL/model/converter mapping, and write normalization/validation are implemented in draft PRs; signed-read enrichment below remains pending.

1. **Reuse the existing image contract.** Add optional repeated `QuizQuestionImage images = 14` on `TrainingScenario`, importing `quiz_template.proto`. Reuse `QuizQuestionImageList` and its generated `QuizQuestionImageListSQL` wrapper; do not introduce scenario-specific image messages or rename existing quiz types. Update the image-message documentation to cover quiz questions and scenario context images, including scenario read responses for `presigned_uri`. The quiz-specific name is an accepted tradeoff: reusing the coupled proto, SQL wrapper, and converters avoids duplicate representations and a coordinated client-type rename. The product's exact per-scenario count limit remains unspecified.
2. **Reuse upload and commit RPCs unchanged.** Scenario callers use `GenerateQuizMediaUploadUrls` and `CommitQuizMedia` with the existing profile parent, request/response messages, authoring roles, and validation. These methods have no quiz/question resource dependency; commit returns the reused `QuizQuestionImage` descriptors for the subsequent scenario save. Retain the `quizMedia` HTTP routes, `quiz-media/{profile}/{upload-id}/{filename}` permanent keys, and `tmp/quiz-media/...` staging keys. Update RPC/request documentation to describe both quiz and scenario usage. No new RPCs, routes, storage namespace, or upload/commit handlers are needed.
3. **Persist revisioned metadata using the existing SQL type and converters.** Add nullable `images JSONB` to `director.training_scenarios` with the `QuizQuestionImageListSQL` type comment. Regenerate GORM models/DAO and goverter mappings, reusing `ConvertQuizQuestionImageListSQLToSlice` and `ConvertQuizQuestionImageSliceToSQL`. Normalize scenario image metadata before conversion so `presigned_uri` is excluded from writes without duplicating the wrapper converters. Existing NULL rows map to no images; no historical data backfill is needed. The additive schema still needs deployment before code starts selecting/writing the new column.
4. **Use existing save paths.** Extend `BatchCreateTrainingScenarios` and `BatchUpdateTrainingScenarios` via their converters; `CreateTrainingModule` and `UpdateTrainingModule` already delegate to them. Persist a new scenario revision for add/remove/replace. Match existing full-replacement semantics: empty images clears the list. Coordinate clients so older editors that omit images do not inadvertently clear them. Validate image references against the selected profile and existing permanent `quiz-media/{profile}/` prefix before persistence/signing; clients should supply the committed descriptor.
5. **Sign reads in `ListTrainingScenarios`.** Lazily resolve the bucket only for scenarios with images, then reuse `presignQuizQuestionImages` to add fresh download URIs after conversion; the reused message type already matches its input. `ListTrainingModules(include_scenarios=true)` and `ListTrainingLessons(include_modules=true, include_scenarios=true)` inherit this automatically. Match quiz behavior: save responses contain stable descriptors; refreshed display URLs come from reads. Do not add storage I/O to the pure converter.
6. **Keep image-only changes outside VA inputs.** For trainee reference images, leave `buildSystemPrompt` and `needsVAUpdate` unchanged. Image-only edits create a scenario revision while retaining the existing VA revision. There is no established need for bot-server/Python/voice-pipeline changes.

## Definitions and preparation action items

Items 1–2 and 4–7 are implemented in the draft PRs, with local proto/model/converter generation and validation. Item 3 still needs the published proto release and committed backend/client dependency bumps. Item 8 remains rollout preparation only; no schema deployment was performed.

1. **Protobuf definition — `cresta-proto`.** In `cresta/v1/trainingsimulator/training_scenario.proto`, import `quiz_template.proto` and add `repeated QuizQuestionImage images = 14` with OPTIONAL field behavior. Verify field 14 is still free when implementing. The inspected quiz proto has no scenario import, so this introduces no import cycle.
2. **Shared contract documentation — `cresta-proto`.** In `quiz_template.proto`, broaden `QuizQuestionImage` / `QuizQuestionImageList` comments to cover scenario images, and describe fresh `presigned_uri` values on scenario reads. In `training_simulator_service.proto`, document that the existing generate/commit media RPCs and their messages also serve scenarios. Preserve RPC names, routes, permissions, request constraints, and message shapes.
3. **Generated artifacts and dependency preparation.** Run the repository's prescribed proto generation and lint/build/breaking/dependency checks, including any applicable import allowlist check. Release the additive contract, then update `go-servers`' proto dependency (`go.mod`, `go.sum`, and Bazel dependency metadata as required). No new image SQL wrapper or RPC stub definitions are needed. Frontend consumption of the new scenario field requires its corresponding generated-client release.
4. **SQL schema — `go-servers`.** In `apiserver/sql-schema/director/director-schema.sql`, add nullable `images JSONB` to `director.training_scenarios`; set its `go_type` comment to `github.com/cresta/cresta-proto/v2/gen/go/cresta/v1/trainingsimulator:QuizQuestionImageListSQL`, matching the quiz-question column. Preserve the scenario revision key and legacy NULL behavior. No new table or data backfill.
5. **Generated DB model/DAO.** Run `mage RegenerateGorm apiserver` per repository instructions. Inspect `apiserver/sql-schema/gen/model/training_scenarios.go` for `Images` using `QuizQuestionImageListSQL`; retain only relevant generator changes, including DAO/build metadata if generated. Do not hand-edit generated files.
6. **Scenario converter mappings.** Regenerate `apiserver/internal/trainingsimulator/converter/generated/goverter_gen.go` using the package's `go generate` directive. Existing goverter extensions already register `ConvertQuizQuestionImageListSQLToSlice` and `ConvertQuizQuestionImageSliceToSQL`; matching `Images` fields should use them without new wrapper converters. Verify both scenario conversion directions, NULL/empty behavior, and image ordering.
7. **Write normalization and validation preparation.** At the scenario API-to-DB boundary, copy stable descriptor fields or clear `presigned_uri` on a copy before invoking the existing conversion; avoid mutating the incoming request. Use the existing profile-specific quiz-media key rules for scenario references. Add focused coverage for descriptor round-trip, transient-field exclusion, and invalid profile/prefix handling.
8. **Schema rollout preparation.** Plan additive schema application before deploying backend code using the generated column. Verify schema diff/generation output and document staging/production ordering; schema deployment is not part of this plan-edit task.

After preparation, integrate fresh image signing in `ListTrainingScenarios`, verify create/update revision persistence and expanded module/lesson reads, and confirm image-only changes retain the existing VA revision. Run existing quiz media regressions alongside focused scenario tests. The upload and commit implementation remains shared and unchanged.

## Behaviors and limits to preserve or handle deliberately

- Quiz upload/commit accepts PNG, JPEG, WebP; request annotations constrain each batch to 1–20 entries. Upload and download TTL defaults are 15 minutes. The inspected backend does not enforce a byte-size cap or image decoding; do not describe frontend limits as backend guarantees.
- Commit verifies staged object metadata with `HeadObject`, copies to the permanent key, and best-effort deletes the temporary object. It does not persist the descriptor in a scenario/question row; content save is separate.
- Existing commit is sequential and non-atomic. A later failure can leave earlier objects committed, with no returned descriptors. Retry after a completed commit can fail because the temporary object is gone. Director works around partial batches by committing one image per call; this still does not establish idempotence after response loss. The reused RPC retains this constraint; separately scope any idempotent-finalization improvement.
- Persist keys/metadata only. Quiz proto marks `presigned_uri` output-only, but its SQL wrapper marshals the provided message and its converter does not strip that field. Director drops it from form state. For the new path, explicitly normalize persistent metadata rather than relying on annotations alone.
- Download URLs must be refreshable during a long editor/voice session. The read API supplies fresh values; frontend caching and image-error refresh are consumer work.
- Removing an image from a new scenario revision should not delete the permanent object needed by an older revision. Do not add destructive object cleanup as part of attachment removal.
- Scenario reads currently return the latest revision, unlike quiz Get's explicit revision name. Preserve current scenario semantics; historical assignment snapshot redesign is a separate issue.

## Focused validation plan

- Existing upload regression coverage (reuse the current suite; do not duplicate it for scenarios): valid PNG/JPEG/WebP, filename separators, unsupported MIME, malformed parent, 0/21 entries through normal validation, bucket/config errors, signing errors, correct temporary/permanent key relation and TTL.
- Existing commit regression coverage (reuse the current suite): correct profile prefix, missing staged object, unsupported object metadata MIME, copy failure, nonfatal temp-delete failure, ordered descriptors derived from object metadata, documented partial/retry behavior.
- Persistence: create with images; add/remove/replace; empty/NULL legacy images; unchanged earlier revision; `presigned_uri` absent from stored JSONB; invalid profile/prefix rejected.
- Reads: direct scenario list and nested module/lesson reads return descriptors with fresh URIs; no storage calls for image-free content; isolation by customer/profile; signing failure behavior.
- VA behavior: image-only edit preserves VA revision and makes no VA RPC; text changes still create the expected VA revisions.
- Authorization: author roles may upload/commit; AGENT can read but cannot upload/commit, matching existing quiz/scenario method annotations.
- Regression: existing quiz upload/commit/Get plus scenario/module/lesson tests. Proto lint/build/breaking checks, model/converter regeneration, dependency/build metadata updates, and scoped backend tests belong to implementation.
- Staging follow-up: author upload → commit → save scenario/module → reload lesson as trainee → refresh expired URI → remove/replace while preserving prior revision metadata.

## Dependencies, validation, and rollout

- Publish additive proto first; backend currently consumes `cresta-proto v2.25.16` and will need a release containing the new contract.
- Apply the additive schema before new backend model use. Repository workflow: `mage RegenerateGorm apiserver`; schema sync triggers automatically for staging after merge, with production workflow run separately. Coordinate actual deployment ordering.
- Frontend integration remains outside this backend investigation, but reuse of the quiz-named RPCs/types/storage keys, count/size expectations, replacement semantics, and URI refresh must be communicated to its owner.
- Implementation verification: proto lint/API lint/dependency and scoped Buf compatibility checks passed; Gazelle and the affected proto Bazel build passed. GORM and goverter generation passed. Local Go scenario create/update, quiz upload/commit/Get, key validation, and converter tests passed against the generated proto change. Both focused Bazel test targets passed using the proto repository override. No staging/production DB, storage upload, live RPC, or deployment validation performed.

## Next action

Review the two draft PRs, land/release the additive proto when ready, update the backend dependency metadata, and integrate signed scenario reads. Coordinate schema-before-code rollout and frontend use of the shared RPCs; then perform staging end-to-end validation.

## Timeline

- 2026-09-18 — Investigated live ticket and current source; confirmed quiz precedent, scenario revision/read integration, and retry/URI boundaries. Evidence: [session](../sessions/2026-09-18/codex-convi-7707-scenario-images.md), [daily log](../log/2026-09-18.md).
- 2026-09-18 — User agreed to reuse `QuizQuestionImage`, `QuizQuestionImageListSQL`, and existing converters, accepting their quiz-specific names across the coupled layers. Revised the plan to remove duplicate scenario image types and reuse download signing. Implementation remains pending.
- 2026-09-18 — User agreed to reuse `GenerateQuizMediaUploadUrls` and `CommitQuizMedia` as well. Removed proposed scenario media RPCs and key prefixes; added an ordered definitions/preparation checklist covering protobuf, shared docs, generated artifacts/dependencies, SQL, DB models, converters, normalization, and rollout ordering.
- 2026-09-18 — Implemented preparation in dedicated worktrees and opened draft [proto#9918](https://github.com/cresta/cresta-proto/pull/9918) / [backend#32494](https://github.com/cresta/go-servers/pull/32494). Schema/model/converter generation and local focused tests passed; published dependency bump and signed reads remain follow-ups. Evidence: [implementation session](../sessions/2026-09-18/codex-convi-7707-preparation.md).
