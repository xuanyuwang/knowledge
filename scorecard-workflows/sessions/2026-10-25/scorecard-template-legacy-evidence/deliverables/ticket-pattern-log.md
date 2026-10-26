# Scorecard / Template Ticket Pattern Log

**Created:** 2026-05-17  
**Status:** Seeded starter log  
**Purpose:** Capture repeated patterns across ticket work so local fixes accumulate into domain insight

## How To Use This Document

Each entry should separate:

- the local issue
- the repeated domain pattern behind it
- the class of system weakness it revealed

This log is meant to grow from real work. The goal is not exhaustive history. The goal is to make repeated pain visible.

## Entry Template

### Ticket / issue

- Local issue:
- Lifecycle stage:
- Concepts involved:
- What looked local at first:
- Repeated pattern underneath:
- Rule clarified:
- Category: doc gap / naming gap / model gap / API gap / test gap / observability gap / ownership gap
- Candidate small improvement:

## Seeded Examples

### CONVI-6709 reversed scorecard labels

- Local issue: scorecard panel displayed reversed predicted-resolution and predicted-CSAT labels
- Lifecycle stage: historical querying and display
- Concepts involved: template option wiring, scorecard stored `numeric_value`, read-only UI rendering, decoupled scoring representation
- What looked local at first: a frontend label-rendering bug in the scorecard panel
- Repeated pattern underneath: the system repeatedly confuses array index, option identity, and score meaning across representations
- Rule clarified: historical scorecard display must treat stored values as option identities, not assume display array index is the same thing
- Category: model gap
- Candidate small improvement: define and enforce a single helper contract for option identity lookup across read-only and editable UI paths

### N/A score design work

- Local issue: customers wanted N/A to carry a score instead of always being excluded
- Lifecycle stage: score semantics computation
- Concepts involved: N/A semantics, option wiring, AutoQA mapping, analytics projection
- What looked local at first: add one new scoring feature for N/A
- Repeated pattern underneath: “minor” criterion settings often affect multiple layers at once: builder UI, runtime score storage, backend calculation, and analytics interpretation
- Rule clarified: N/A is a scoring rule with lifecycle consequences, not just a grader UI affordance
- Category: model gap
- Candidate small improvement: maintain a checklist for any criterion-setting change that touches authoring, stored runtime values, scoring, and analytics

### Process scorecards missing from reindex flow

- Local issue: process scorecards were not reindexed into ClickHouse by the existing conversation-based workflow
- Lifecycle stage: repair / reindex / backfill
- Concepts involved: scorecard type, recovery path, ClickHouse projection
- What looked local at first: missing support in one reindex job
- Repeated pattern underneath: many operational paths quietly assume all scorecards are conversation-centric
- Rule clarified: scorecard lifecycle and recovery design must treat process scorecards as first-class, not as an edge case hidden behind conversation flows
- Category: model gap
- Candidate small improvement: classify every operational workflow explicitly by supported scorecard types

### percentage_value bug after analytics-path change

- Local issue: some analytics scores rendered as 10000% because the new projection path computed `percentage_value` incorrectly
- Lifecycle stage: analytics projection
- Concepts involved: raw `numeric_value`, mapped score, max score, derived analytics fields
- What looked local at first: one arithmetic bug in ClickHouse row building
- Repeated pattern underneath: direct projection rewrites are dangerous when they bypass canonical scoring semantics and reimplement business rules ad hoc
- Rule clarified: analytics projection must reuse authoritative score semantics, not re-derive them with simplified assumptions
- Category: test gap
- Candidate small improvement: add semantic parity tests between authoritative scoring logic and analytics projection logic

### Missing historic / ClickHouse scorecards due to async races

- Local issue: scorecards were present in authoritative Postgres state but missing or stale downstream
- Lifecycle stage: authoritative persistence and analytics projection
- Concepts involved: update/submit ordering, async work, source-of-truth boundary, repair flows
- What looked local at first: intermittent sync failures
- Repeated pattern underneath: the domain repeatedly suffers when derived systems are allowed to race on stale snapshots instead of re-reading authoritative state
- Rule clarified: downstream sync must derive from the latest authoritative state, not from closure-captured state tied to API timing
- Category: model gap
- Candidate small improvement: document and standardize the “authoritative write first, async re-read later” pattern for scorecard-related projection

## Pattern Categories Emerging So Far

### Representation mismatch

- The same logical value means different things in builder form state, persisted template JSON, stored score rows, and UI display code.

### Conversation-centric assumptions

- Scorecard logic often assumes conversation-based flow even when process scorecards need equal support.

### Reimplemented semantics

- Bugs appear when a downstream path re-implements score meaning instead of reusing canonical scoring logic.

### Hidden multi-layer impact

- Seemingly small criterion-setting changes often affect authoring, runtime storage, scoring, display, and analytics all at once.

## Small-Improvement Backlog

- Write a compact “representation boundaries” note that names what `option.value`, `scores[].value`, `scores[].score`, and `numeric_value` mean in each layer.
- Define a reusable parity-testing strategy between authoritative scoring logic and analytics projection.
- Create an explicit scorecard-type support matrix for operational workflows.
- Create a checklist for any change that touches criterion semantics across UI, storage, scoring, and analytics.
