# Engineering Plan: Template Schema Versioning and Updater

**Created:** 2026-05-25
**Status:** Draft
**Audience:** engineering, product-adjacent technical planning, future implementation work

## Executive Summary

The template JSON stored in the database evolves over time as new scorecard/template features are introduced. Today the compatibility model is implicit: old templates are handled by scattered migration logic, special-case parsing, and assumptions embedded in different parts of the frontend and backend.

The proposal is to make this explicit:

- add a schema version to the template structure
- define sequential updater functions between versions
- normalize old template shapes into the latest schema before the rest of the system uses them

This does not need to start as a big rewrite. The first useful milestone is smaller:

- define the version field
- define the updater contract
- implement updater execution in one canonical load path
- make the rest of the system consume the normalized latest schema

## Problem

We keep adding features to template structure, but the stored JSON does not have an explicit schema-versioning model with a deliberate updater chain.

Current consequences:

- compatibility logic is scattered
- old migrations become hard to discover and audit
- new features must reason about several historical shapes
- frontend and backend can diverge on how old templates should be interpreted
- removing old compatibility code is risky because there is no clear lifecycle for schema support

This is especially problematic in a revisioned domain where historical correctness matters.

## Goals

1. Make template-JSON evolution explicit instead of implicit.
2. Create one clear path to convert older schema versions into the latest supported schema.
3. Reduce the number of historical shapes that normal feature code needs to handle.
4. Make future template changes cheaper by requiring a version bump and updater strategy.
5. Preserve historical correctness while avoiding a one-time global rewrite requirement.

## Non-Goals

1. Do not redesign the entire template domain.
2. Do not immediately rewrite every historical template row in the database.
3. Do not solve the broader concern that evaluation rules and permission rules may want different revision semantics.
4. Do not require all services to migrate simultaneously in phase 1.

## Proposed Model

## 1. Add explicit schema version to template JSON

Each persisted template structure should carry a version field, for example:

```json
{
  "version": 5,
  "items": [...],
  "shouldDisplayCommentField": true
}
```

This version is about the **schema shape of the JSON**, not the business revision of the template record.

Important distinction:

- `revision` = immutable business revision of a template record
- `version` = schema version of the JSON structure inside that revision

Those are different concepts and should stay separate.

## 2. Define sequential updater chain

Updater functions should be explicitly defined as:

- `v1 -> v2`
- `v2 -> v3`
- `v3 -> v4`
- ...

Each updater:

- accepts one exact schema version
- returns the next exact schema version
- is deterministic and side-effect free
- is testable in isolation

This avoids “magic convert anything to latest” code that becomes impossible to reason about.

## 3. Normalize to latest at load boundary

When a template is read from storage, the canonical load path should:

1. inspect its schema version
2. apply updater functions sequentially until latest
3. return the latest normalized shape to downstream callers

That means most downstream code only handles the latest schema.

## 4. Save only latest schema

The frontend editor and primary write paths should save only the latest schema version.

That creates a one-way pressure toward convergence:

- old templates are upgraded on read
- new writes persist latest schema

## Architecture Direction

## Canonical compatibility boundary

The cleanest initial boundary is:

- **frontend loads template from API**
- **frontend applies updater chain before rendering editor**
- **frontend editor only understands latest schema**

Why this is a good first boundary:

- the concern is strongest in builder/editing paths
- many existing shape transformations already live near frontend editing flow
- it reduces UI complexity first, where historical shapes are costly

However, the end-state should not rely on frontend only.

Longer-term, backend also needs a canonical normalization utility because:

- backend scoring and historical interpretation depend on template meaning
- multiple consumers may read template JSON outside the builder
- FE-only normalization does not fully protect service-side semantics

So the recommended direction is:

### Phase 1

- make versioning explicit
- implement updater chain in the primary FE template load path
- define latest-schema contract clearly

### Phase 2

- add a backend normalization library for service-side consumers
- migrate important backend template-entry points to use it

## Design Details

## Version contract

Suggested rules:

- schema version must be required for all newly written templates
- missing version is treated as legacy and mapped to a well-defined assumed base version
- updater chain must be able to start from that legacy baseline

The sharp edge here is legacy data. We need one explicit decision:

- choose what “missing version” means

Recommended first decision:

- treat missing version as `v1` or `legacy_v0`
- make that mapping explicit in code and tests

## Updater contract

Each updater should:

- only know about one input version
- produce one output version
- preserve business meaning
- fill defaults explicitly where needed
- rename, reshape, or normalize fields as required

Each updater should not:

- depend on ambient UI state
- silently skip incompatible input
- mix multiple version jumps in one function

## Validation contract

After normalization to latest schema, run validation against the latest-schema contract.

This gives two clean failure classes:

- updater failure: old shape cannot be safely converted
- validation failure: converted shape is still not a valid latest template

## Migration Strategy

## Phase 0: Planning and inventory

- inventory known historical schema differences
- define a first concrete version ladder
- identify canonical load/save paths in FE and BE

Output:

- version matrix
- first updater list
- decision on missing-version baseline

## Phase 1: FE-first normalization

- add version field to latest saved template structure
- implement updater registry in frontend
- normalize templates before builder rendering
- make builder operate only on latest schema
- save only latest schema

Output:

- editor no longer needs to branch on old template shapes directly

## Phase 2: Backend normalization utility

- implement canonical backend normalization package
- use it in major backend entry points that parse template JSON
- align BE tests with FE version ladder

Output:

- fewer scattered service-side legacy code paths

## Phase 3: Optional persistence cleanup

- backfill or lazy-rewrite old templates to latest schema when touched
- optionally add offline migration for heavily used old templates

Output:

- shrinking amount of legacy-version data in storage

This phase is optional at first. It should not block the value of phases 1 and 2.

## Testing Plan

## Unit tests

- each updater transforms exact input version to exact next version
- missing-version legacy templates map to expected baseline
- latest-schema validation passes after full upgrade chain

## Golden fixtures

- store example templates from several historical shapes
- verify full upgrade output matches expected latest schema

## Round-trip tests

- old template -> normalize to latest -> render/edit/save -> persisted as latest

## Semantic regression tests

- ensure updater preserves key business meaning:
  - option wiring
  - score mapping
  - AutoQA mapping
  - N/A semantics
  - criterion hierarchy

## Risks

## 1. Schema version and template revision get conflated

Mitigation:

- document the distinction explicitly
- use naming that keeps them separate

## 2. FE and BE version ladders diverge

Mitigation:

- define one shared version matrix
- add fixture-based parity checks where possible

## 3. Updaters accidentally change business meaning

Mitigation:

- require semantic regression fixtures
- review updater logic as business-rule code, not just shape conversion

## 4. Legacy templates without version are ambiguous

Mitigation:

- explicitly define missing-version baseline
- document known assumptions and exceptions

## 5. Rollout becomes too big

Mitigation:

- keep phase 1 FE-first
- defer bulk data rewrite
- normalize at load boundary instead of forcing immediate DB migration

## Open Questions

1. What is the exact known historical schema ladder today?
2. Is missing version best treated as `legacy_v0` or `v1`?
3. Which template consumers outside the builder must normalize in phase 2?
4. Should backend eventually reject writes that are not latest schema?
5. Which parts of template structure are truly schema evolution versus business-rule revisioning?

## Recommended First Implementation Slice

The smallest slice that proves the idea:

1. define `latestTemplateSchemaVersion`
2. define `legacy/no-version` baseline
3. implement one or two real updater steps for known historical shapes
4. run updater chain in the FE template-load path
5. make editor save only latest version
6. add golden fixtures for old templates

This gives real leverage without requiring immediate cross-system migration.

## Suggested Deliverables After This Plan

- a version matrix doc with known historical shapes
- an updater registry design note
- a fixture set of old templates
- a follow-up backend normalization plan
