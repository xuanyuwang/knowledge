# Knowledge

This repo is the operating system for staff-level engineering work.

It sits alongside source repos under `~/repos` and captures the context that should outlive a single branch, prompt, terminal session, or AI tool run.

## Core model

- Source code truth lives in the target repo or repo worktree.
- Reasoning truth lives here: investigations, design tradeoffs, execution notes, reviews, decisions, and synthesis.
- Every durable artifact should point back to a concrete repo, worktree, branch, commit, ticket, or PR.

## Why this exists

This repo exists to make growth toward **Staff Engineer** explicit, measurable, and repeatable.

It is used to:

- track where work is operating at senior level versus staff level
- preserve staff-level artifacts instead of losing them in chat logs
- turn day-to-day delivery into reusable judgment, design records, rollout plans, and retrospectives
- give Codex, Claude Code, and other AI tools a shared operating protocol

## Canonical interfaces

The workflow treats these files as the main interfaces:

- `workflow/ai-operating-model.md` - shared operating spec for humans and AI tools
- `workspace/repos.yaml` - repo and named worktree registry
- `<project>/project.yaml` - machine-readable project state
- `<project>/README.md` - human-readable project summary
- `<project>/log/YYYY-MM-DD.md` - daily progress record
- `<project>/sessions/YYYY-MM-DD/*.md` - raw session notes
- `CLAUDE.md` and `AGENTS.md` - tool-specific entrypoints into the shared spec

## Domain-centered structure

The default working unit is a long-lived **domain project**. Tickets are tracked as work items inside their primary domain. Standalone initiative projects are reserved for outcomes with multiple workstreams, meaningful cross-team coordination, or an independent design/rollout lifecycle.

Broad domains may contain `subdomains/` for product surfaces, capabilities, or workflows with distinct semantics. Subdomains hold durable reference knowledge; work items and daily operating history remain at the parent domain.

## Product domain catalog

### [Analytics](analytics/README.md)

Owns Performance Insights and Leaderboard across frontend and backend: displayed metric semantics, filters, analytics APIs, query/aggregation behavior, and data lineage. It should make each visible value traceable from UI interpretation to its source data.

- [Shared Analytics Platform](analytics/subdomains/shared-analytics-platform/README.md) — shared API, grouping, request, and source-data contracts
- [Performance Insights](analytics/subdomains/performance-insights/README.md) — page charts, tables, filters, and frontend transformations
- [Leaderboard](analytics/subdomains/leaderboard/README.md) — agent, team, and manager ranking and presentation behavior
- [Active Days](analytics/subdomains/active-days/README.md) — activity evidence, source filtering, attribution, and freshness
- [Quintiles](analytics/subdomains/quintiles/README.md) — ranked-population partition semantics and presentation
- [QA Score](analytics/subdomains/qa-score/README.md) — QA aggregation, N/A, weighting, option, and criterion semantics
- [Insights User Filter](analytics/subdomains/insights-user-filter/README.md) — user/team/group resolution, hierarchy, access, and scalable filtering
- [Conversation Volume](analytics/subdomains/conversation-volume/README.md) — surface-specific count definitions and data sources

### [Scorecard Workflows](scorecard-workflows/README.md)

Owns scorecard and template business behavior from authoring and evaluation through lifecycle transitions, permissions, appeals, calibration, and generation. PostgreSQL-to-ClickHouse projection mechanics belong to Scorecard Data Sync instead.

- [Template Authoring and Versioning](scorecard-workflows/subdomains/template-authoring-and-versioning/README.md) — template structure, revisions, duplication, and schema compatibility
- [Evaluation and Scoring](scorecard-workflows/subdomains/evaluation-and-scoring/README.md) — manual/AutoQM evaluation, option mapping, N/A, weighting, and score computation
- [Scorecard Lifecycle](scorecard-workflows/subdomains/scorecard-lifecycle/README.md) — creation, editing, submission, publishing, reversal, and state transitions
- [Permissions and Visibility](scorecard-workflows/subdomains/permissions-and-visibility/README.md) — capabilities, audiences, submitted editing, and runtime visibility
- [Appeals](scorecard-workflows/subdomains/appeals/README.md) — appeal request/resolution, final values, comments, and exports
- [Group Calibration](scorecard-workflows/subdomains/group-calibration/README.md) — answer keys, responses, completion, permissions, reporting, and exports
- [Process Scorecards and Generation](scorecard-workflows/subdomains/process-scorecards-and-generation/README.md) — existence rules, generation, repair, and backfill orchestration

### [Scorecard Data Sync](scorecard-data-sync/README.md)

Owns the correctness and operation of PostgreSQL-to-ClickHouse scorecard projection: ordering, monitoring, mismatch diagnosis, reindexing, backfill, repair, and validation. It currently remains one cohesive domain without subdomains.

### [Notifications](notifications/README.md)

Owns cross-workflow notification triggers, recipients and visibility, channels, templates, delivery behavior, retry/idempotency, observability, and diagnosis. It currently remains one cohesive domain without subdomains.

### [Training Simulator](training-simulator/README.md)

Owns scenario-based AI practice for agents: training content configuration, assignment as Director Tasks, Customer AI simulation runtime, Opera/LLM evaluation, and session-level reporting. Launched July 9, 2026 as a paid QM & Coach add-on; the engineering surface spans go-servers, director, cresta-proto, and python-ai-services.

- [Training Content](training-simulator/subdomains/training-content/README.md) — lessons, modules, scenarios, evaluation criteria, and quiz templates
- [Assignment and Session](training-simulator/subdomains/assignment-and-session/README.md) — DirectorTask modeling, audience expansion, task runs, and statuses
- [Simulation Runtime](training-simulator/subdomains/simulation-runtime/README.md) — Customer AI virtual agents, voice-agent/LiveKit pipeline, and role mapping
- [Evaluation](training-simulator/subdomains/evaluation/README.md) — Opera/LLM evaluation, moment annotations, scoring, pass/fail, and auto-fail
- [Reporting](training-simulator/subdomains/reporting/README.md) — session/agent/task stats, completion and pass-rate rollups

Each active engineering project should eventually contain:

- `project.yaml`
- `README.md`
- `subdomains/`
- `work-items/`
- `log/`
- `sessions/`
- `decisions/`
- `deliverables/`

Legacy projects migrate into domains gradually, normally when they are reopened or when related domain work needs their evidence. Synthesize durable knowledge and add canonical pointers before considering removal; do not perform a mechanical bulk move.

The canonical model and initial domains are defined in `workflow/domain-centered-knowledge-model.md`.

## Promotion path

The intended flow is:

1. A session note captures investigation or execution details.
2. A work item maintains current state for a continuing ticket or task.
3. The daily log records the meaningful movement for that day.
4. The domain `README.md` is updated when durable understanding changes.
5. Important decisions are promoted into `decisions/`.
6. Weekly and annual summaries promote impact evidence without duplicating raw notes.

## Staff track anchors

- Senior vs Staff gap framework: `train-for-staff/senior-to-staff.md`
- Staff project framing: `train-for-staff/staff-project.md`
- Resume bullets and synthesis: `train-for-staff/resume-snippets.md`
