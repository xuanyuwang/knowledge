# Scorecard Lifecycle

**Created:** 2026-05-17  
**Status:** First populated pass  
**Purpose:** Describe the runtime lifecycle of a scorecard from creation through analytics projection and recovery

## Why This Document Exists

The template documents explain the reusable definition side of the system. This document explains the runtime artifact side.

Most scorecard bugs are easier to reason about when they are placed in one lifecycle stage:

- instantiation
- scoring
- update
- submission
- authoritative persistence
- analytics projection
- historical recovery or repair

Without that lifecycle view, it is easy to mix together:

- template semantics
- scorecard state changes
- ClickHouse projection behavior
- repair flows and backfills

## The Core Mental Model

A scorecard is a mutable runtime artifact that starts from template semantics, accumulates score inputs, becomes historically meaningful when persisted and often when submitted, and is later projected into analytics storage for reporting.

The most important source-of-truth rule is:

- **Postgres is authoritative for scorecards and scores**
- **ClickHouse is a downstream projection for analytics and reporting**

That means the lifecycle has two distinct tracks:

1. the **authoritative track** in Postgres
2. the **projection track** into ClickHouse

Many historical bugs came from confusing or racing these two tracks.

## Lifecycle Overview

1. Scorecard is instantiated in a concrete evaluation context
2. Score inputs are recorded or updated
3. Score semantics are computed using template rules
4. Scorecard state is persisted in authoritative Postgres tables
5. Scorecard may be submitted or finalized
6. Authoritative data is projected into analytics storage
7. Historical scorecards are queried, displayed, aggregated, or repaired

## Stage 1: Instantiation

### What happens

A scorecard comes into existence for a real runtime context, such as:

- a conversation-based evaluation
- a process scorecard flow
- a coaching or QA workflow
- an AutoQA-triggered scoring path

At this stage, the scorecard stops being “only template configuration” and becomes a concrete business artifact.

### Inputs

- template identity and effective revision
- business context such as conversation or process scope
- operational metadata needed for ownership, customer/profile scoping, and runtime flow

### Key questions

- What parts of template semantics are copied into the scorecard context versus looked up later?
- What fields are mutable after creation?
- Which scorecards are conversation-based versus process-based?

### Common risks

- using the wrong template revision
- assuming all scorecards are conversation-centric
- missing context needed by downstream projection or recovery flows

## Stage 2: Scoring Input Capture

### What happens

Criterion-level outcomes are captured on the scorecard.

Those outcomes may come from:

- manual grading
- AutoQA mapping
- mixed manual and AI-assisted paths

At this stage, the scorecard accumulates raw runtime values such as:

- `numeric_value`
- `ai_value`
- `not_applicable`
- manual override state

### Important distinction

The stored runtime value is often a **lookup key**, not the business score itself.

For example:

- `numeric_value` in Postgres is typically the criterion value or option identity
- the actual score meaning may come later through template score mapping

This is one of the most important sources of confusion in the domain.

### Common risks

- treating stored `numeric_value` as the final score
- mixing manual display semantics with AutoQA semantics
- misreading `option.value` across different representations

## Stage 3: Score Semantics Computation

### What happens

Backend scoring logic turns raw runtime values into meaningful score outputs.

This includes:

- value-to-score mapping
- max-score determination
- percentage-score computation
- weighting
- auto-fail logic
- N/A handling
- branch validation
- per-message or multi-select logic where applicable

### Why this stage matters

This is where template business rules become actual score behavior.

Two scorecards with identical raw values may produce different analytics outcomes if:

- template score mappings differ
- criterion max score differs
- N/A semantics differ
- branch applicability differs

### Common risks

- computing percentages from raw values instead of mapped scores
- using `GetMaxValue()` when business logic requires max mapped score
- failing to validate scores against the active criterion tree
- including N/A rows incorrectly in aggregation

## Stage 4: Authoritative Persistence

### What happens

The scorecard row and its score rows are written to authoritative Postgres storage.

The key mental model is:

- this is the state the application should trust
- downstream systems should be derived from this state, not treated as equal peers

### Why this stage matters

The authoritative store is where correctness must be recoverable.

If analytics projection fails, the system should still be able to rebuild from authoritative Postgres data.

### Common risks

- partial writes across scorecard row and score rows
- stale closure data used by async work
- assuming downstream projection state is authoritative

## Stage 5: Submission / Finalization

### What happens

The scorecard may transition from an in-progress mutable artifact to a submitted or otherwise finalized state.

Submission matters because it often changes the business meaning of the scorecard:

- it becomes a completed evaluation
- it becomes eligible for reporting or historical comparison
- it may trigger downstream sync or analytics relevance

### Important nuance

Scorecards are still a mutable domain in practice. Some scorecards may be updated multiple times before or after submission.

That means “submitted” is important, but it is not the same as “immutably done forever” in every system path.

### Common risks

- submit state in ClickHouse becoming stale relative to Postgres
- assuming monotonic lifecycle where updates can still occur
- racing update and submit flows

## Stage 6: Analytics Projection

### What happens

Authoritative scorecard data is transformed and written into ClickHouse-friendly scorecard and score rows for analytics.

Historically this path has gone through different mechanisms:

- historic-schema based intermediate computation
- direct reconstruction from director data
- scorecard-centric reindex or repair flows

### What this stage produces

Projection rows may contain derived analytics fields such as:

- `percentage_value`
- weight fields
- max value fields
- manual/AI scoring flags
- flattened scorecard rows for reporting

### Source-of-truth rule

Projection is derived. If projection is wrong, the recovery path should start from authoritative Postgres state and template semantics.

### Common risks

- async ordering causes stale projection writes
- derived fields are computed with the wrong semantics
- process scorecards are missed by conversation-centric recovery paths
- projection rows exist but are content-wrong, not just count-wrong

## Stage 7: Historical Querying and Display

### What happens

Once scorecards exist historically, they are:

- displayed in product surfaces
- aggregated in analytics
- used in filtering and reporting
- compared across time
- investigated during bugs or customer escalations

### Why this stage matters

Historical correctness depends on more than row existence.

The system must preserve:

- correct label interpretation
- correct score semantics
- correct submit/finalization state
- correct revision-aware behavior

### Common risks

- using the wrong representation when displaying stored values
- assuming array index equals option identity
- silently interpreting old scorecards using current template assumptions

## Stage 8: Repair, Reindex, and Backfill

### What happens

When authoritative and analytics states diverge, recovery flows may rebuild projection data.

Examples:

- time-range backfills
- scorecard-centric reindex jobs
- customer-specific repair workflows
- historical verification scripts

### Why this stage exists

Projection failures are a normal operational reality in this system. Recovery is part of the lifecycle, not a special case outside it.

### Important insight

Recovery paths are only reliable if they understand the true shape of scorecards:

- some scorecards are not conversation-centric
- not all mismatches are “missing rows”
- some failures are stale or semantically wrong content

### Common risks

- recovery flow only handles conversation scorecards
- projection is rebuilt with incomplete context
- version semantics in ClickHouse do not match application intent

## Main Write Paths

These are the write-path categories that matter for lifecycle reasoning:

- create
- update
- submit
- auto-score
- backfill
- repair / reindex

This list is useful because bugs often affect only some write paths, not the whole lifecycle equally.

## Common Failure Classes By Lifecycle Stage

### Instantiation

- wrong template revision used
- unsupported scorecard type assumed to be conversation-based

### Scoring input capture

- raw stored value misunderstood as business score
- AutoQA mapping writes semantically correct value but UI reads it incorrectly

### Score semantics computation

- wrong percentage calculation
- invalid N/A handling
- branch-excluded scores still included

### Authoritative persistence

- scorecard and score rows diverge
- stale data captured before async write

### Submission / finalization

- submit state races with update state
- projection reflects pre-submit state

### Analytics projection

- missing scorecard row
- missing score rows
- stale submit state
- stale criterion scores
- incorrect derived fields

### Historical display / querying

- wrong labels shown for correct stored values
- current UI assumptions misinterpret historical values

### Repair / reindex

- repair only restores counts, not correct semantics
- process scorecards are skipped by conversation-centric logic

## Questions This Lifecycle Helps Answer

- At which stage does this bug first become true?
- Is the problem in authoritative state, derived state, or interpretation?
- Is this a template-semantic bug or a scorecard-runtime bug?
- Is the issue local to one write path, or common across create/update/submit/reindex?
- Is the failure a missing-row problem, a stale-data problem, or a wrong-meaning problem?

## Current Best Working Model

If I compress the scorecard lifecycle into one paragraph:

A scorecard is instantiated from template semantics in a concrete business context, accumulates raw criterion outcomes from manual and/or automated scoring, relies on backend scoring logic to turn those outcomes into business-meaningful percentages and weights, is stored authoritatively in Postgres, may transition through submission/finalization while remaining operationally mutable in some paths, is projected into ClickHouse for analytics, and may later be repaired or reindexed from authoritative data when projection paths fail or drift.
