# CONVI-7402 submit-time moment source scan

- Source repo: `/Users/xuanyu.wang/repos/go-servers`
- Worktree: `/Users/xuanyu.wang/repos/go-servers-convi-7402`
- Branch: `convi-7402-bswift-users-getting-qa-score-stats-errors`
- Starting head: `8810713e2a`.
- User requested independent verification and a minimal validated fix for unbounded moment CTE scans on SUBMIT_TIME score statistics requests.
- Verified against current code: SUBMIT_TIME deliberately passes no conversation-time range to the moment parser; generated CTEs had no candidate-conversation predicate. Finding remains valid.

## Fix

Commit `a9cf40e6c6` adds a candidate predicate to all included, excluded, selected-value and existence filters only for SUBMIT_TIME. It uses `conversation_id GLOBAL IN (SELECT DISTINCT conversation_id FROM scorecard_score_per_conversation)` so distributed annotation shards receive the complete candidate set. Existing conversation-time predicates for every other time-target enum are unchanged. The candidate CTE does not depend on moment CTEs, so this creates no recursive dependency.

## Validation

- Endpoint regression checks all six source CTEs (latest, selected values, exclusion and overlap absence) and all time-target enum values. Before the fix, all six SUBMIT_TIME assertions failed; after the fix, they pass. An initial test compile error used a nonexistent enum name; the test now enumerates actual proto values.
- Combined parser/query/QA-suite/ES run passes: analyticsimpl 18.652s, Elasticsearch 1.142s. Same offline/local Go environment and selection as the partial-type fix session.
- Local ClickHouse 26.8 execution of the full generated service query returns two expected conversations with aggregate `agent / 160 / 2 / 2 / 2`: old selected and missing values are retained; old nonmatching values, excluded conversations and out-of-window submissions are rejected.
- Direct execution of all six source CTEs confirms they exclude conversations whose scorecards fall outside the submit window and conversations without scores. Empty candidate set returns no results.
- The runtime harness initially matched CTE references as well as definitions; restricting its regex to definitions corrected the harness, with all three runtime checks passing. No source change was required.
- gofmt, Gazelle diff check, git diff --check and pre-commit hook pass. Temporary SQL exporter removed; only the intended source/test changes are committed.
- [Evidence](convi-7402-submit-time-scan-evidence/) contains tests, generated SQL, synthetic fixtures, runtime harness and results.
- Credentials used: local embedded PostgreSQL test credential and the previously cleared GitHub SSH key `~/.ssh/id_ed25519`. Local ClickHouse requires no credential.

Production physical read volume and distributed-cluster performance are not measured here; the candidate restriction is validated locally. CI/code-owner review and production cost/freshness validation remain.


Normal push completed. Remote branch and local HEAD both verified as `a9cf40e6c6d23ecdc74c79b227add42b71e3d6fb`; source worktree is clean. Knowledge session, work item, README, project notes and daily log updated, with scoped Markdown/YAML diff check passing. Knowledge changes remain uncommitted.
