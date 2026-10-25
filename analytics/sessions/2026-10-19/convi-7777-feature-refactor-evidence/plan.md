# CONVI-7777 feature characterization and extraction plan

The user approved four remaining feature groups, two worktrees and two PRs per group, regression tests before production extraction, and one interactive construction report per group. KB has existing validated checkpoints and must be published alongside them. No new API integration or unrelated behavior corrections.

Base: 6ba478ae4d7d7322e6f904dce2eb3f4ff2ea58c9 (fetched origin/main). Relevant feature query files are identical to prior 6789995fb9. Independent stacks target main, with refactor PR based on tests PR. Each tests branch and refactor branch has its own worktree. Existing population PR stack stays untouched.

Global constraints: Read repo AGENTS/CLAUDE and applicable testing skill. Real ClickHouse characterization, distinct profiles per scenario, Given/When/Then, suite hooks, stable business fields asserted. Baseline and refactor run the exact same frozen tests. Freeze hash and evidence logs before production edits. Preserve known legacy issues; report rather than fix. No API, proto, or schema changes. Gofmt every Go edit. Generate BUILD with Gazelle. No GitHub comments or messages. Root owns PR creation, native stack linkage, final report production and reviews.

## Tasks

1. Chat Suggestions: characterize all counters, event types, duplicate rows, denominators, selected agents, group/time buckets, generic/metadata filters, missing/null/empty evidence and source behavior; extract raw event selector and retain old aggregation.
2. Notes / Guided Workflow: characterize each RPC independently, CopyEntity or document/session gates, duplicates, grouping/time/filter and metadata branches. Extract feature rows without changing grain or aggregates.
3. Summary / GenAI: characterize each RPC, valid generation/use linkage and knowledge-session gates, duplicates, grouping/time/filter/metadata branches. Extract rows with semantics intact.
4. Hints: characterize supported grouping branches (behavior/policy/agent/time), delivered/adherence paths, joins, zero/missing states, filters and metadata, ordering/limits. Extract reusable row selection retaining denominators and aggregation semantics.
5. Root: independent semantic review of every tests+refactor diff. Resolve issues, freeze expectations, rerun only affected suites. Publish ten PRs as five independent two-PR native stacks; include KB checkpoints rebased to main and verified.
6. Root: construction report per group, with actual snapshots and diffs, baseline/test/extraction steps, before/after test hashes and results, scenario judgments, preserved bugs, effects beyond diff, open limits, pinned source links. Programmatically check HTML/JS/navigation; visual check when available.

## Conflict scan

| Tasks | Shared boundary | Decision |
| --- | --- | --- |
| 1/2/3/4 | Same package names and BUILD file | Separate Git worktrees; no shared helper edits across stacks. Root performs combined build validation after all groups. |
| all tasks | Test container runtime | Unique fixture tenants/profiles; fresh CH containers; reduce compile parallelism. |
| 5 and implementers | Branch refs | Root waits for committed tests and refactor before pushing or restacking. |
| 6 and implementers | Evidence | Implementer report JSON records immutable tests hash, commands, log paths, cases and preserved behavior. |
| tasks 1-4 internal | Tests must precede refactor | Tests-only worktree stays production-identical to base; refactor worktree fast-forwards tests then extracts. |

Ruling: Use independent parallel agents under dispatching-parallel-agents for isolated worktree pairs. No overlapping filesystems or branches; this is the exception to serial implementers recommended by subagent-driven-development.
