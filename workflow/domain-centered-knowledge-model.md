# Domain-Centered Knowledge Model

## Objective

Organize durable knowledge around stable product/system domains while keeping tickets easy to continue, daily work easy to summarize, and impact evidence easy to promote into weekly and annual reviews.

## Information hierarchy

```text
domain project
  -> subdomain (optional knowledge partition)
    -> work item (operated at parent domain)
    -> dated session evidence
      -> daily movement
        -> weekly synthesis
          -> annual performance evidence
```

## Project types

### Domain

A long-lived ownership and mastery surface. It should answer:

- What is in and out of scope?
- What are the user-visible semantics?
- Which FE, API, service, job, storage, and configuration paths implement them?
- What are the major data/request flows and invariants?
- What fails, how is it diagnosed, and how is it repaired?
- Which decisions and historical constraints explain the current system?
- What remains risky or poorly understood?

### Initiative

A temporary outcome that deserves independent coordination. Create one only when work has multiple independently tracked workstreams, meaningful cross-team coordination, or a distinct design/rollout lifecycle. Link it back to affected domains and promote its durable learning when it closes.

### System

A repository operating surface such as `workspace`, `train-for-staff`, or promotion destinations.

## Domain Families and Subdomains

Broad domains may act as domain families. Use `subdomains/<name>/` for product surfaces, bounded capabilities, or workflows that need their own semantic and architectural map but still share parent entities, APIs, operations, or ownership.

- Start as a subdomain when shared truth would otherwise be duplicated.
- Keep work items, sessions, logs, and decisions at the parent domain; record an optional primary subdomain.
- Store cross-subdomain contracts at the parent.
- Promote to an independent top-level domain only when several are true: distinct ownership/roadmap, distinct entities/lifecycle, mostly independent code/data flow, sustained work volume, independent operational health, and low dependence on parent context.
- A separate UI workflow, large document set, or long-running ticket is not sufficient by itself.

## Ticket model

Tickets are normally work items, not projects.

- Select exactly one primary domain.
- Select at most one primary subdomain.
- Maintain one canonical `work-items/<ticket>.md` for current technical state.
- Keep official workflow/status in Linear or the relevant issue tracker.
- Put deep dated evidence in `sessions/YYYY-MM-DD/`.
- Put concise dated movement in `log/YYYY-MM-DD.md`.
- Link from secondary domains; never duplicate the work item.
- On completion, promote lasting semantics, architecture, operational lessons, or decisions into domain artifacts.

## Initial domains

### `analytics`

Performance Insights and Leaderboard, frontend and backend. Covers exact UI/chart/table semantics, filter interpretation, request construction, analytics-service APIs, backend aggregation/query behavior, and the correspondence between displayed values and source data.

### `scorecard-workflows`

Scorecard and template lifecycle, creation/evaluation/submission, permissions, reversal, appeals, group calibration, related review workflows, backfill/generation behavior, and surrounding business rules. PG-to-ClickHouse synchronization mechanics are explicitly excluded.

### `scorecard-data-sync`

PostgreSQL-to-ClickHouse scorecard projection, ordering and consistency, reindex/backfill mechanics, monitoring, diagnosis, repair, validation, and rollout safety.

### `notifications`

Notification triggers, recipients/visibility, delivery channels, templates/configuration, retry and idempotency behavior, observability, diagnosis, and ownership boundaries.

## Daily-to-yearly promotion

Capture facts once and synthesize upward:

1. Work item: current ticket/task state.
2. Session: rich evidence and reasoning.
3. Daily log: outcome, impact, role, evidence, follow-up.
4. Weekly summary: outcomes, stewardship, risk reduction, leverage, influence, metrics, lessons, and priorities.
5. Annual evidence: selected outcomes grouped by the performance rubric, with role, impact, collaborators, and measurable proof.

Ticket/PR counts and hours are supporting evidence, not impact by themselves.

## Migration principles

- Inventory before moving.
- Synthesize durable knowledge; do not mechanically concatenate ticket notes.
- Preserve history through links and thin legacy pointers until confidence is high.
- Give each artifact one canonical home.
- Do not force unrelated personal, career, workflow, or experimental material into product domains.
- Correct security and separation-of-concerns issues separately from taxonomy changes.
