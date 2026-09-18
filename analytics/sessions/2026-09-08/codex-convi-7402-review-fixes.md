# CONVI-7402 correctness review fixes

- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7402`
- Branch: `convi-7402-bswift-users-getting-qa-score-stats-errors`
- Starting head: `12117463df3318998e944551b63913c6110ba6b6` (clean; rebased during review).
- User authorized fixing each reviewed finding in its own commit. No push authorization.
- Sequence: QA conversations submit time; complete shared overlap rejection; numeric bin boundaries; empty metadata value validation; whitespace-sensitive outcome assertion in a separate cleanup commit.
- Evidence and original findings: `deliverables/convi-7402-pr-31335-adversarial-review.md` and `sessions/2026-09-08/convi-7402-review-evidence/`.

## Completed commit stack

| Commit | Change | Regression coverage |
|---|---|---|
| `5970423a86` | Pass effective conversation time range to QA conversations moment parser | New endpoint test fails before the fix, passes after; submit-only and incompatible-ended-at cases pass |
| `cd3990ac3c` | Validate overlapping metadata IDs across entire shared groups before lowering | Multi-ID and duplicate overlaps, malformed names, distinct IDs, include-only, exclude-only and flag-off behavior |
| `a3efa37d07` | Respect both numeric-bin boundary flags | Ten include/overlap cases covering all open/closed combinations and singleton [5,5] |
| `1036223149` | Reject empty metadata oneofs and nil attributes before SQL construction | Invalid combinations reject with InvalidArgument; zero, false and empty string remain valid; includes and overlaps covered |
| `fe0109acd5` | Normalize whitespace for outcome SQL source assertion | Previously failing outcome query test passes while retaining full golden comparison |

All five commits passed the repository pre-commit hook. No push, external comment, or review-thread update was performed. Final source head is `fe0109acd5`; source worktree is clean. User changes outside the scoped source files were preserved. No new BUILD dependency entries are required (the test inherits its imports through the embedded library).

## Validation

- Each correctness regression was run against the preceding code and reproduced failure before its fix. The numeric suite showed eight failing nondefault-boundary cases; the default cases passed. Value-validation tests reproduced both accepted empty values and a nil-attribute panic before the fix.
- After each fix, the relevant tests passed before committing it. Logs are retained in [fix evidence](convi-7402-fix-evidence/).
- Combined Go run passed: analyticsimpl 17.208 seconds; ES package 0.999 seconds. Pattern covers all `TestParseMomentConditionsForQAAttribute*`, `TestParseClickhouseFilter*`, `TestQATimeRangeFilterTargetOptions`, `TestRetrieveQAScoreStatsClickhouseQuery*`, both QA endpoint suites, and `TestConvertConvoMomentGroup_IncludedAndExcludedSameMoment`.
- Command: `env -u TEST_DATABASE_URL GOPROXY=off GONOPROXY=none GOTOOLCHAIN=local go test ./insights-server/internal/analyticsimpl ./insights-server/internal/analyticsimpl/elasticsearch -run '^(TestParseMomentConditionsForQAAttribute.*|TestParseClickhouseFilter.*|TestQATimeRangeFilterTargetOptions|TestRetrieveQAScoreStatsClickhouseQuery.*|TestRetrieveQAConversations|TestRetrieveQaScoreStats|TestConvertConvoMomentGroup_IncludedAndExcludedSameMoment)$' -count=1 -timeout=3m`.
- Cached repository Gazelle runner: `BUILD_WORKSPACE_DIRECTORY=/Users/xuanyu.wang/repos/go-servers-convi-7402 bazel-bin/gazelle -mode=diff -r=false insights-server/internal/analyticsimpl` passed with no differences.
- Temporary Go exporters generated SQL from the fixed parsers/builders; endpoint regression independently verifies the actual time-range wiring. Export tests passed. Temporary source was removed after validation.
- ClickHouse local 26.8.2.7 executed both full committed goldens, both generated full submit-time queries, and generated string/raw/numeric CTE fixtures. Assertions passed: submit-time conversations and stats both include two rows; singleton selects `five, missing`; (5,10] selects `missing, six, ten`; selected/missing strings and raw path select `missing, selected`; ordinary independent-filter/latest-update/duplicate fixtures still select the correct three rows. Empty metadata values fail validation before export.
- `git diff 12117463df..HEAD --check`, final working diff check and clean source status passed.

## Boundaries and next steps

The full analytics package suite, Bazel build/test, live ES differential testing and production distributed/load testing were not run. The QA endpoint suites use embedded PostgreSQL and mocked ClickHouse; separate local ClickHouse Memory-table fixtures exercise real SQL. Production scan cost, MV freshness and raw/MV ordering remain the residual risks identified in the original review.

Only the repository embedded-PostgreSQL test credential (`cresta` account) was used in this implementation pass, after its entry had been individually read and cleared in this session. `TEST_DATABASE_URL` was unset. Git commits were local and unsigned; no GitHub/SSH credential was used for these fixes. Local ClickHouse used no credentials.

Ready for user review and a separately authorized push. Because the starting branch was already rebased in the previous review, the remote history differs; no remote update was attempted.

