# Scorecard & Template Working Reference

**Created:** 2026-05-17  
**Status:** Active  
**Purpose:** Project brief and execution plan for building the living working reference

## Goal

Build a living working reference for scorecard and template in the coaching service.

The goal is not to write perfect documentation. The goal is to create the best working map of the domain so future investigation, implementation, debugging, and product discussion do not restart from zero.

## Why This Project Exists

Scorecard/template work repeatedly exposes the same problem:

- business rules are real and often critical
- many of those rules are scattered across code, UI behavior, historical decisions, tickets, and tribal knowledge
- each feature or bug contains both a local issue and a repeated domain pattern

Without a working reference, the team pays repeated costs:

- rediscovering the same rules
- using inconsistent terminology
- misunderstanding which representation is canonical at a given stage
- missing small but meaningful opportunities for system improvement

## Scope

The working reference should cover:

- core concepts and relationships
- main lifecycle flows
- important business rules and invariants
- historical decisions and legacy behavior that still matter
- system/code entry points
- known ambiguities, sharp edges, and recurring pain points

## Outputs

The project should produce:

1. A concept map of the domain
2. A glossary of important terms
3. A lifecycle and flow document
4. A business-rules catalog
5. A known-sharp-edges section
6. A ticket learnings log
7. A short list of incremental improvement opportunities

## Starting Principle

Do not start by trying to enumerate every business rule.

That approach is too large and provides no framework for placing new knowledge. Start with a domain skeleton first, then let the rule catalog grow from real work.

## Starting Point: Domain Skeleton

The first version of the working reference should answer only four questions:

1. What are the core concepts?
2. How do they relate?
3. What are the main lifecycle transitions?
4. Where do the most important rules show up?

That is the minimum viable structure that makes the rest of the domain capturable.

## Suggested Core Concepts

Start with a small set of nouns:

- Scorecard
- Template
- Criterion
- Option
- Score
- Assignment
- Version / revision
- Evaluation context

This list does not need to be perfect on day one. It is a scaffolding tool.

## Rule-Collection Framework

Capture rules by lifecycle and by bucket, not as a flat list.

Suggested buckets:

- Creation rules
- Edit rules
- Versioning rules
- Assignment rules
- Scoring rules
- Visibility and access rules
- Historical consistency rules
- Migration and backward-compatibility rules

## Working Method

When a new ticket touches this area, add a small entry with:

- Local issue
- Concepts involved
- Lifecycle stage involved
- Rule discovered or clarified
- Edge case or historical constraint
- Where the knowledge currently lives
- Whether it should become part of the reference

This keeps the reference tied to real work instead of turning it into a side documentation project.

## Incremental Improvement Lens

The purpose of the reference is not only understanding. It is also to expose low-cost improvements.

Recurring patterns should be classified into categories such as:

- doc gap
- naming gap
- model gap
- API gap
- test gap
- observability gap
- ownership gap

Those categories make it easier to propose small, practical improvements instead of large rewrites.

## First Milestones

1. Create the domain skeleton.
2. Identify the top rule buckets.
3. Add the first set of open questions.
4. Use the next 3 to 5 tickets to enrich the skeleton rather than creating ad-hoc notes.
5. Review the accumulated notes and extract the first recurring patterns.

## Definition of Success

This project is succeeding if:

- future scorecard/template work starts from a shared map instead of a blank investigation
- repeated confusion starts landing in stable categories
- the team can discuss the domain with clearer terms
- at least a few small, high-leverage improvements become obvious and actionable
