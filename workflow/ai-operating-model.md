# AI Operating Model

This document is the shared operating protocol for `knowledge`.

It exists so Codex, Claude Code, and other AI tools can all work in the same repository without inventing incompatible habits or burying durable reasoning inside tool-specific chat history.

## Purpose

`knowledge` is the **context and synthesis layer** for engineering work.

- `/Users/xuanyu.wang/repos` is a lightweight workspace metadata Git repo that tracks shared AI/editor instructions and ignores child source repositories.
- Code changes happen in source repos under `~/repos` or in their worktrees.
- Cross-session reasoning happens here.
- Staff-level outputs are promoted from raw execution context into durable artifacts.

The goal is not just note-taking. The goal is to convert daily engineering work into reusable judgment, decision records, and evidence of staff-level impact.

The repository uses the domain-centered model defined in `workflow/domain-centered-knowledge-model.md`. Long-lived domain projects are the default home for product knowledge; tickets are normally tracked as work items inside a domain rather than as new top-level projects.

## Top-level model

The canonical working unit is a **project folder** at the repository root. Projects have three types:

- **domain**: a long-lived product/system ownership and knowledge surface; the default home for ticket work;
- **initiative**: a temporary outcome with multiple workstreams, independent coordination, or its own design/rollout lifecycle;
- **system**: repository workflow, career synthesis, or another shared operating surface.

A ticket's duration alone does not justify a new initiative project. A multi-week bug can remain a work item in its primary domain.

Each active engineering project should converge on this structure:

```text
<project>/
  project.yaml
  README.md
  work-items/
    <ticket-or-work-item>.md
  log/
    YYYY-MM-DD.md
  sessions/
    YYYY-MM-DD/
      <tool>-<topic>.md
  decisions/
    YYYY-MM-DD-<decision>.md
  deliverables/
    <artifact>.md
```

### Shared system folders

These folders support the workflow and are not normal project work areas:

- `templates/` - standard artifact templates
- `workflow/` - shared operating rules
- `workspace/` - repo and worktree registry
- `train-for-staff/` - long-horizon growth artifacts
- `blog/`, `weekly-summary/`, `general-learnings/` - promotion destinations for synthesized output
- `workflow/skills/` - canonical repository-local skills shared by AI tools
- `.claude/skills/` - optional Claude discovery aliases for canonical workflow skills

## Canonical interfaces

These files are the official interfaces between tools and humans:

- `workflow/ai-operating-model.md`
- `workflow/domain-centered-knowledge-model.md`
- `workspace/repos.yaml`
- `<project>/project.yaml`
- `<project>/README.md`
- `<project>/work-items/<ticket-or-work-item>.md`
- `<project>/log/YYYY-MM-DD.md`
- `<project>/sessions/YYYY-MM-DD/*.md`
- `CLAUDE.md`
- `AGENTS.md`

### Contract expectations

- Paths must be absolute when referring to local source repos or worktrees.
- Each project must have exactly one `primary_source_repo`.
- Each session note must identify exactly one source repo and one branch/worktree context.
- Durable claims should be traceable to a source artifact such as a commit, PR, ticket, query result, or design note.
- Every ticket or long-running task has at most one canonical work item in `knowledge` and exactly one primary domain.
- Linear or the relevant issue tracker remains authoritative for official ticket status; work items preserve technical state and evidence.

## Worktree rules

- Do not create a repo-specific worktree for `knowledge`. Work directly in the main `knowledge` checkout.
- For other repos, create new worktrees under `/Users/xuanyu.wang/repos`.
- Prefer stable, human-recognizable worktree folder names that match the ticket or topic.
- Record the exact worktree path in `project.yaml` and in session notes.
- Do not move source repos under `knowledge`. Keep them as siblings under the Git-backed `/Users/xuanyu.wang/repos` workspace metadata repo.

## Permissions policy

Many AI tools bind trust and permissions to the opened folder path, not to the underlying Git repository identity. A new worktree often looks like a new project and can trigger fresh permission prompts.

Use these defaults to reduce repeated permission requests:

- Keep source repos and new worktrees under `/Users/xuanyu.wang/repos`.
- Treat `/Users/xuanyu.wang/repos` as a lightweight metadata repo, not a monorepo. Its `.gitignore` intentionally excludes child repositories.
- When a tool supports choosing a workspace root, prefer `/Users/xuanyu.wang/repos` as the root for code work so repo checkouts and worktrees share one parent boundary.
- Keep `knowledge` separate: work directly in the main `knowledge` checkout rather than creating a `knowledge` worktree.
- Prefer persistent command allowlists or prefix-based approvals for recurring commands when the tool supports them.
- Keep repo-specific behavior in checked-in files such as `AGENTS.md` or `CLAUDE.md`; keep global trust or permission rules in the tool's user-level config when available.

Expected outcome:

- New worktrees under `/Users/xuanyu.wang/repos` are more likely to inherit the same broad filesystem boundary.
- Repeated command prompts should be reduced by persistent allow rules.
- A first trust prompt may still happen for some tools when a brand-new folder is opened; this is a tool limitation, not a workflow violation.

## File roles

### `project.yaml`

Machine-readable state for the project. It should be the fastest way for an agent to answer:

- What is this project?
- Which source repo does it belong to?
- Which worktree or branch is active?
- What is the current objective?
- What are the related tickets, PRs, and key artifacts?
- Is this a domain, initiative, or system project?
- Which active work items need continuation?

### `README.md`

Human-readable summary of the project. It should explain:

- problem statement
- current status
- key findings
- meaningful log history
- links to deeper artifacts

For a domain project, it should also explain scope boundaries, owned surfaces, architecture/data flows, semantics/invariants, operational risks, and current knowledge gaps.

### `work-items/`

The canonical current-state companion for a ticket or long-running task. A work item should capture:

- objective, impact, scope, and non-goals
- current status and latest conclusion
- source repos, branches, and worktrees
- findings, decisions, blockers, and dependencies
- validation and rollout state
- next actions
- a concise dated timeline linking to detailed logs and sessions

Choose one primary domain for cross-domain work. Secondary domains link to the canonical work item rather than creating copies.

### `log/YYYY-MM-DD.md`

Concise daily movement for the project. This is not the full transcript. It is the “what changed today” record.

### `sessions/YYYY-MM-DD/*.md`

Raw operating record for a specific AI or human session. Use this for:

- inputs reviewed
- commands or actions summarized
- findings
- temporary hypotheses
- decisions made during the session
- next steps

### `decisions/`

Durable decisions that should survive beyond the day they were made. Use this when the choice or tradeoff matters later.

### `deliverables/`

Polished artifacts intended for consumption beyond the immediate session, such as rollout plans, one-pagers, retrospectives, or review summaries.

## Session lifecycle

Use the following flow by default:

1. Decide whether the task creates durable reasoning. Investigations, designs, reviews, and multi-step execution do; tiny one-shot commands and narrow factual answers usually do not.
2. If the task creates durable reasoning, identify its primary domain. Use an initiative project only when the work meets the initiative threshold in `workflow/domain-centered-knowledge-model.md`.
3. Resolve the source repo via `workspace/repos.yaml`.
4. Use the main checkout for `knowledge`; if another repo needs a worktree, create it under `/Users/xuanyu.wang/repos`.
5. Read `project.yaml` and `README.md`; if the project does not have `project.yaml`, add it from `templates/project.yaml`.
6. For a ticket or long-running task, create or update its canonical `work-items/<id>.md` from `templates/work-item.md`.
7. Create or update a session note in `sessions/YYYY-MM-DD/`.
8. Do the work in the source repo or worktree if code changes are required.
9. Before final handoff, update the work item and `log/YYYY-MM-DD.md` with meaningful movement. Do this automatically for substantial work; do not wait for the human to request logging.
10. Update `README.md` if project/domain state or understanding changed.
11. Promote durable decisions or polished artifacts as needed.

The default mode for substantial work is therefore **domain-first, then work-item/session, then log-and-promote**. A durable investigation is incomplete until its findings and current state are saved under its primary domain or justified initiative.

## Artifact promotion rules

Promotion is how raw work becomes staff-level output.

1. Session note captures investigation or execution context.
2. Daily log captures the day’s meaningful movement.
3. Project README captures the current human-facing state.
4. Decision records capture durable tradeoffs and chosen direction.
5. Reusable lessons move to `general-learnings/`, `blog/`, or `weekly-summary/`.
6. Significant weekly outcomes are candidates for annual performance evidence under `train-for-staff/deliverables/`.

If an artifact is only useful while one session is open, keep it in `sessions/`.
If it should matter next week or next month, promote it.

## Allowed writes

Agents may update only:

- project `README.md`
- `project.yaml`
- `work-items/`
- `log/`
- `sessions/`
- `decisions/`
- `deliverables/`
- `templates/`
- `workflow/`
- `workspace/`
- `workflow/skills/`
- `.claude/skills/`

Agents must not invent new top-level folders or new artifact classes without first updating this workflow document.

## Naming conventions

- Project folders use stable, human-recognizable names.
- Daily logs use `YYYY-MM-DD.md`.
- Session notes use `<tool>-<short-topic>.md`.
- Decision records use `YYYY-MM-DD-<short-decision>.md`.
- Deliverables use descriptive names over ticket-only names.
- Work items use a stable ticket ID when one exists, for example `CONVI-1234.md`; otherwise use a short descriptive slug.
- Domain folders use stable product/system names and must not be named for a single ticket.

## Work-type expectations

### Investigation

- Create or update the canonical work item when the investigation belongs to a ticket or continuing task.
- Create or update a session note.
- Update the daily log.
- Update `README.md` if the current understanding changed materially.

### Design

- Record option analysis in a session note or working doc.
- Promote the chosen direction to `decisions/` if it affects future work.
- Update `README.md` with the current plan.

### Execution

- Keep current status, validation, and next actions in the work item.
- Record execution context in a session note.
- Keep code changes in the source repo or worktree.
- Link commits, branches, PRs, and validation results back into the project artifacts here.

### Review

- Record findings, risks, and follow-ups in a session note or deliverable.
- Promote durable review lessons into `general-learnings/` when they generalize beyond the project.

## Capture and synthesis skills

- Use `workflow/skills/daily-capture/SKILL.md` to close substantial work and ensure work items, session notes, and daily logs contain enough evidence for later synthesis.
- Use `workflow/skills/weekly-summary/SKILL.md` to synthesize weekly movement across domains into impact-oriented progress and performance-review evidence candidates.
- Skills must not stage, commit, or push unless the human explicitly asks for those Git actions.
- Daily capture is part of normal task completion. Weekly synthesis is run on request or at the user's chosen weekly cadence.

## Legacy backfill policy

This repo contains older folders created before the shared protocol existed.

- Do not rewrite historical material just to fit the new structure.
- When a legacy project is reopened for meaningful work, add or refresh `project.yaml` first.
- Add `sessions/`, `decisions/`, or `deliverables/` only when there is actual content to store there.

The goal is forward consistency, not cosmetic churn.
