# Hints: tests before refactor

TESTS: /Users/xuanyu.wang/.codex/worktrees/convi-7777-hints-tests/go-servers

REFACTOR: /Users/xuanyu.wang/.codex/worktrees/convi-7777-hints-refactor/go-servers

BASE: 6ba478ae4d7d7322e6f904dce2eb3f4ff2ea58c9

Source: insights-server/internal/analyticsimpl/retrieve_hint_stats_clickhouse.go

Scope: Most complex group. Cover behavior/policy/agent/time grouping branches, exact denominator source, sent/delivered and adherence paths, duplicate/missing actions and annotations, metadata filters. Extract selected raw rows into reusable feature relations while retaining legacy aggregation, joins, enums and guards. Do not silently redefine hint use or change existing behavior.

REPORT_PATH: /Users/xuanyu.wang/repos/knowledge/analytics/sessions/2026-10-19/convi-7777-feature-refactor-evidence/hints-report

## Required workflow and boundaries

Do not spawn subagents. Work only in your two assigned worktrees, never change another branch or root main. No push/PR creation or GitHub comments. Read .claude/skills/go-servers-testing/SKILL.md and repo instructions. User approved execution; no new design approval needed. Preserve original semantics including known bugs; no new API integration, proto/schema changes, or behavior correction. Keep one RPC per implementation/test file; helpers may be feature-specific common_*_usage_clickhouse.go. Build a genuinely reusable row relation, not merely move the whole aggregate function to a new file. Preserve raw grain, alias resolution, grouping, denominators, argument order, external tables and filters. Maintain old SQL where possible to avoid pointless snapshot churn; update SQL golden shapes when required.

1. In TESTS worktree, inspect original query, reader and relevant tests, then add comprehensive real ClickHouse regression tests (suite + isolated profile per scenario). Existing tests-only golden queries are not enough. Exercise the real old reader with business payload expectations. No production file edits. Include usage gates, duplicates, same-conversation multiple agents, denominators, filtered users both inline/external, time bounds/timezone grouping, empty/missing data, metadata include/exclude/latest semantics and existing feature-specific branches. Use DBTestSuite for response conversion if required, CH suite for actual SQL. Tests can target lower raw query rows when needed for difficult branches, but meaningful full-reader coverage is required.
2. Run all new tests plus existing feature suites on original production code. Fix mistaken fixture expectations only while production remains original. Once passing freeze SHA256 of new test files and baseline json logs. Commit tests + generated BUILD in TESTS worktree, title CONVI-7777: characterize <group> statistics before extraction.
3. In REFACTOR worktree, git merge --ff-only the tests branch (currently both begin at same base); perform extraction only there. Never change tests expectations to match extraction. Run the exact frozen tests and legacy suites. Record hashes and command logs and check unchanged hash. Gofmt every edited Go file.
4. Gazelle command before each commit: python3 /tmp/convi7777-gazelle.py "$PWD" <new_test_filename.go> [other new test files]. This runs the repo's prebuilt Gazelle wrapper for source BUILD and generates new test targets via isolated Gazelle workspace because package excludes *_test.go. Do not hand-maintain BUILD. Wrapper path already cached from earlier work; avoid waiting minutes on new-worktree Bazel repository mapping. If helper fails, report exact failure to root and continue useful independent work.
5. Tests command environment: DOCKER_HOST=unix:///Users/xuanyu.wang/.colima/default/docker.sock REUSABLE_CLICKHOUSE_CONTAINER_NAME= GOMAXPROCS=3 go test -p 2 ./insights-server/internal/analyticsimpl -run '<your new suites and relevant existing suite names>' -count=1 -json > /tmp/convi7777-<group>-before.jsonl 2>&1; after extraction use -after.jsonl. Run in background via exec session and keep progress updates; ensure final package pass. Do not share names or logs with other agents. Container init can take 15sec, regression suites minutes. Do not change global credential profiles; self-authenticating git/gh exempt but no external communication for this task.
6. Commit extraction + generated BUILD + updated SQL snapshots in refactor worktree. Self-review production equivalence and tests. Ensure tests branch diff includes only tests/BUILD, refactor branch diff includes only extraction/snapshot shapes/BUILD. Respond concise status + tests commit + refactor commit + evidence report path.

## Deliver evidence

Write a report at REPORT_PATH.json (use your group key): group, tests_worktree, refactor_worktree, base_commit, tests_commit, refactor_commit, test_files [{path,sha256}], commands [{stage,command,log,result,elapsed,passed_nodes}], scenarios [{name,expected_behavior,evidence}], extraction [{file,boundary,why,invariants}], preserved_issues [{title,trigger,observed,scope}], risks, source_files. Include enough details for root to generate construction report. Also write REPORT_PATH.md with semantic reasoning and exact concerns. Actual report not template must distinguish observed and unverified.
