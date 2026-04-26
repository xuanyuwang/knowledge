# Solution Space

## Purpose

Evaluate concrete prevention, detection, and repair approaches after the theory and case mapping are clear.

## Buckets

### Prevention

- Stronger write-path ordering
- Explicit versioning for sink updates
- Transactional outbox or CDC-based projection
- Safer idempotent upsert semantics

### Detection

- Count-based sync monitoring
- ID-level inventory comparison
- Submitted/unsubmitted split metrics
- Field-level verification sampling

### Repair

- Time-range reindex
- ID-based targeted reindex
- Full projection rebuild from PostgreSQL
- Periodic reconciliation sweeps

## Evaluation Criteria

- Correctness
- Operational cost
- Recovery latency
- Coverage of edge cases
- Ease of proving convergence
