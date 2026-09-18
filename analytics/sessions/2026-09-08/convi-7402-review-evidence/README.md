# PR 31335 review reproductions

All fixture data is synthetic. No production credentials are needed for SQL execution.

Run any `execute-*.sql` or `*-fixture-results.sql` file directly with:

```sh
clickhouse local --multiquery --queries-file /absolute/path/to/file.sql
```

Each file creates its own in-process Memory tables. `execute-submit-conversations.sql` incorrectly returns three rows including `old-other`; `execute-submit-stats.sql` correctly counts two. `execute-generated-singleton.sql` incorrectly returns only `missing`. `execute-generated-exclusive.sql` incorrectly selects 5 instead of 10. `execute-generated-empty-oneof.sql` fails with ClickHouse Code 43.

`convi7402_review_test.go` was temporarily run inside the source `insights-server/internal/analyticsimpl` package and then removed. It contains expected-contract assertions (intentionally failing on this head) and generator tests. The generators write into `/tmp/convi7402-review`, which must exist. Saved SQL files can be rerun without restoring the Go test. `run_sql.py`, `run_generated.py`, and `run_submit.py` record the fixture-generation procedure and use the original workspace and temporary paths.

`qa-suites.log` is a passing embedded-PostgreSQL/mocked-ClickHouse run. `targeted-existing.log` is a passing narrow parser/overlap run. `broad-existing-tests.log` records the unrelated-to-SQL-semantics whitespace assertion failure caused by the CTE extraction. `repro-tests.log` records the five failed expected-contract assertions.

See the [full review](../../../deliverables/convi-7402-pr-31335-adversarial-review.md) for interpretation and limits.
