# CONVI-7707 generalized training media infrastructure

- Primary source: `/Users/xuanyu.wang/repos/go-servers-convi-7707`, branch `xw/convi-7707-scenario-images`.
- Contract dependency: draft cresta-proto#9918 in `/Users/xuanyu.wang/repos/cresta-proto-convi-7707`.
- User superseded the September 18 reuse decision: introduce general names for image protobuf, SQL wrapper, converters and media RPCs; use them for scenarios and defer legacy quiz contract migration.
- Implementing `TrainingImage`, `TrainingImageList`, `GenerateTrainingMediaUploadUrls`, `CommitTrainingMedia`, and the `training-media/` namespace. Legacy quiz types, routes, keys and TTL settings stay compatible through thin adapters to shared internals.
- Existing draft backend PR: https://github.com/cresta/go-servers/pull/32494. Published proto dependency remains a merge prerequisite.
- Authentication: existing GitHub HTTPS only; no AWS, Okta, Azure or SSH credentials used.

## Published revision

- Proto draft #9918: `d6a39b908c905cbcf90f0f4a7527a01fc86756ff`, title `CONVI-7707 Add generalized training image and media contracts`.
- Backend draft #32494: `4fc51213f8cc5da9c96651a227296672d248c6ce`, title `CONVI-7707 Add shared training media infrastructure for scenarios`.
- Verified both remote heads, OPEN states and draft flags. Updated PR bodies after checking remote updatedAt plus body hash; preserved the existing proto PR automated go-generate warning/acknowledgement appendix byte-for-byte. No merge-readiness acknowledgement was checked.
- Plan, README, project state and daily log reflect the September 21 direction. The September 18 records remain historical evidence.

## Validation and review

- Refreshed GitHub main: proto `da4dd1043b4e52580de73a2cd64415c40ee96ac4`, backend `f37e6fe722d4d87cb394443e1ff177342b8a735b`. No conflicting Training Simulator source changes; proto package BUILD load ordering is the only relevant baseline difference.
- Proto: `mage -v lint`, `mage -v apiLint`, `mage checkAllProtoDependencies`, Gazelle, and `bazel build //cresta/v1/trainingsimulator:all` passed. All 940 API-linter file results had no problems.
- Buf: built package-scoped descriptor images for the current tree and an archive of current main, then `buf breaking current.binpb --against base.binpb` passed. Direct local worktree Git-ref comparison resolved the prior draft instead of main, and a direct directory/path comparison hit a Buf path-context bug; neither was treated as compatibility evidence.
- GORM regenerated from a disposable local PostgreSQL container; model/DAO builds passed with the local proto repository override. Scenario model executable mode was normalized to 644. Goverter generation passed with `/tmp/convi-7707-go.mod`.
- Initial focused Go run passed (converter 0.957s, Training Simulator 40.493s). Final focused Bazel run passed both targets: converter 1.2s and Training Simulator three shards (maximum 28.3s; total command 34.943s). It selected the new media suite, converters/key checks, existing quiz upload/commit/Get, scenario create/update/list, and expanded module/lesson listing. A compile error in a set-membership method name was corrected before this final successful run.
- Coverage includes generic namespace upload paths/TTLs, commit metadata and cleanup/copy failures, rejecting foreign profiles and cross-namespace keys, fresh scenario signed reads and signing failures, no persisted URLs, request immutability, legacy JSON shape, NULL/empty images, and image-only scenario revision/VA preservation.
- Self-review checked legacy response field mapping, namespace boundaries, retained quiz TTL/route/types, generated dependencies, signed-read inheritance, and URL stripping in the generalized SQL converter. No generated proto files or local dependency overrides are committed.

## Environment and remaining work

- Backend worktree is clean. Proto worktree retains local generated Go files for dependent validation; they are intentionally excluded from its draft PR. Knowledge edits remain uncommitted alongside existing user work.
- Used existing GitHub HTTPS authentication and the cached local Docker codegen image. Git SSH execution was disabled; an existing URL rewrite initially caused fetch/codegen failures and was overridden per command to use HTTPS. No AWS, Okta, Azure or SSH credentials were used.
- Backend still declares released proto v2.25.16. Publish the additive contract, bump go.mod/go.sum/deps.bzl and client artifacts, coordinate schema-before-code deployment, and validate the full flow in staging. No release, deployment, schema application to shared environments, or staging/production RPC/storage validation was performed.
- Legacy quiz protobuf/client migration remains separate. Generic SQL JSON is compatible with legacy metadata, but existing quiz storage keys need an explicit migration/read-compatibility policy; published field types must not be swapped in place.

## Module image naming follow-up

- User accepted `TrainingModuleImage` / `TrainingModuleImageList` in `training_module_image.proto`. Defining them in `training_module.proto` would create a scenario → module → scenario import cycle and a quiz → module → quiz cycle during future migration.
- Renamed the unpublished types and source file, updated scenario and commit-response references, and regenerated the dependent SQL wrapper/model and converter mappings. Preserved the five legacy image fields and field numbers, list wrapper shape, RPC names, storage paths, and legacy quiz contract.
- Removed obsolete locally generated `training_image*.go` files from the previous draft; new generated artifacts remain local and excluded from the proto PR.
- Rename validation: proto lint/API lint/dependency checks, scoped Buf compatibility against main, proto Bazel build, GORM model/DAO generation, and goverter generation passed. Focused backend regression results and final heads follow after publication.
- Rename follow-up published: proto `153723fe64` and backend `96343434f3`. Both focused Bazel targets passed (converter 1.1s, Training Simulator three shards with maximum 29.0s; command total 76.105s). Both draft PR descriptions were patched with current names and verified after guarding remote updatedAt/body hash; existing automated text was preserved. Backend still needs the published proto dependency. Authentication remained existing GitHub HTTPS only.

## Existing storage follow-up

- Clarified that `trainingMediaPrefix` was an object-key prefix, not a bucket. Bucket lookup has always used `getCustomerPrivateBucket` → profile configuration → `GetS3PrivateBucket`; no external storage resource was created.
- User requested keeping existing external storage. Both new and legacy APIs now use `quiz-media/` permanent keys and `tmp/quiz-media/` staging keys, declared as `trainingMediaPrefix` / `trainingMediaTempPrefix`. The previous draft derived temporary keys with `"tmp/" + permanentKey`; staging was never removed. Explicit constants make both locations visible again.
- Removed the per-endpoint namespace parameter and redundant private commit wrapper. Generic commit and scenario save/read validation accept existing profile-bound quiz-media keys. Tests use the actual shared paths and continue rejecting temporary/foreign-profile/unrelated-prefix keys.
- Published backend `68aacebf7f` to draft #32494 and patched its description while preserving existing content. Focused Bazel converter and Training Simulator targets passed (1.4s / maximum shard 22.1s). Only existing GitHub HTTPS authentication was used; no AWS credentials or shared-environment storage calls.

## Proposed review decomposition

- User asked how to split the backend PR into two or three smaller PRs. Verified live #32494 is +801/-175 across 20 files, head `68aacebf7f`, base `1ae17fe508`.
- Recommended three stacked changes: persistence foundation (proto bump/schema/converters/save validation); generalized upload/commit infrastructure and legacy adapters; scenario signed reads. Split mixed training_media_test.go by behavior and keep each PR's generated BUILD metadata. Verified ServiceImpl embeds the unimplemented generated server, so PR1 can consume the released contract before PR2 implements new methods.
- Two-PR alternative combines generalized media endpoints with scenario signing. Proposal only: no source edits or GitHub mutations made. Existing GitHub HTTPS authentication used for read-only PR inspection.

## Executing the approved PR split

- User authorized the three-PR stack. Preserved the complete implementation at local branch `xw/convi-7707-before-split` (`68aacebf7f`). Keep #32494 on `xw/convi-7707-scenario-images` for persistence; create separate media and signed-read worktrees/branches stacked on it.
- Preserve branch history using forward commits rather than force-pushing. The reviewed Files changed diffs will each contain their own scope. Proto release/dependency bump remains a merge prerequisite; no release is performed in this split.
- Split the mixed training_media_test.go by behavior so each PR carries its own tests; compare the completed stack against the saved implementation to ensure production code is unchanged.
- Isolated local heads: persistence `fdd3a31db1` (+378/-28, 13 files); media `63be8a2cd8` (+333/-150, 7 files); signed reads `98c5fa63d2` (+103/-0, 4 files). New worktrees: `/Users/xuanyu.wang/repos/go-servers-convi-7707-media` and `/Users/xuanyu.wang/repos/go-servers-convi-7707-reads`.
- Production equivalence audit passed: all 13 changed production/generated files match `68aacebf7f` byte-for-byte at the stack tip. Mixed test bodies were preserved exactly across training_media_test.go and scenario_image_download_test.go; the only combined-tree changes are that split and the extra generated BUILD test-file entry.
- Persistence Bazel checks passed (converter 0.6s; Training Simulator max shard 17.9s). Media Go checks passed (27.536s). Complete-stack Go checks passed (converter 1.701s, Training Simulator 75.389s) and Bazel checks passed (2.3s / max shard 30.3s).
- The new media worktree's cold Bazel build was stopped after toolchain initialization to avoid redundant compilation; final Bazel verification uses exact detached stack commits in the existing warm checkout. Gazelle ran through the repository target for each scope, including `-repo_root` for the new reads worktree. No implementation/configuration workaround or test exclusion was introduced.
- Final media-only Bazel run passed at `63be8a2cd8` (max shard 15.8s; command 19.573s). Original worktree restored to persistence branch after detached-commit validation; all three source worktrees are clean.
- Atomically pushed all three branches via GitHub HTTPS. Published and verified draft #32494 (base main, head fdd3a31db1), #32597 (base xw/convi-7707-scenario-images, head 63be8a2cd8), and #32598 (base xw/convi-7707-media-apis, head 98c5fa63d2). Descriptions include scope, tests, dependency caveats, and stack links. Updates guarded updatedAt/body hash and preserved appended automation content.
- Kept existing PR history and a local backup; no force push, release, merge, schema deployment or external storage changes. Only existing GitHub HTTPS authentication used, with Git SSH disabled. Proto draft #9918 is unchanged by this split.
