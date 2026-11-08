# CONVI-7852: CI failures

- Date: 2026-11-05
- Source repo: /Users/xuanyu.wang/repos/go-servers
- Worktree: /Users/xuanyu.wang/repos/go-servers-convi-7852-analytics-roles
- Local branch: xw/convi-7852-analytics-review
- Published branch: xw/convi-7852-analytics-membership-roles
- PR: https://github.com/cresta/go-servers/pull/35094
- Head: ffbd308b566cd53c6b06452a2f70b4e9eb3166d5

## Diagnosis

Three failed checks, two independent download failures:

- Lint Bazel BUILD files, run 42045923836/job 127308704982: fetching bazelbuild/rules_pkg release 1.0.1 returned HTTP 504. Gazelle analysis never completed; no BUILD-file discrepancy was reported.
- Detect affected services, run 42045925206/job 127308715399: fetching bazelbuild/rules_cc release 0.1.5 returned HTTP 504. Bazel reverse dependency resolution could not run.
- required, run 42045925206/job 127309940915: failed because detect failed; downstream services were skipped.

Raw logs: /tmp/convi-7852-build-lint-failed.log, /tmp/convi-7852-detection-failed.log, /tmp/convi-7852-required-failed.log.

## Action

Requested failed-job reruns for both workflow runs, preserving the published head. No source edits warranted by these logs. Waiting for fresh CI results, including downstream jobs previously skipped.

## Credentials

GitHub CLI authenticated itself. No AWS/Okta/Azure/SSH credential or profile explicitly selected or read.

## Rerun progress

Attempt 2: BUILD lint and affected-service detection both passed on the unchanged head. Insights build/lint/five test shards now running; previously skipped service jobs are being exercised.

Insights Go lint, Bazel build, and shards 2/3/4/5 passed. Shard 1 remains running; all completed checks are green.

## Final result

Both failed workflow reruns completed successfully on unchanged head ffbd308b566cd53c6b06452a2f70b4e9eb3166d5:

- Other Lints attempt 2: success, including BUILD lint.
- CI attempt 2: success, including affected-service detection, Insights Go lint, Bazel build, all five test shards, and aggregate required.

Links: https://github.com/cresta/go-servers/actions/runs/42045923836 and https://github.com/cresta/go-servers/actions/runs/42045925206 .

The root cause was transient HTTP 504 responses for GitHub-hosted Bazel release assets; reruns recovered. No source changes, bypasses, new commits, or merge performed. Codeowner approval remains separate from CI.
