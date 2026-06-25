# ALOs in Coaching

**Created:** 2026-06-17
**Status:** Design review
**Ticket:** [CONVI-7071](https://linear.app/cresta/issue/CONVI-7071/alos-in-coaching)

## Objective

Review the engineering design for bringing agent-level outcomes (ALOs) into the Coaching suite.

The current design proposes reusing existing coaching target infrastructure by hosting ALO targets under empty scorecard templates with a new `USER_OUTCOME` template type. ALO values are agent-global, while ALO targets are scoped by use case.

## Key Artifacts

- `engineering-design-review-guide.md`
