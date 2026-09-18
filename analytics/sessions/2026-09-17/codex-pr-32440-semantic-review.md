# PR #32440 semantic review

Date: 2026-09-17 (America/Toronto)
Primary source repo: `/Users/xuanyu.wang/repos/go-servers`
Branch/worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7706`, `xw/convi-7706-conversation-stats-value-or-missing`
Reviewed head: `e7704f66378c40466620eacd4535cfede19e942b`
PR base: `6e2648ebeddfd56c8484ea2766d6e5c01b4f0dce`

## Scope

Review-only comparison of PR #32440 with the semantic findings and fixes from PR #31335. Check boolean grouping, parser rejection, numeric boundaries and invalid values, latest/stale behavior, annotation time scope, CTE aliases, binding order, aggregation fanout, and shared-helper callers. No product fixes or external review actions authorized.

## Initial evidence

- Local source head matches GitHub head; source checkout initially clean.
- GitHub has no submitted reviews or inline review comments. Required CI fails on three Bazel test workflow shards; investigate failure logs before attribution.
- Existing analytics domain and CONVI-7706 work item are the canonical home.
- Historical findings are hypotheses to revalidate against this head, not automatically new findings.

## Validation and findings

Completed: no actionable correctness findings. [Full review](../../deliverables/convi-7706-pr-32440-semantic-review.md) records the migrated #31335 checks, coverage, CI diagnosis, and limits.

- Focused existing tests, nine additional invalid/unsupported-shape cases, and ES overlap contract test pass.
- 96 complete generated queries yielded 176/176 passing local ClickHouse checks, including alternate time axes, multiple groups, latest/stale transitions, numeric/boolean/empty values, duplicate protection and grouped aggregates.
- Required CI remains red. The ES `TestRetrieveConversationMessages/BasicRequest` EOF reproduces on both PR head and exact parent, using a Go overlay to reverse all three PR-changed files. Other failing CI targets were inspected but not individually base-tested.
- Temporary Go generator initially had a misspelled frequency enum; fixed only the review harness. No product implementation was modified.
- Preserved [synthetic reproduction bundle](pr-32440-review-evidence/README.md). Product checkout restored clean; no external review actions.
- Credentials: GitHub CLI HTTPS authentication and local embedded-PostgreSQL test account. No AWS, Okta, Azure, SSH, or production database credentials used. ClickHouse local requires none.
