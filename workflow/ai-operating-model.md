# AI Operating Model

This document is the shared operating protocol for `knowledge`.

It exists so Codex, Claude Code, and other AI tools can all work in the same repository without inventing incompatible habits or burying durable reasoning inside tool-specific chat history.

## Purpose

`knowledge` is the **context and synthesis layer** for engineering work.

- Code changes happen in source repos under `~/repos` or in their worktrees.
- Cross-session reasoning happens here.
- Staff-level outputs are promoted from raw execution context into durable artifacts.

The goal is not just note-taking. The goal is to convert daily engineering work into reusable judgment, decision records, and evidence of staff-level impact.

## Top-level model

The canonical working unit is a **project folder** at the repository root.

Each active engineering project should converge on this structure:

```text
<project>/
  project.yaml
  README.md
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

## Canonical interfaces

These files are the official interfaces between tools and humans:

- `workflow/ai-operating-model.md`
- `workspace/repos.yaml`
- `<project>/project.yaml`
- `<project>/README.md`
- `<project>/log/YYYY-MM-DD.md`
- `<project>/sessions/YYYY-MM-DD/*.md`
- `CLAUDE.md`
- `AGENTS.md`

### Contract expectations

- Paths must be absolute when referring to local source repos or worktrees.
- Each project must have exactly one `primary_source_repo`.
- Each session note must identify exactly one source repo and one branch/worktree context.
- Durable claims should be traceable to a source artifact such as a commit, PR, ticket, query result, or design note.

## Worktree rules

- Do not create a repo-specific worktree for `knowledge`. Work directly in the main `knowledge` checkout.
- For other repos, create new worktrees under `/Users/xuanyu.wang/repos`.
- Prefer stable, human-recognizable worktree folder names that match the ticket or topic.
- Record the exact worktree path in `project.yaml` and in session notes.

## Permissions policy

Many AI tools bind trust and permissions to the opened folder path, not to the underlying Git repository identity. A new worktree often looks like a new project and can trigger fresh permission prompts.

Use these defaults to reduce repeated permission requests:

- Keep source repos and new worktrees under `/Users/xuanyu.wang/repos`.
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

### `README.md`

Human-readable summary of the project. It should explain:

- problem statement
- current status
- key findings
- meaningful log history
- links to deeper artifacts

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

1. Resolve the source repo via `workspace/repos.yaml`.
2. Use the main checkout for `knowledge`; if another repo needs a worktree, create it under `/Users/xuanyu.wang/repos`.
3. Open or create the project folder.
4. Read `project.yaml` and `README.md`.
5. Create or update a session note in `sessions/YYYY-MM-DD/`.
6. Do the work in the source repo or worktree if code changes are required.
7. Update `log/YYYY-MM-DD.md` with the meaningful movement.
8. Update `README.md` if project state changed.
9. Promote durable decisions or polished artifacts as needed.

## Artifact promotion rules

Promotion is how raw work becomes staff-level output.

1. Session note captures investigation or execution context.
2. Daily log captures the day’s meaningful movement.
3. Project README captures the current human-facing state.
4. Decision records capture durable tradeoffs and chosen direction.
5. Reusable lessons move to `general-learnings/`, `blog/`, or `weekly-summary/`.

If an artifact is only useful while one session is open, keep it in `sessions/`.
If it should matter next week or next month, promote it.

## Allowed writes

Agents may update only:

- project `README.md`
- `project.yaml`
- `log/`
- `sessions/`
- `decisions/`
- `deliverables/`
- `templates/`
- `workflow/`
- `workspace/`

Agents must not invent new top-level folders or new artifact classes without first updating this workflow document.

## Naming conventions

- Project folders use stable, human-recognizable names.
- Daily logs use `YYYY-MM-DD.md`.
- Session notes use `<tool>-<short-topic>.md`.
- Decision records use `YYYY-MM-DD-<short-decision>.md`.
- Deliverables use descriptive names over ticket-only names.

## Work-type expectations

### Investigation

- Create or update a session note.
- Update the daily log.
- Update `README.md` if the current understanding changed materially.

### Design

- Record option analysis in a session note or working doc.
- Promote the chosen direction to `decisions/` if it affects future work.
- Update `README.md` with the current plan.

### Execution

- Record execution context in a session note.
- Keep code changes in the source repo or worktree.
- Link commits, branches, PRs, and validation results back into the project artifacts here.

### Review

- Record findings, risks, and follow-ups in a session note or deliverable.
- Promote durable review lessons into `general-learnings/` when they generalize beyond the project.

## Legacy backfill policy

This repo contains older folders created before the shared protocol existed.

- Do not rewrite historical material just to fit the new structure.
- When a legacy project is reopened for meaningful work, add or refresh `project.yaml` first.
- Add `sessions/`, `decisions/`, or `deliverables/` only when there is actual content to store there.

The goal is forward consistency, not cosmetic churn.
