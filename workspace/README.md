# Knowledge Workspace Operating Protocol

This project tracks the shared workspace rules that let AI tools and humans coordinate across repos under `/Users/xuanyu.wang/repos`.

## Scope

- Maintain `workspace/repos.yaml` as the source of truth for repo paths and named worktrees.
- Keep the `knowledge` operating model explicit enough for Codex, Claude Code, and other tools to save durable reasoning in project folders.
- Capture workflow-level fixes that do not belong to a product-ticket project.

## Current State

The workspace now has parent-level `/Users/xuanyu.wang/repos/AGENTS.md` and `/Users/xuanyu.wang/repos/CLAUDE.md` files that tell Codex and Claude Code sessions started from the repo parent to use `/Users/xuanyu.wang/repos/knowledge` as the durable context layer.

`/Users/xuanyu.wang/repos` is now also a lightweight `workspace-meta` Git repository. It tracks shared workspace metadata such as `AGENTS.md`, `CLAUDE.md`, `README.md`, and `workspace.code-workspace`, while ignoring child source repositories so each child repo keeps its own Git history and status.

Substantial investigations, designs, reviews, and multi-step execution should be project-first: open or create a project under `knowledge`, create or update a session note, then update the daily project log before handoff.

As of 2026-07-14, the repository is moving to a domain-centered model. The initial product domains are `analytics`, `scorecard-workflows`, `scorecard-data-sync`, and `notifications`. Tickets are work items inside one primary domain by default; standalone initiative projects require multiple workstreams, independent coordination, or their own design/rollout lifecycle.

Daily capture is now a completion requirement for substantial work. Shared workflows at `workflow/skills/daily-capture/` and `workflow/skills/weekly-summary/` make work-item state, daily evidence, weekly progress, and performance-review candidates part of one promotion chain. Existing Claude `wrap-day` and `wrap-week` skills are compatibility aliases and no longer commit or push automatically.

Migration progress as of 2026-07-15: all four initial domains have canonical synthesized artifacts and legacy source links. `analytics` is organized into eight product/metric/filter subdomains, and `scorecard-workflows` into seven workflow subdomains. Operating history remains at each parent domain so tickets spanning multiple subdomains retain one coherent record.

## Domain Catalog

| Domain | Scope | Subdomains |
|---|---|---|
| [`analytics`](../analytics/README.md) | Performance Insights and Leaderboard semantics from frontend display through APIs, aggregation, filters, and source data | Shared Analytics Platform, Performance Insights, Leaderboard, Active Days, Quintiles, QA Score, Insights User Filter, Conversation Volume |
| [`scorecard-workflows`](../scorecard-workflows/README.md) | Template and scorecard authoring, evaluation, lifecycle, permissions, appeals, calibration, and generation | Template Authoring and Versioning, Evaluation and Scoring, Scorecard Lifecycle, Permissions and Visibility, Appeals, Group Calibration, Process Scorecards and Generation |
| [`scorecard-data-sync`](../scorecard-data-sync/README.md) | PostgreSQL-to-ClickHouse projection correctness, monitoring, diagnosis, reindexing, backfill, and repair | None currently; kept cohesive |
| [`notifications`](../notifications/README.md) | Cross-workflow triggers, recipients, visibility, channels, delivery, retries, observability, and diagnosis | None currently; kept cohesive |

Legacy projects migrate into these domains gradually as they are reopened or become relevant to active domain work. Migration is synthesis-first: preserve evidence, establish the canonical domain home, add a pointer, and review deletion separately rather than bulk-moving folders.

## Key Artifacts

- `/Users/xuanyu.wang/repos/AGENTS.md`
- `/Users/xuanyu.wang/repos/CLAUDE.md`
- `/Users/xuanyu.wang/repos/README.md`
- `/Users/xuanyu.wang/repos/workspace.code-workspace`
- `workflow/ai-operating-model.md`
- `workflow/domain-centered-knowledge-model.md`
- `workspace/deliverables/domain-reorganization-plan.md`
- `workflow/skills/daily-capture/SKILL.md`
- `workflow/skills/weekly-summary/SKILL.md`
- `workspace/repos.yaml`
- `AGENTS.md`
- `CLAUDE.md`
