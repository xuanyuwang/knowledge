# CONVI-7777 alignment boundary

**Source repo and context:** `/Users/xuanyu.wang/repos/go-servers`, read-only `main`; source checkpoint in the [V1 audit](../../deliverables/convi-7777-assistance-api-semantics-audit.md) is fetched `origin/main` `48d308822e90e17d2ca190987220601601ee9d97`. No source checkout changes.

**User direction:** The key alignment problem is the eligible conversation population. Existing SQL and code are the source of truth for each feature's current count.

**Documentation change:** The audit now separates current per-feature count semantics from the new distinct-conversation union. It recommends a common eligible conversation/agent cohort for numerator and denominator, while preserving the existing feature predicates and leaving current counts unchanged. Aggregate feature totals remain unusable as inputs to the union because their units vary.

**Decision still needed:** Current Summary SQL counts stored types 1/2, whereas the proposed copy-only inventory implies 1/3. Matching the current count means using 1/2; adopting copy-only requires a deliberate existing-RPC change before claiming parity. Notes and Guided Workflow metadata paths also need correction or exclusion from parity checks because their event type values belong to a different cluster.

**Validation:** Documentation edits only; no product code, production queries, or Linear mutations.
