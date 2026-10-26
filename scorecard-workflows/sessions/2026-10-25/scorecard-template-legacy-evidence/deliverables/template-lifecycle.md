# Template Lifecycle

**Created:** 2026-05-17  
**Status:** First populated pass  
**Purpose:** Describe how a scorecard template moves from authoring through revisioned runtime use

## Why This Document Exists

The scorecard lifecycle explains the runtime artifact. This document explains the reusable definition that the runtime artifact depends on.

Template bugs often come from mixing together:

- edit-time behavior in the builder
- persisted canonical shape
- revision boundaries
- runtime use by scorecards
- operational scope such as audience, permissions, and use case applicability

## Core Mental Model

A template is a versioned business definition, not just a UI form.

It has to work across multiple phases:

1. authoring and editing
2. normalization and save
3. revisioned persistence
4. assignment and applicability
5. runtime use by scorecards
6. historical interpretation after later edits

The main rule is:

- a template can evolve
- a historical scorecard must still be interpreted against the correct template semantics

## Lifecycle Overview

1. Template is created as a reusable evaluation definition
2. Template is edited in the builder
3. Builder state is normalized into persisted API shape
4. Template revision is persisted in authoritative storage
5. Template is assigned or scoped to real business contexts
6. Template is used at runtime by scorecards and scoring flows
7. Template may later be edited again, creating new semantics for future use
8. Historical scorecards continue to depend on the effective older revision

## Stage 1: Creation

### What happens

A template is first created as a business object that defines:

- what gets evaluated
- how it gets scored
- where it is applicable
- who can use it

### Main concerns

- basic structure exists
- title and business meaning are clear
- template type is correct
- use-case scope is coherent

### Common risks

- starting with incomplete structure
- mixing conversation and process assumptions
- unclear ownership of audience and permission semantics

## Stage 2: Authoring / Editing

### What happens

Admins or internal users edit the template in the builder.

This includes:

- chapters and criteria
- criterion types
- options and score mappings
- N/A behavior
- AutoQA mappings
- access and audience settings

### Important distinction

Builder form state is not always the same as the canonical persisted model.

Some edit-time transforms intentionally optimize editing behavior and temporarily change field meaning.

### Common risks

- assuming form-state fields mean the same thing as persisted fields
- overlooking criterion-type-specific behavior
- forgetting that option order and option identity are not always the same

## Stage 3: Save / Normalization

### What happens

The builder transforms the editable form state into the canonical API/persisted shape.

This is where:

- temporary edit-time meanings are normalized
- options may be renumbered or reindexed
- score mappings are serialized into the persisted shape
- some criterion types may follow special-case save behavior

### Why this stage matters

Many “template bugs” are really normalization bugs. The builder may look right, but the persisted meaning may differ.

### Common risks

- pass-through paths that skip normalization
- outcome criteria behaving differently from ordinary criteria
- AutoQA mappings staying internally consistent while no longer matching UI assumptions

## Stage 4: Revisioned Persistence

### What happens

The template is stored authoritatively in Postgres with:

- a stable template identity
- an immutable revision boundary
- persisted structure and operational metadata

### Why this stage matters

Revision is the core historical-control mechanism.

Without a clear revision boundary:

- runtime scorecards can be misinterpreted
- rescoring and replay become ambiguous
- current template state can accidentally overwrite historical meaning

### Common risks

- unclear distinction between metadata changes and semantic changes
- treating template identity as sufficient without revision awareness

## Stage 5: Assignment / Applicability

### What happens

The template becomes applicable in real business contexts through:

- use-case association
- audience configuration
- permission rules
- active/inactive/archive state

### Why this stage matters

Templates are not only configuration for scoring. They are operationally scoped business objects.

### Common risks

- correct template semantics but wrong audience/applicability behavior
- scope bugs being misdiagnosed as scoring bugs

## Stage 6: Runtime Use by Scorecards

### What happens

Runtime scorecards depend on template semantics for:

- criterion structure
- option wiring
- value-to-score mapping
- AutoQA interpretation
- aggregation logic

### Why this stage matters

This is the bridge from reusable definition to concrete business output.

### Common risks

- runtime consumers assuming the wrong representation of a field
- using current template assumptions when the scorecard actually depends on an older revision

## Stage 7: Later Edits and Forward Evolution

### What happens

The template may be edited again to support:

- changed product requirements
- corrected scoring behavior
- new AutoQA behavior
- clearer operational scoping

These edits should affect future runtime usage, but not silently rewrite historical meaning.

### Common risks

- assuming latest template semantics should apply retroactively
- hidden compatibility issues between old and new representations

## Stage 8: Historical Interpretation

### What happens

Old scorecards, analytics rows, and debugging workflows need to interpret historical data correctly even after the template has changed.

### Why this stage matters

This is where revision really proves its value.

Historical correctness depends on:

- using the right revision
- respecting old option wiring and score mapping
- not reinterpreting old values through new assumptions

### Common risks

- index-based UI assumptions on historical stored values
- mislabeling historical values after builder/save-model changes

## Main Failure Classes

### Authoring / save-model mismatch

- builder form looks correct
- persisted template meaning differs

### Revision mismatch

- scorecard is interpreted with the wrong template revision

### Applicability mismatch

- template is valid but used in the wrong business scope

### Representation mismatch

- option identity, option order, and score meaning are confused across layers

### Historical interpretation mismatch

- stored scorecard values are displayed or aggregated using current rather than historical semantics

## Current Best Working Model

If I compress the template lifecycle into one paragraph:

A template is a reusable, revisioned business definition that is authored in an editing model, normalized into a canonical persisted model, scoped operationally through audience and permissions, consumed at runtime by scorecards and scoring logic, and then evolved over time while older scorecards continue to depend on the exact historical semantics of their effective revision.
