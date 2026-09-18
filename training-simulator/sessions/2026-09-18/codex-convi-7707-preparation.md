# CONVI-7707 definitions and persistence preparation

- Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree/branch: `/Users/xuanyu.wang/repos/go-servers-convi-7707`, `xw/convi-7707-scenario-images`, based on `1ae17fe508`.
- Related proto worktree: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707`, same branch name, based on `ac81eebac8`.
- User authorized implementation in new worktrees and draft PR creation.
- Scope: agreed definitions/preparation checklist: reuse quiz types and RPCs; add scenario proto field, SQL column, generated models/converters, write normalization/validation, tests and rollout notes. Runtime signed-read enrichment remains the next implementation step.
- Draft backend will depend on the additive proto contract; no main-branch merge, release, or schema deployment is authorized by this draft-PR task.
- Existing knowledge changes will be preserved. Credentials used so far: existing GitHub HTTPS and Linear connector authentication; no AWS/Okta/Azure/SSH credentials used.

## Implemented and published

- Draft proto PR: https://github.com/cresta/cresta-proto/pull/9918 (`01efcfb856aa11ff45fa2d33c6917312ddfc3da6`). Adds `TrainingScenario.images = 14` using `QuizQuestionImage` and broadens existing message/RPC documentation.
- Draft backend PR: https://github.com/cresta/go-servers/pull/32494 (`7783ba65581637404e80282fd27c7d849250593c`). Adds nullable scenario JSONB with the existing SQL type; regenerated model and goverter mappings; clones the scenario before clearing image download URIs; validates profile-bound quiz-media keys before VA/DB side effects.
- New tests cover stable metadata and SQL serialization, NULL/empty images, request immutability, invalid key shapes/profiles, handler validation, and image-only new revision persistence with prior revision/VA preserved.
- Both remote draft flags and head SHAs were verified after creation. Each PR uses the repository template and a CONVI-7707 title. Generated proto code stays local and is excluded from the proto PR.

## Dependency and rollout boundary

The additive proto is still in draft, so it is not published as a module release. Backend `go.mod` / `go.sum` / `deps.bzl` remain on the checked-in dependency and need a coordinated bump after #9918 is released. Ordinary backend CI cannot compile the new field until that dependency is updated. Local validation used a temporary module file (`GOFLAGS='-modfile=/tmp/convi-7707-go.mod -mod=mod'`) replacing only cresta-proto with `/Users/xuanyu.wang/repos/cresta-proto-convi-7707`; Bazel validation uses `--override_repository=com_github_cresta_cresta_proto_v2=/Users/xuanyu.wang/repos/cresta-proto-convi-7707`. No local override is committed.

Apply the additive schema before new backend code. No backfill or object deletion is needed. This task did not merge/release proto, change staging/production schema, or deploy code. Signed reads in `ListTrainingScenarios` and frontend integration remain the next implementation phase.

## Validation evidence

- `mage -v lint`, `mage -v apiLint`, `mage checkAllProtoDependencies`: passed.
- Scoped Buf lint/build/breaking checks: passed. `bazel run //:gazelle` and `bazel build //cresta/v1/trainingsimulator:all`: passed.
- Local generated proto package copied from the successful Bazel output, with normal `cresta_lite` tags injected using the repository script.
- `mage RegenerateGorm apiserver`: passed, including generated model/DAO Bazel builds. Only scenario model changes retained; normalized that generated file to non-executable permissions.
- `go generate ./apiserver/internal/trainingsimulator/converter` using the temporary proto replacement: passed; generated mappings reuse both existing quiz image converters.
- Local Go selected scenario creation/update, quiz upload/commit/Get, image-key validation suites: passed (50.401s). Converter suite initially exposed an incomplete expected timestamp fixture; fixed the fixture, then the suite passed (0.896s). No production behavior change was needed for that test failure.
- Backend Gazelle and diff whitespace checks passed. The first queued Bazel run used metadata preceding the test fixture's timestamp import and failed a strict dependency check; the regenerated, committed BUILD now includes it. Final rerun result recorded below.

## Environment and authentication

- GitHub operations and private Git dependencies use existing HTTPS authentication with command-local Git URL overrides; direct Git SSH is disabled for the commands. Linear used the existing connector.
- Started the existing local Colima VM for the repository's Docker-based lint/schema generation. Proto generator image was already cached; no ECR login or AWS CLI action was performed. Schema generation used a disposable local PostgreSQL container.
- Initial formatter package installation consulted the preconfigured CodeArtifact package index and returned HTTP 401; reran with isolated pip against public PyPI. No credential values were printed or persisted by this task.
- The source main checkouts were preserved; knowledge changes were made directly in its main checkout and left uncommitted alongside pre-existing work.

## Final verification

- Focused Bazel rerun passed: `converter:converter_test` (1.2s) and `trainingsimulator:trainingsimulator_test` (three shards, maximum 14.8s), selecting new image tests, scenario create/update suites, and existing quiz media/Get suites. Both targets passed with the local proto override; total command time 50.558s.
- Backend PR test-plan result updated to passed. Proto PR links the backend draft. Both remain draft at the recorded commits.
- Backend worktree is clean after commit/push. Proto worktree retains only locally generated Go changes for dependent validation; its PR includes handwritten proto changes only.
- All edited knowledge documents pass whitespace/local-link validation. Main-checkout source changes and unrelated knowledge work were preserved.
