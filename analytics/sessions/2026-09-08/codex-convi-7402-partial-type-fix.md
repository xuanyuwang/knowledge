# CONVI-7402 partial-type overlap fix

- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7402`
- Branch: `convi-7402-bswift-users-getting-qa-score-stats-errors`
- Starting head: `fe0109acd5`.
- User explicitly authorized fix, validation, commit, and push of the omitted-type shared overlap finding.
- Existing analytics project/operating-model and workspace registry remain applicable.
- Plan: reject overlaps by identity once either side identifies the group as metadata; add both omitted-type orientations and preservation controls; run focused/shared and QA regressions, commit, push normally, verify remote head.

Completed and pushed commit `8810713e2a1466ccc03d3540f0c86557e879f215` to the existing PR branch, verified by `git ls-remote`; local HEAD matches and the source worktree is clean.

## Implementation and validation

- Reused `elasticsearch.IsConversationMetadataMomentGroup` to classify the whole group before comparing all included/excluded IDs. An omitted type can no longer bypass the shared parser's `InvalidArgument` rejection of unsupported value-or-missing filters.
- Added both omitted-type orientations, both orientations where another ID identifies the group as metadata, and controls for nonmetadata groups and disabled exclusions. Error cases assert the gRPC status code.
- All four new overlap regressions failed before the source fix with a nil error, then passed after it.
- Combined Go validation passed: analyticsimpl (17.996s), Elasticsearch (0.935s). Command used `env -u TEST_DATABASE_URL GOPROXY=off GONOPROXY=none GOTOOLCHAIN=local go test ./insights-server/internal/analyticsimpl ./insights-server/internal/analyticsimpl/elasticsearch -run '^(TestParseMomentConditionsForQAAttribute.*|TestParseClickhouseFilter.*|TestQATimeRangeFilterTargetOptions|TestRetrieveQAScoreStatsClickhouseQuery.*|TestRetrieveQAConversations|TestRetrieveQaScoreStats|TestConvertConvoMomentGroup_IncludedAndExcludedSameMoment)$' -count=1 -timeout=3m`.
- gofmt, cached Gazelle diff check, git diff --check, and the pre-commit hook passed. No BUILD changes needed.
- No SQL generation changed; prior real ClickHouse runtime evidence remains in the second-review session.
- Normal push advanced the remote from `fe0109acd5` to `8810713e2a`; no force push or external review comments.
- Credentials used: individually cleared GitHub SSH key `~/.ssh/id_ed25519` with an isolated SSH configuration, and the embedded PostgreSQL local test credential. No production credentials used.

CI/code-owner review and production scan-cost/MV-freshness validation remain outside this local fix and push.

