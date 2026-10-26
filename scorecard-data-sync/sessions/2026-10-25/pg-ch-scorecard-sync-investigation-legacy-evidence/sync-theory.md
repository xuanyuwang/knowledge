# Sync Theory

## Purpose

Build a theory-first framework for reasoning about synchronization between a transactional source database and a derived analytics database.

## Scope

This document intentionally ignores current scorecard implementation details at first. It focuses on general patterns that should hold for any mutable entity synchronized from PostgreSQL-like OLTP storage into ClickHouse-like OLAP storage.

## Key Questions

- What does “in sync” actually mean?
- Which consistency guarantees are necessary versus nice to have?
- What failure modes exist even when both databases are individually healthy?
- Which classes of problems require prevention, and which can be handled by reconciliation?

## The Core Mental Model

Two-database sync is not “copy rows from A to B.” It is a projection problem.

- The **source database** owns the authoritative business state.
- The **sink database** owns a derived representation optimized for a different workload.
- Sync is the mechanism that keeps the sink projection convergent with the source state.

That means the first question is not “did the write succeed?” It is:

**What exact source state should win in the sink, by what rule, and how do we know the sink has converged to it?**

If that rule is ambiguous, the system will drift even when every component is “working.”

## 1. Define the Sync Contract

Before choosing architecture, define the contract explicitly.

### 1.1 Source of truth

One system must be authoritative for each fact.

- If PostgreSQL is authoritative for scorecard state, then ClickHouse must be treated as a projection, not a peer.
- If the sink can also mutate the same business fields independently, the problem becomes bidirectional replication, which is much harder.

For this investigation, the relevant theoretical model is:

- **Unidirectional sync**
- **Authoritative mutable source**
- **Derived read-optimized sink**

### 1.2 What “in sync” means

There are several possible definitions, and they are not equivalent.

- **Existence sync**: every source entity exists in the sink
- **Field sync**: every materialized field matches the correct source-derived value
- **Version sync**: the sink reflects the latest valid source version
- **Aggregate sync**: counts and rollups match, even if some entity rows are wrong
- **Temporal sync**: the sink catches up within an acceptable lag window

The trap is treating aggregate sync as proof of field sync. Counts can match while individual entities are stale or malformed.

### 1.3 Freshness vs correctness

These are separate dimensions.

- **Freshness** asks how quickly the sink reflects source changes.
- **Correctness** asks whether the sink eventually reflects the right state.

A system can be:

- fresh but wrong: low lag, stale overwrite wins
- correct but slow: eventual convergence after minutes or hours
- neither: silent failures plus no repair path

In OLTP → OLAP pipelines, eventual correctness usually matters more than immediate read-after-write consistency, but that only works if convergence is provable.

### 1.4 Entity-level vs projection-level correctness

The sink often does not store a raw copy of source rows. It stores a projection that may join, flatten, denormalize, enrich, or aggregate.

So the contract should be:

- which source entities contribute to the sink row
- which transformations are deterministic
- which external lookups are required
- which fields are lossless copies versus derived fields

Without this, “mismatch” is underspecified.

## 2. Common Sync Strategies

There are five standard patterns. Most real systems use a hybrid.

### 2.1 Application dual write

The application writes source state and sink projection in the same request path.

**Strengths**

- Simple mental model
- Low propagation latency
- Easy to start with

**Weaknesses**

- Hard to make atomic across heterogeneous stores
- Easy to get partial success
- Ordering races appear when multiple async writes are launched from separate request paths
- Retries often create duplicates or stale overwrites unless versioning is explicit

This pattern is operationally cheap at small scale and deceptively dangerous once entities become mutable.

### 2.2 Transactional outbox + async consumer

The application commits the source write and an outbox event in the same source transaction. A separate consumer reads the outbox and updates the sink.

**Strengths**

- Removes “source committed, event lost” class of failures
- Gives a durable replay log
- Decouples business transaction from sink availability

**Weaknesses**

- Still requires ordering, idempotency, and replay correctness in the consumer
- Can propagate bad or incomplete events if the event schema is underspecified
- Adds operational components

This is often the best tradeoff when the source database is clearly authoritative.

### 2.3 CDC from source log

The sink is updated from the source database change log rather than application-emitted events.

**Strengths**

- Captures all committed source mutations
- Reduces dependence on application code paths
- Good for broad coverage and replay

**Weaknesses**

- Raw row changes may not map cleanly to business-level projection updates
- Derived projections still need enrichment and join logic
- Delete semantics and ordering across tables can be subtle

CDC is strongest when the projection can be rebuilt deterministically from committed source data.

### 2.4 Periodic snapshot / reconciliation

A recurring job compares source and sink, then repairs divergence.

**Strengths**

- Catches silent gaps missed by real-time paths
- Can be simple to reason about
- Useful as a backstop even when primary sync is real time

**Weaknesses**

- Detection may be delayed
- Range-based scans can be expensive
- Repair selection logic may miss edge cases
- Cannot always reconstruct missing context if the source no longer retains it

Reconciliation is usually necessary, but rarely sufficient by itself.

### 2.5 Hybrid model

Most robust systems use:

- one real-time projection path
- one durable replay path
- one periodic reconciliation path

The point is not redundancy for its own sake. It is to cover different failure classes:

- real-time path for freshness
- replay path for durability
- reconciliation path for blind-spot detection

## 3. Canonical Guarantees

These guarantees matter more than the transport mechanism.

### 3.1 Delivery guarantee

Questions:

- Can a committed source mutation fail to produce any sink update attempt?
- Can an update attempt fail without durable evidence?
- Can failed updates be replayed automatically?

At-least-once delivery is usually acceptable if updates are idempotent and correctly versioned. Exactly-once is rarely necessary and often impractical across heterogeneous systems.

### 3.2 Ordering guarantee

For mutable entities, ordering is usually the hardest problem.

If version `N+1` is followed by a delayed write for version `N`, the sink may regress unless it has a rule that rejects older state.

Ordering can be enforced by:

- serial processing per entity key
- monotonic version numbers
- sink-side last-write-wins using a correct version field
- compare-and-swap semantics

If the chosen ordering signal is not the true business version, the sink can consistently pick the wrong winner.

### 3.3 Idempotency

Retries are unavoidable. An update must be safe to apply multiple times.

Idempotency requires:

- stable entity identity
- stable version identity
- deterministic projection logic
- sink upsert semantics that do not double-apply side effects

“Insert again and hope the engine merges it later” is not a complete idempotency strategy unless merge semantics are proven to match business intent.

### 3.4 Monotonic versioning

Every mutable entity projection needs a single rule for which version wins.

Good version candidates are:

- source transaction sequence
- source commit timestamp plus tie-breaker
- explicit incrementing version
- immutable event sequence number

Bad version candidates are usually:

- worker execution time
- retry time
- ingestion time in the sink

Those measure delivery timing, not source truth.

### 3.5 Replay safety

A healthy sync system assumes replay will happen.

Replay is safe only if:

- old events can be recognized as old
- projection code is deterministic for a given source version
- duplicate writes do not change outcome
- repair writes do not regress newer sink state

### 3.6 Delete semantics

Deletes are often the first place theory and implementation drift apart.

Possible source meanings:

- hard delete
- soft delete
- filtered-out entity
- entity that should never have been projected

The sink contract must specify whether delete means:

- physically remove row
- write tombstone
- mark `_row_exists = 0`
- ignore because downstream queries exclude it

Unclear delete semantics produce “ghost data” problems that backfills do not fix.

## 4. Failure Taxonomy

This taxonomy is the main output of the theory phase. It gives names to distinct divergence classes.

### 4.1 Omission failures

The correct sink update never materializes.

Subtypes:

- source write committed, no sink attempt made
- sink attempt made, failed silently
- event emitted, never consumed
- reconciliation ran, but selection logic excluded the entity

Observable symptom:

- entity missing from sink

### 4.2 Staleness failures

The sink contains the entity, but not the latest valid state.

Subtypes:

- delayed older update overwrote newer state
- partial retry re-projected stale source snapshot
- merge engine chose wrong winner due to wrong version column

Observable symptom:

- counts may match, but fields do not

### 4.3 Partial projection failures

Only part of the entity projection was updated.

Examples:

- parent row exists, child rows missing
- some derived fields refreshed, others stale
- enrichment data missing or from the wrong version

Observable symptom:

- impossible field combinations
- entity exists but business semantics are broken

### 4.4 Coverage failures

The repair or backfill mechanism exists, but its selection logic does not cover all valid entities.

Examples:

- time-range query based on the wrong timestamp
- recovery path assumes conversation-backed entities only
- filter excludes entities with empty foreign keys

Observable symptom:

- system appears repairable in theory, but some mismatches survive every repair run

### 4.5 Semantic mismatch failures

The sink engine’s merge/upsert semantics do not match business semantics.

Examples:

- “latest ingestion time wins” but business truth is “latest source version wins”
- duplicate rows are merged eventually, but queries observe intermediate wrong state
- tombstones do not dominate prior rows

Observable symptom:

- repeated repairs produce inconsistent outcomes
- correctness depends on timing rather than source version

### 4.6 Observability failures

Divergence exists, but monitoring cannot localize or classify it.

Examples:

- counts differ but IDs are unknown
- IDs are known but field mismatch is not measurable
- repair runs are triggered but their effectiveness is not verified

Observable symptom:

- long-lived incidents with weak root-cause confidence

## 5. Prevention, Detection, Repair

These are three different system responsibilities.

### 5.1 Prevention

Prevention reduces the probability of divergence.

Examples:

- eliminate dual-write races
- use source-derived monotonic versions
- make writes idempotent
- emit durable change records transactionally

Prevention is the only durable answer for systematic stale-overwrite bugs.

### 5.2 Detection

Detection answers:

- did divergence happen?
- how much?
- which entities?
- of what class?

Detection must become progressively more precise:

- aggregate counts
- entity IDs
- field-level mismatches
- mismatch classification

Count-only monitoring is useful, but it cannot by itself drive reliable repair.

### 5.3 Repair

Repair restores convergence after divergence already exists.

Repair mechanisms include:

- replay event
- re-project single entity by ID
- backfill by time range
- rebuild projection from authoritative source

Repair is only reliable when it uses the same winning-version rules as prevention.

## 6. What Good Looks Like

A sound OLTP → OLAP sync system usually has the following properties:

- The source of truth is unambiguous.
- The winner for a mutable entity version is unambiguous.
- Sink writes are idempotent.
- Old writes cannot overwrite newer source truth.
- Silent failures are detectable.
- Detection can localize exact entity IDs.
- Repair can reconstruct the correct projection for every supported entity class.
- Repair effectiveness is measured, not assumed.

If any one of those is missing, the system may still work most of the time, but it is not robust.

## 7. Practical Heuristics

When investigating a sync issue, ask these in order:

1. What exact source state should be present in the sink?
2. What source version signal defines “latest”?
3. Could an older update arrive after a newer one?
4. If yes, what prevents regression?
5. Could a committed source change produce no durable projection attempt?
6. If yes, how is that detected?
7. If divergence is detected, can we identify exact entities?
8. If entities are known, can we fully reconstruct their projection from source truth alone?
9. If not, what transient context is missing?
10. Are we dealing with prevention failure, detection failure, repair failure, or multiple at once?

This sequence usually separates root cause from symptom quickly.

## Working Definitions

### Convergence

The sink eventually represents the latest valid source state for the entity, using an agreed versioning rule.

### Divergence

The sink is missing the entity, contains stale fields, contains impossible combinations of fields, or reflects a version that should no longer win.

### Repairability

A divergence is repairable if the source still has enough information to reconstruct the correct sink representation without relying on lost transient context.

### Version winner

The deterministic rule that decides which of multiple candidate updates represents the correct sink state for an entity.

### Sync contract

The explicit definition of what data should flow from source to sink, with what freshness target, versioning rule, and correctness criteria.
