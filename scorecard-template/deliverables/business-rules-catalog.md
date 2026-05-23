# Scorecard / Template Business Rules Catalog

**Created:** 2026-05-17  
**Status:** First populated pass  
**Purpose:** Organize the most important current business rules by lifecycle stage and rule bucket

## How To Use This Document

This is not a full spec. It is a working catalog of the rules that repeatedly matter in scorecard/template work.

Each rule is intentionally short and placed where it is most useful for investigation:

- by lifecycle stage
- then by rule bucket

## Stage 1: Template Creation and Authoring

### Creation rules

- A template is a reusable evaluation definition, not a one-off runtime artifact.
- Template type matters. Conversation and process scorecards do not share every runtime assumption.
- Criterion type determines which settings are meaningful and which scoring paths are valid.

### Edit rules

- Builder form state is not always canonical persisted state.
- For some scorable criteria, edit-time values may be optimized for authoring convenience rather than persisted meaning.
- Outcome-related criteria may follow different save behavior from ordinary criteria.

### Visibility and access rules

- Templates include operational semantics such as audience, permissions, and use-case scope.
- Some “template issues” are actually applicability issues, not scoring issues.

## Stage 2: Save / Normalization / Revision

### Edit rules

- Save transforms are part of the domain, not just UI plumbing.
- Persisted/API shape should be treated as canonical when debugging stored data.

### Versioning rules

- A template has both an identity and a revision.
- Revision is the boundary for historical interpretation and replay.
- Current template identity alone is not enough to interpret a historical scorecard safely.

### Representation rules

- `options[].value` is usually an option identity or lookup key in persisted data.
- `scores[].value` points to that same lookup key.
- `scores[].score` is the business score meaning.

## Stage 3: Scorecard Instantiation

### Creation rules

- A scorecard is a runtime artifact created in a concrete business context.
- Scorecard creation depends on both template semantics and runtime context.
- Not all scorecards are conversation-centric; process scorecards are a distinct runtime path.

### Versioning rules

- The effective template revision matters at scorecard instantiation and later interpretation.

## Stage 4: Scoring Input Capture

### Scoring rules

- Stored `numeric_value` is often a raw criterion value or option key, not the final score.
- AutoQA writes values in the same domain as option wiring, not in the domain of mapped business score.
- Manual scoring and AutoQA can share runtime storage while differing in how values are produced.

### Historical consistency rules

- Stored values should preserve enough meaning to be interpreted later against the right template revision.

## Stage 5: Score Semantics Computation

### Scoring rules

- Percentage score must be computed from mapped score semantics when value-score mappings exist, not from raw value alone.
- The denominator should reflect the criterion’s effective max score semantics, not just a raw max option value when those differ.
- N/A handling is a real scoring rule, not a display-only detail.
- Branch applicability and criterion validity affect which score rows should participate in aggregation.
- Weight is part of score meaning because rollups depend on it.

### Migration and compatibility rules

- Legacy and newer scoring paths must preserve the same derived semantics for analytics fields such as `percentage_value`.

## Stage 6: Authoritative Persistence

### Historical consistency rules

- Postgres is the authoritative store for scorecards and scores.
- Downstream representations must be recoverable from authoritative Postgres data plus template semantics.

### Operational rules

- Scorecard and score writes need to stay coherent across update and submit flows.
- Async work must not treat stale closure state as authoritative truth.

## Stage 7: Submission / Finalization

### Lifecycle rules

- Submission changes business meaning and analytics relevance.
- Submission is important, but scorecards may still be operationally mutable in some flows.

### Historical consistency rules

- Submitted state in analytics must reflect authoritative Postgres state.

## Stage 8: Analytics Projection

### Projection rules

- ClickHouse is a derived analytics projection, not a source of truth.
- Derived analytics fields must preserve the same semantics as the authoritative scoring pipeline.
- Process scorecards require scorecard-centric projection or reindex support; conversation-centric recovery is not enough.

### Failure-awareness rules

- Projection failures are not only missing-row failures.
- A row can exist in ClickHouse and still be wrong because submit state, score semantics, or derived fields are stale or miscomputed.

## Stage 9: Historical Querying and Display

### Interpretation rules

- Historical scorecards must be interpreted using the right representation and, ideally, the right revision semantics.
- UI display logic must not assume array index and option identity are interchangeable.
- Label display is part of correctness, not just presentation.

### Historical consistency rules

- Changing template authoring behavior should not silently change how old scorecards are interpreted.

## Stage 10: Repair / Reindex / Backfill

### Recovery rules

- Recovery should start from authoritative Postgres state and correct template semantics.
- Recovery paths must account for multiple scorecard types and not assume conversation-only flows.
- Repair success should be evaluated at both count level and semantic level.

## Hotspot Rules Worth Memorizing

- Persisted option value is usually a lookup key, not the business score.
- Mapped score and raw value are different concepts.
- Revision matters for historical correctness.
- Postgres is authoritative; ClickHouse is derived.
- Conversation-centric logic does not cover every scorecard path.
- Representation mismatches often look like business-rule bugs.

## Suggested Next Updates

When a new ticket lands, add:

- the lifecycle stage where the rule first mattered
- the exact rule clarified
- whether it points to a doc gap, model gap, API gap, test gap, or observability gap
