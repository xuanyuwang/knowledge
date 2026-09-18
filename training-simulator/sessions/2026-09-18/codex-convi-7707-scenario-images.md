# CONVI-7707: Scenario context image backend investigation

- Date: 2026-09-18 (America/Toronto)
- Primary domain/subdomain: training-simulator / training-content
- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Source context: local `main` checkout, inspecting `origin/main` objects without changing the checkout; live GitHub main SHA confirmed as `2995bc56b2b487fb0f4442d425ed716149266798`.
- Related contract repo: `/Users/xuanyu.wang/repos/cresta-proto`.
- Scope: investigation and implementation recommendation only; mirror quiz image support for scenario content.

## Ticket evidence

- CONVI-7707 is In Progress, assigned to Xuanyu Wang; its description is empty and it has no comments.
- Parent CONVI-7626: “Similar to the add media capability in quiz questions, support adding image stored at the scenario level to be referenced during a voice simulation.” No comments.
- Product interpretation: scenario-attached media available during simulation; no confirmed requirement to feed image pixels to the Customer AI model.

## Outcome

Quiz's staged upload/commit + persisted descriptor + signed read pattern fits scenario context images. The recommended scope is additive proto, scenario JSONB persistence, reuse of existing quiz upload/commit handlers, and enrichment in the existing scenario list. Full implementation and validation plan: [canonical work item](../../work-items/CONVI-7707.md).

## Current source snapshots

- go-servers: `2995bc56b2b487fb0f4442d425ed716149266798` (same as live GitHub main).
- cresta-proto: `ac81eebac8390e7b5b07057e24b00863d9cb7191` (refreshed and checked against live GitHub main).
- Director, consumer inspection only: `ec8886f59746a1fd1deedd0216df0d6af6e70472` (refreshed and checked against live GitHub main).
- Read source with `git show origin/main:<path>` because local main checkouts lag their remote refs. Links below identify exact inspected commits rather than potentially older working-tree lines.

## Source map


- [Quiz upload validation and signing](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/action_generate_quiz_media_upload_urls.go#L30)
- [Quiz commit metadata/copy/delete](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/action_commit_quiz_media.go#L14)
- [Private bucket and profile-key validation](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/storage_helper.go#L13)
- [Quiz read-time download signing](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/action_get_quiz_template.go#L103)
- [Quiz descriptor and SQL wrapper](https://github.com/cresta/cresta-proto/blob/ac81eebac8390e7b5b07057e24b00863d9cb7191/cresta/v1/trainingsimulator/quiz_template.proto#L146)
- [Quiz media RPC roles and routing](https://github.com/cresta/cresta-proto/blob/ac81eebac8390e7b5b07057e24b00863d9cb7191/cresta/v1/trainingsimulator/training_simulator_service.proto#L391)
- [Scenario contract; field 14 available at snapshot](https://github.com/cresta/cresta-proto/blob/ac81eebac8390e7b5b07057e24b00863d9cb7191/cresta/v1/trainingsimulator/training_scenario.proto#L14)
- [Scenario and quiz schema](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/sql-schema/director/director-schema.sql#L54)
- [Scenario revision insertion](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/action_batch_update_training_scenarios.go#L96)
- [VA change predicate](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/action_batch_update_training_scenarios.go#L256)
- [Text-only scenario system prompt](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/action_batch_create_training_scenarios.go#L195)
- [Scenario read conversion](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/action_list_training_scenarios.go#L56)
- [Nested module scenario read](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/action_list_training_modules.go#L75)
- [Nested lesson module read](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/action_list_training_lessons.go#L123)
- [Quiz image persistence wrapper](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/internal/trainingsimulator/converter/converter.go#L537)
- [SQL serializer marshals full message](https://github.com/cresta/cresta-proto/blob/ac81eebac8390e7b5b07057e24b00863d9cb7191/gen/go/cresta/v1/trainingsimulator/quiz_template.pb.sql.go#L370)
- [Single-image commit consumer](https://github.com/cresta/director/blob/ec8886f59746a1fd1deedd0216df0d6af6e70472/packages/director-app/src/hooks/training-simulator/useCommitQuizMedia.ts#L9)
- [Transient URL excluded from form](https://github.com/cresta/director/blob/ec8886f59746a1fd1deedd0216df0d6af6e70472/packages/director-app/src/features/training-simulator/create-module/form/mappers.ts#L65)
- [Voice simulation lesson/scenario loading](https://github.com/cresta/director/blob/ec8886f59746a1fd1deedd0216df0d6af6e70472/packages/director-app/src/features/training-simulator/simulation/api/useTrainingLesson.ts#L10)
- [Schema generation/deployment instructions](https://github.com/cresta/go-servers/blob/2995bc56b2b487fb0f4442d425ed716149266798/apiserver/README.md#L141)

## Detailed conclusions

- No images field exists on the current scenario proto or DB model. Quiz questions persist their image list in JSONB. Add the scenario field/column and regenerate mappings while reusing the existing image message, SQL wrapper, and converters.
- Scenario create/update wrappers are already reused by module save. ListScenarios is already reused by expanded module and lesson reads; Director's voice simulation uses this expanded lesson response.
- Media upload/commit only touches object storage. The permanent object descriptor must subsequently be saved in the scenario revision. No scenario resource ID is needed to stage media before initial save.
- Image-only changes do not belong in the existing VA change predicate under the trainee-reference interpretation. Existing VA inputs are text/state/title; image interpretation by the model is not part of the established requirement.
- Upload/commit methods have author roles; list/get methods also allow AGENT. Reuse this authorization split.
- Metadata is validated from object-store headers, not image byte decoding. The inspected handlers have no backend size check; 1–20 is a per-request annotation constraint, not a per-scenario limit.
- Current quiz commit has partial-success and retry limitations. Director explicitly uses one image per call to prevent the multi-image prefix-commit failure. A lost response after success still needs an idempotence strategy if stronger guarantees are required.
- Output-only annotation alone does not remove presigned URLs from the SQL marshalled message. New persistence conversion should strip transient fields deliberately.
- No delete RPC or new destructive object cleanup is required; older revisions may continue to reference old images.

## Follow-up decision: reuse quiz image types

The user agreed to reuse `QuizQuestionImage` after discussing its coupling to the generated SQL wrapper and converters. The plan now uses `TrainingScenario.images` with that existing element type, `QuizQuestionImageListSQL` for scenario JSONB, and the existing slice/SQL converters and download-signing helper. Quiz-specific naming is accepted for this feature; no duplicate scenario image types or client-type rename is planned. Scenario persistence will still strip transient `presigned_uri` before conversion. This supersedes the original recommendation for `TrainingScenarioImage` and a dedicated list wrapper.

## Follow-up decision: reuse quiz media RPCs

The user also agreed to reuse `GenerateQuizMediaUploadUrls` and `CommitQuizMedia`. They operate on a profile and uploaded objects, without quiz/question IDs, and already return the selected shared image descriptor. Keep existing `quizMedia` routes and `quiz-media/` / `tmp/quiz-media/` keys; no scenario upload/commit handlers or storage prefix will be added. The canonical plan now includes an ordered definitions/preparation checklist. Verified that `quiz_template.proto` has no scenario import and that both image converter extensions are already registered in the shared converter interface. This was a documentation update only; implementation and tests remain pending.

## Verification and actions

- Read live Linear issue/parent/comments using the dedicated Linear connector; no issue mutations or messages.
- Verified source snapshots using GitHub CLI; fetched proto and Director refs through HTTPS. Initial refresh attempts were stopped by an explicit disabled SSH command; no SSH credential was used. Existing Git URL rewriting explained the failed attempts.
- Authentication used: existing Linear connector and GitHub CLI/HTTPS authentication. No AWS, Okta, Azure, or SSH credentials were read or used.
- Read related test suites; did not execute tests or claim deployed behavior. No DB connection, production API call, S3 action, product-code edit, commit, push, or PR creation.
- Preserved pre-existing knowledge edits. Created work item/session/daily log and added small domain index references.
- Validation: scoped documentation whitespace checks and local-link checks recorded at handoff.
