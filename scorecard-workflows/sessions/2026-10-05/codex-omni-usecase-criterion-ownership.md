# Omni-use-case criterion ownership and queryability

- Date: 2026-10-05
- Primary subdomain: template-authoring-and-versioning
- Source repo: `/Users/xuanyu.wang/repos/go-servers` (`main`, read-only)
- Product input: Omni Use Case QM / Coaching PRD, linked from the engineering-plan draft; Xuanyu's clarification on 2026-10-05

## Decision input

Xuanyu confirmed that this project does not require a criterion to be shared by different templates or independently versioned. A criterion remains owned by one template, which can span multiple use cases. The interest in an independent criterion resource was mainly to make criterion definitions directly searchable and database operations easier.

## Schema check

`apiserver/sql-schema/director/director-schema.sql` defines `director.scorecard_templates.template` and `director.scorecard_template_revisions.template` as JSONB. It has no criterion-definition table or index on criterion definitions within those JSONB columns. `ix_scores_cp_criterion` indexes evaluated `director.scores` by `(customer, profile, criterion_identifier)`; it does not index template definitions. The schema does not by itself prove query performance or deployed indexes.

## Plan implication

Keep criterion ownership and revision semantics under the template. Treat direct criterion lookup as a separate storage/queryability requirement. Inventory lookup and mutation patterns before choosing a template-owned relational model, revision-keyed projection, or JSON query strategy. The older independent-resource initiative's actual design and outcome remain unverified.
