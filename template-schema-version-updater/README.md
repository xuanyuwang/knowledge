# Template Schema Versioning and Updater

**Created:** 2026-05-25
**Updated:** 2026-05-25

## Overview

This project captures the proposal to add an explicit schema version and sequential updater pipeline for the template JSON stored in the database.

The current system keeps evolving the template JSON shape as features are added. Migration logic exists, but it is scattered across frontend and backend behavior, and old compatibility code can become hard to reason about once most templates have already moved to newer shapes.

The core idea is:

- store an explicit schema version with the template structure
- define sequential updaters between versions
- normalize older templates into the latest schema before the rest of the system works with them

## Current Objective

Turn the concern into a concrete engineering plan that can start small:

- define the versioning model
- define where updater logic should live first
- define compatibility boundaries
- define rollout and migration strategy

## Scope

In scope:

- template JSON schema versioning
- sequential updaters between schema versions
- runtime normalization to latest schema
- compatibility strategy for FE and BE
- migration and rollout plan

Out of scope for the first phase:

- rewriting all historical templates in place immediately
- redesigning template revision semantics
- unifying evaluation-rule and permission-rule revisioning
- broad template-domain refactors unrelated to versioning

## Key Problem Statement

Today template evolution has these costs:

- old-shape handling is scattered and hard to audit
- compatibility logic becomes stale after old templates largely disappear
- new features have to reason about multiple historical shapes
- frontend and backend risk drifting in how they interpret old templates

An explicit schema version plus updater chain would make the compatibility model more deliberate.

## Status

Active planning

## Related Context

- Concern source: [`scorecard-template/deliverables/concerns.md`](../scorecard-template/deliverables/concerns.md)
- Related working-reference project: [`scorecard-template/README.md`](../scorecard-template/README.md)

## Related Artifacts

- `project.yaml`
- `deliverables/eng-plan.md`
- `log/2026-05-25.md`
