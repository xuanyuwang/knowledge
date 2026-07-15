# Notifications Domain

## Purpose

The canonical knowledge home for QM/Coaching notification behavior from business trigger through recipient selection, visibility checks, rendering, delivery, retry, and operational diagnosis.

## Scope and Boundaries

**In scope**

- notification event/trigger inventory
- recipient, audience, permission, and visibility evaluation as used for delivery
- email, Slack, in-product, and other supported channels
- templates, payloads, links, localization, and configuration
- queues/jobs, retries, deduplication, and idempotency
- delivery observability, failures, diagnosis, and recovery
- cross-workflow notification architecture and ownership boundaries

**Out of scope**

- the underlying scorecard or review business workflow except where needed to define a trigger: `scorecard-workflows`
- analytics presentation semantics: `analytics`
- scorecard PG/CH projection: `scorecard-data-sync`

## Initial Knowledge Map

- [Architecture and routing](deliverables/architecture-and-routing.md)
- [Notification catalog](deliverables/notification-catalog.md)
- [Recipient and visibility semantics](deliverables/recipient-and-visibility-semantics.md)
- [Operational playbook](deliverables/operational-playbook.md)
- [Legacy source index](deliverables/legacy-source-index.md)

## Current State

The first migration is complete. Operational Slack/PagerDuty/GroundCover research from `oncall` and scorecard submit/publish recipient semantics from `scorecard-permission-policy` are synthesized here. Those source projects remain canonical for incident process and scorecard permission policy respectively; this domain owns the cross-workflow trigger-to-delivery view.

## Reading Order

1. Start with architecture and routing for the channel/ownership model.
2. Use the catalog to find a notification class and its source evidence.
3. Use recipient/visibility semantics for product notification eligibility.
4. Use the operational playbook for missing, misrouted, duplicated, or noisy notifications.
5. Use the legacy source index to trace claims back to their original investigation.

## Source Context

- **Primary repo:** `go-servers`
- **Related repos:** `director`, `config`, `cresta-proto`
- **Default branch context:** `main`

## Related Artifacts

- `project.yaml`
- `log/2026-07-14.md`
- `log/2026-07-15.md`
- `work-items/` when active tickets are migrated or created
- `sessions/`, `decisions/`, and `deliverables/` as content is synthesized
