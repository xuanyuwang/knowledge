# PR #32440 review evidence

Reviewed head `e7704f66378c40466620eacd4535cfede19e942b`, exact parent/base `6e2648ebeddfd56c8484ea2766d6e5c01b4f0dce`. All data is synthetic.

- `generated-queries.json`: 96 complete SQL queries from production parsers/builders.
- `pr32440_review_test.go`: temporary generator and nine rejection cases, removed from the source checkout after review. Its export path is `/tmp/pr32440-review`; create that directory before regenerating.
- `run_runtime.py`: independent fixture membership and aggregate oracle; executes the saved SQL using `clickhouse local` and writes `fixture.sql` / `runtime-results.json`.
- `runtime-results.json`: all 176 checks pass on ClickHouse 26.8.2.7.
- `focused-tests.log`, `generator.log`, `es-contract.log`: passing test summaries.
- `ci-base-comparison.log`: same `TestRetrieveConversationMessages/BasicRequest` response-body EOF at both head and base; base was evaluated with Go's overlay replacing every PR-changed file, without modifying the checkout.

Replay without Go or service credentials:

```sh
python3 /Users/xuanyu.wang/repos/knowledge/analytics/sessions/2026-09-17/pr-32440-review-evidence/run_runtime.py
```

The oracle tests default empty join values plus NULL joins for the new OR logic. Ordinary missing-only predicates have unchanged empty-string assumptions, so those two scenarios run only with `join_use_nulls=0`. See the [full review](../../../deliverables/convi-7706-pr-32440-semantic-review.md) for coverage and limitations.
