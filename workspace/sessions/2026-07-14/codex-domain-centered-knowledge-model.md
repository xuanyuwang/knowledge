# Domain-centered knowledge model assessment

## Context

Xuanyu is pausing new material for one week and wants to use the pause to reconsider the repository's project model.

Current problems:

- ticket-named projects will grow without bound;
- durable domain knowledge is fragmented across ticket projects;
- the repository should help Xuanyu become an effective owner of assigned QM/Coaching domains at both detailed and architectural levels.

## Inputs reviewed

- `workflow/ai-operating-model.md`
- `workspace/repos.yaml`
- root project/folder inventory
- root `README.md`
- `workspace/project.yaml` and `workspace/README.md`
- [QM Coaching Owners](https://docs.superhuman.com/d/QM-Coaching-Owners_d3uj6WqhzW1/QM-Coaching-Owners_suZEl4vj#_lujcz3l4)

The ownership document says maintenance issues remain the responsibility of `qm-coaching-oncalls`, while domain owners are responsible for incoming feature requests.

## Assigned domains

Rows listing Xuanyu as an owner:

- Scorecard Data Sync
- Performance Insights
- Assistant Insights/Leaderboard BE
- Appeal (shared)
- Group Calibration (shared)
- Scorecard General Issues (shared)
- Notifications (shared)

## Assessment

The repository currently overloads "project" to mean both a durable body of knowledge and a temporary unit of execution. Ticket folders are useful while work is active, but are a poor long-term navigation layer. The stable unit should be a domain or durable initiative; a ticket should normally be evidence recorded inside that unit.

The current material already forms recognizable clusters. Examples include Performance Insights and analytics behavior tickets, scorecard synchronization and lifecycle investigations, leaderboard work, and group-calibration/appeal work. This supports consolidation rather than a completely new classification system.

## Recommended model

Use two explicit project types:

1. **Domain projects**: long-lived ownership and knowledge surfaces. These are the default home for tickets, investigations, architecture, operations, and decisions in an owned domain.
2. **Initiative projects**: temporary, outcome-oriented efforts that deserve their own state because they span domains, run for weeks, or have an independent design/rollout lifecycle.

A single bug or feature ticket should not create a project by default. It should create a session/case note in its primary domain project and be linked from `project.yaml` or a domain case-study index. Cross-domain tickets should have one primary home and links from secondary domains, avoiding duplicated notes.

## Suggested initial domain map

Prefer a small number of coherent projects rather than mechanically creating one folder for every ownership-table row:

- `performance-insights`: PI computation, filters, counts, scores, outcome integration, analytics semantics, and operational failure modes.
- `scorecard-platform`: scorecard lifecycle and general backend concepts, with explicit sections for PG/CH data sync, processing/reindexing, templates/evaluation, submission/permissions, and consistency. If this becomes too broad, split `scorecard-data-sync` only after the content demonstrates a clear independent boundary.
- `leaderboard-backend`: Assistant Insights/Leaderboard backend architecture, aggregation semantics, APIs, rollout/configuration, and operational behavior.
- `review-workflows`: Appeals and Group Calibration as related post-evaluation review workflows. Preserve `group-calibration` as the likely seed and split later only if ownership or architecture diverges.
- `notifications`: notification triggers, delivery paths, templates/configuration, retries/idempotency, observability, and ownership boundaries.

`Scorecard General Issues` should be treated as an ownership/intake label, not necessarily as the name of a knowledge project. Its knowledge belongs in the more concrete scorecard architecture sections.

## Standard domain project content

Each domain should make ownership competence testable through a small set of maintained artifacts:

- `README.md`: scope/boundaries, current architecture map, source repos/services, ownership, health, open questions, and links to deeper material.
- `deliverables/architecture-overview.md`: components and request/data flows.
- `deliverables/domain-model.md`: entities, storage, invariants, and terminology.
- `deliverables/operational-playbook.md`: alerts, dashboards, logs, common failure modes, diagnosis, recovery, and escalation.
- `deliverables/case-studies.md`: concise index of important tickets/incidents and what each taught.
- `decisions/`: durable choices and tradeoffs.
- `sessions/`: raw investigations and ticket execution notes.
- `log/`: changes in understanding, system behavior, ownership, or project state.

The exact artifact set should be created only when content exists; empty scaffolding is not useful.

## Ticket intake rule

For each new ticket:

1. Choose one primary domain.
2. Add the ticket to that domain's active references.
3. Save the investigation/execution session there, including source repo and worktree context.
4. After completion, update the domain's architecture, operations, decisions, or case-study material only when understanding changed.
5. Create a separate initiative project only when the effort is multi-week, cross-domain, or independently managed.

This turns tickets into observations that refine a domain model instead of permanent top-level taxonomy.

## Migration strategy

Avoid a wholesale file move. Use a synthesis-first migration:

1. Inventory ticket folders and map each to a primary domain, optional secondary domains, and disposition (`merge`, `keep as initiative`, `legacy/reference`, or `unrelated`).
2. Create/choose the small set of domain projects and their README boundary statements.
3. Extract durable knowledge from ticket folders into the appropriate architecture, model, operations, decisions, and case-study artifacts.
4. Turn migrated ticket folders into thin legacy pointers or leave them frozen until confidence is high; do not duplicate active truth.
5. Update the operating model so future agents default to domain-first ticket intake.
6. Only then consider moving or removing legacy folders.

## Proposed success criteria

For each owned domain, Xuanyu should be able to answer from the repository:

- What is in and out of scope?
- Which services, jobs, APIs, storage systems, configs, and UI surfaces participate?
- What are the main request/data flows and invariants?
- What commonly fails, how is it detected, and how is it repaired?
- Which decisions explain the current design?
- Which recent tickets changed the model?
- What are the major risks, gaps, and next improvements?

## Daily, weekly, and performance-review integration

Domain stewardship and career tracking should use the same evidence rather than separate journals. Use a promotion chain:

```text
ticket / PR / design / incident
  -> domain session note
  -> domain daily log
  -> weekly summary
  -> quarterly/yearly performance evidence
```

### Daily capture

The domain project's `log/YYYY-MM-DD.md` remains the source of truth. Record meaningful movement, not a transcript or list of hours. A useful entry captures:

- outcome: what changed, was decided, learned, shipped, or unblocked;
- impact: why it matters to customers, reliability, delivery speed, correctness, or the team;
- role: led, designed, implemented, reviewed, diagnosed, coordinated, or supported;
- evidence: ticket, PR, commit, dashboard/query, design, decision, or feedback;
- follow-up: missing validation, adoption, metrics, or next action.

When one day touches several domains, write to each relevant project log. Do not copy full notes into a global daily journal.

### Weekly synthesis

`weekly-summary/` should be generated from the week's project logs and sessions. Organize it by impact rather than ticket chronology:

- outcomes delivered;
- domain stewardship and knowledge gained;
- reliability/risk reduction;
- leverage for other engineers;
- cross-team influence and decisions;
- evidence and measurable results;
- misses, lessons, and next-week priorities.

The weekly summary is the first aggregation layer and should link back to domain evidence.

### Quarterly and yearly promotion

Maintain one annual evidence artifact under `train-for-staff/deliverables/`, for example `performance-evidence-2026.md`. Promote only significant weekly items into it, grouped by the company's performance rubric or the existing Staff dimensions:

- scope and ownership;
- problem framing and strategy;
- architecture and correctness over time;
- execution through leverage;
- influence without authority;
- operational excellence and risk management.

Each promoted item should state the situation, Xuanyu's role, action/judgment, outcome, measurable evidence, and collaborators. Keep pending metrics explicit so they can be closed later. Resume bullets are a later, more selective promotion from this annual evidence artifact.

### Tracking principle

Do not treat ticket count, PR count, or hours as impact. They can be supporting evidence, but the durable record should emphasize outcomes, changed understanding, reduced risk, enabled people, and sustained domain health.

This model makes domain ownership and performance-review preparation reinforce each other: ticket work improves the domain reference, and the same improvement becomes evidence of growing scope, judgment, leverage, and ownership.

## Long-running ticket tracking

Daily logs answer "what changed on this date?" but do not provide a convenient current-state view for a bug that lasts days or weeks. Add a third concept between a domain project and daily evidence: a **work item**.

Suggested location:

```text
<primary-domain>/
  work-items/
    CONVI-1234.md
```

The work-item file is the single technical companion for the ticket. Linear remains the source of truth for official workflow/status; the repository file holds investigation and execution context that Linear does not preserve well.

It should contain:

- objective and customer/system impact;
- current status and latest conclusion;
- scope and non-goals;
- source repos, branches, and worktrees;
- investigation hypotheses and findings;
- decisions and important evidence links;
- blockers and dependencies;
- validation/rollout state;
- next actions;
- a concise dated timeline linking to detailed daily logs or session notes.

The update flow becomes:

1. Update the work item with the latest state and next action.
2. Add a short entry to the relevant domain daily log describing that day's movement and link to the work item.
3. Store deep raw investigation in dated session notes when needed.
4. On completion, close the work item and promote durable learning into the domain architecture, operations, decisions, or case-study material.

For a ticket spanning multiple domains, choose exactly one primary domain and keep the canonical work item there. Secondary-domain logs link to it and record only the movement relevant to that domain. Do not create duplicate work-item files.

Escalate a ticket from a work item into a standalone initiative project only when it develops its own multi-week plan, multiple independently tracked workstreams, cross-team coordination, or a distinct design/rollout lifecycle. Duration alone is not sufficient: a difficult three-week bug can remain one work item.

`work-items/` would be a new artifact class, so it must be added to `workflow/ai-operating-model.md` and the repository write boundaries before use. This is preferable to forcing mutable ticket state into `deliverables/` or scattering it across dated `sessions/`.

## Next recommended action

After the break, run a read-only inventory and produce a migration matrix before changing folders. Pilot consolidation with `performance-insights`, because the repository already has a dense cluster of related ticket material and it will expose whether the proposed domain artifact structure is sufficient. During the pilot, also test the promotion chain by producing one domain daily log, one weekly summary, and one candidate annual evidence entry from the same work.
