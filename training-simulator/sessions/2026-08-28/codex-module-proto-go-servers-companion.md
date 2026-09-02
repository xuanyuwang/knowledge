# CONVI-7601 go-servers companion verification

Source repo: `/Users/xuanyu.wang/repos/go-servers`

Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7601-companion`

Branch: `convi-7601-module-reporting-proto-companion`

## Objective

Determine whether cresta-proto [#9676](https://github.com/cresta/cresta-proto/pull/9676) needs a new go-servers companion PR, and cross-link the appropriate change.

## Observed behavior

- The proto CI replaces `github.com/cresta/cresta-proto/v2` with the synthetic PR merge checkout and runs `go generate ./...` in each affected go-servers module.
- go-servers has no Training Simulator SQL-schema proto mirror for the changed service or new module-statistics proto.
- Reproducing generation against PR #9676's synthetic merge ref produced no Training Simulator converter or generated-file diff.
- Generation failed on `Moment_SCREEN_ACTIVITY` and `SessionConversationMapping.ConversationChannel`, exactly the unrelated compatibility failures addressed by go-servers [#31683](https://github.com/cresta/go-servers/pull/31683).
- The only generated diff before failure concerned unrelated workflow-automation feature fields.

## Confirmed decision

Do not create an empty or duplicate CONVI-7601 go-servers PR. Treat #31683 as the compatibility companion for the failing generation check, link it from #9676, add a backlink from #31683, and acknowledge the proto PR checkbox.

## Actions

- Added #31683 under Related changes in #9676 and documented why the failure is unrelated to module reporting.
- Checked the readiness acknowledgement in #9676.
- Added a backlink comment on #31683: https://github.com/cresta/go-servers/pull/31683#issuecomment-5453844496
- Preserved the unfinished module-reporting implementation worktree unchanged.

## Validation

- Clean reproduction worktree returned to a clean Git state after the diagnostic run.
- No new go-servers source change or PR was created because there is no module-specific companion diff.

## PR rebase

- Fetched current cresta-proto `origin/main` at `8453649516` after a subsequent CI run continued to include unrelated changes.
- Rebased #9676 onto that head and force-pushed `ce8545efb7`; its reviewed patch remained identical: three files, 94 insertions, and five deletions.
- Rebuilt stacked #9677 directly on the updated #9676 head and force-pushed `37608b3499`. The old merge topology was squashed into one lesson commit while preserving the exact reviewed three-file, 64-line lesson diff.
- Targeted `buf lint --path cresta/v1/trainingsimulator`, `buf build --path cresta/v1/trainingsimulator`, and `bazel build //cresta/v1/trainingsimulator:all` passed.
- New GitHub CI runs started for both PRs after the force-pushes.

## Root cause of unrelated CI failures

- The rebased #9676 job `98896102488` selected the Training Simulator Go package correctly from only `module_stats.proto` and `training_simulator_service.proto`.
- The workflow then grepped go-servers for imports of that package, found consumers in the repository's root Go module, and reduced the affected module directory to `.`.
- It replaced the root module's cresta-proto dependency with the complete #9676 merge checkout and ran `go generate ./...`, which executes every generator in the root module rather than only Training Simulator generators or importing packages.
- That broad run reached unrelated moment and Teleport goverter directives. They failed because current cresta-proto contains `Moment_SCREEN_ACTIVITY` and `SessionConversationMapping.ConversationChannel`, while current go-servers main lacks the corresponding DB enum and converter ignore. These are the exact #31683 changes.
- Therefore rebasing cannot remove these failures: the incompatible fields are already on current cresta-proto main. The durable fixes are landing #31683 or narrowing the cross-repository workflow's generation scope.
