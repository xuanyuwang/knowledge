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

- notification catalog by trigger and workflow
- end-to-end architecture and delivery channels
- recipient/visibility decision matrix
- template and configuration inventory
- retry/idempotency semantics
- observability and operational playbook
- known gaps and historical incidents

## Migration State

The domain is scaffolded. Existing notification-related material is distributed through oncall, scorecard permission, and ticket notes; those sources remain canonical until extracted and validated.

## Source Context

- **Primary repo:** `go-servers`
- **Related repos:** `director`, `config`, `cresta-proto`
- **Default branch context:** `main`

## Related Artifacts

- `project.yaml`
- `log/2026-07-14.md`
- `work-items/` when active tickets are migrated or created
- `sessions/`, `decisions/`, and `deliverables/` as content is synthesized
