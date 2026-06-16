# Codex Session: Blog Draft on Artifacts and Workflows

**Date:** 2026-06-15
**Tool:** Codex
**Source repo:** /Users/xuanyu.wang/repos/knowledge
**Knowledge repo:** /Users/xuanyu.wang/repos/knowledge
**Project:** train-for-staff

## Prompt

Draft a blog candidate from the scorecard/template modeling shift:

- earlier confusion came from focusing on scorecard code and generic APIs
- the product originally had one dominant evaluation workflow
- later workflows such as calibration, group calibration, and appeal made generic APIs like `updateScorecard`, `submitScorecard`, and `createScorecard` ambiguous
- an object-oriented scorecard-type solution helps but is not complete
- the better framing separates domain artifacts from behavioral frames

## Key Framing

The blog argues that the problem was not only code complexity. The business model had outgrown the code names.

Core distinction:

- **Domain artifacts**: template and scorecard
- **Behavioral frames**: lifecycle and workflow

The draft explains why scorecard-type dispatch is useful but incomplete:

- it keeps the abstraction boundary centered on the artifact
- generic verbs remain ambiguous
- type dispatch can replace rather than solve domain modeling
- workflow rules leak into artifact helpers

## Output

Created `train-for-staff/deliverables/from-scorecard-apis-to-business-workflows.md`.
Published the post to `blog/2026-06-15-from-scorecard-apis-to-business-workflows.md` with the title "From Scorecard APIs to Business Workflows".

The draft proposes a layered design direction:

- artifact primitives for low-level scorecard/template operations
- workflow commands for business actions
- explicit workflow state and audit when the workflow is mature enough

It also lists design options:

- generic APIs with workflow parameters
- scorecard type dispatch
- workflow-specific commands over shared primitives
- full workflow engine or state machine

## Follow-Ups

- Add concrete code examples if the post should become more technical.
- Decide whether to anonymize or generalize product terms before sharing externally.
