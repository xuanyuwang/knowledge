# Engineering Design Review Guide

A solid engineering design review usually has two passes:

1. Validate the problem and the shape of the solution.
2. Look for failure modes.

## Review Steps

1. **Restate the goal**

   Make sure the design solves the actual product or user problem, not just the implementation problem. Confirm that goals, non-goals, and success criteria are explicit.

2. **Check assumptions**

   Identify anything the design depends on: data ownership, existing APIs, lifecycle behavior, permissions, scale, latency, consistency, migrations, user flows, and rollout order.

3. **Trace the main flows**

   Walk through create, read, update, delete, backfill, permissions, UI rendering, exports, analytics, and failure or retry paths. Designs often look fine until one concrete object is traced end to end.

4. **Review data model and contracts**

   Look for schema fit, uniqueness constraints, indexing, enum and backward compatibility, API contract changes, versioning, and whether meanings are being overloaded in risky ways.

5. **Review correctness**

   Ask whether the system returns the right answer under edge cases: missing data, stale data, concurrent writes, deleted entities, changed definitions, mixed old and new records, and partially rolled-out services.

6. **Review operational risk**

   Consider scale, latency, caching, fanout, query cost, transaction boundaries, observability, alerts, retries, idempotency, and runbooks.

7. **Review security and privacy**

   Check authorization, visibility scoping, data leakage through pickers, search, export, and logs, cross-tenant isolation, and whether existing permissions actually map to the new object.

8. **Review UX and product behavior**

   Confirm the user-facing behavior is understandable: naming, empty states, unavailable metrics, historical values, filters, exports, reports, and consistency across surfaces.

9. **Review migration and rollout**

   Look for feature flags, compatibility with old clients, backfill needs, staged rollout, rollback strategy, and what happens if the launch is paused halfway.

10. **Review test plan**

    Tests should cover not only happy paths, but also concurrency, permissions, mixed entity types, deleted or missing references, scale, and integration between services.

## Review Angles

- **API:** Is the contract minimal, explicit, and backward compatible?
- **Data model:** Are we reusing structures cleanly or overloading them?
- **Lifecycle:** Who creates, updates, deletes, and repairs each object?
- **Consistency:** What happens when source-of-truth data changes?
- **Concurrency:** Can duplicate or conflicting records be created?
- **Performance:** Are new reads or writes expensive on hot paths?
- **Security:** Could hidden or internal objects leak through generic APIs?
- **Observability:** Can we debug incorrect values in production?
- **Rollout:** Can we ship, monitor, and roll back safely?
- **Testing:** Do tests prove the risky assumptions, not just code coverage?

## ALO-Specific Focus Areas

For the ALOs in Coaching design, review especially hard around:

- Reusing empty scorecard templates as typed hosts.
- Lazy template creation and concurrency.
- Scorecard template listing leakage.
- User-outcome field-definition lifecycle changes.
- Coaching progress value sourcing.
- Use-case scoping semantics.
- Frontend and backend agreement on how to resolve criteria that are not real scorecard criteria.
