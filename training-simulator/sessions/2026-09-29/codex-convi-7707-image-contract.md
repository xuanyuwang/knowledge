# CONVI-7707 reduced scenario image proto contract

- Date: 2026-09-29, America/Toronto
- Primary source repo: `/Users/xuanyu.wang/repos/cresta-proto`
- Worktree/branch: `/Users/xuanyu.wang/repos/cresta-proto-convi-7707-image-contract`, `xw/convi-7707-scenario-image-contract`
- Commits/PR: `0c231aed80`, `52f2c4ddd8`; https://github.com/cresta/cresta-proto/pull/10057
- Ticket: https://linear.app/cresta/issue/CONVI-7707/be-of-add-image-to-scenario-context

## Decision and execution

The prior proto#9918 was closed. Its Training Simulator upload/commit RPCs are superseded by merged generic S3FileService contract #9995. Kept only `TrainingModuleImage`, `TrainingModuleImageList`, and `TrainingScenario.images = 14`. The image descriptor stores stable object key/display/MIME/size data and carries an output-only transient download URL. Legacy quiz protobuf remains unchanged. Created #10057 as draft, then marked it ready for review after validation and CI checks.

Scoped Buf lint and breaking comparison against current main passed. `bazel build //cresta/v1/trainingsimulator:all` passed. CI's initial Bazel generation check found one BUILD indentation difference; ran repo Gazelle, committed the exact formatting fix at `52f2c4ddd8`, and pushed. A subsequent GitHub check snapshot showed no failing jobs, with Codeowner check still pending. Locally generated Go files used to validate the dependent backend were removed from this clean worktree after validation; generated artifacts are excluded from the PR.

## Follow-up

Merge and release #10057, then bump the scenario backend draft #33045 to the published version and run ordinary Bazel checks. No schema, runtime, or staging change was made in this source repo.

Credential use: GitHub HTTPS authentication only. No AWS, Okta, Azure or SSH credentials were selected, read or used.
