# Scorecard Configuration Safety: Initiative Proposals

**Status:** Staff-engineering proposal for product discovery
**Primary domain:** Scorecard Workflows
**Primary subdomains:** Template Authoring and Versioning; Evaluation and Scoring
**Related pattern:** [Mixed-revision QA score semantics](../../analytics/deliverables/mixed-revision-qa-score-semantics.md)
**Prepared:** 2026-07-27
**Revised:** 2026-07-28

## Strategic Framing

Customers can create scorecard templates that are syntactically valid but do not express their intended scoring policy. For example, assigning zero weight to every scored criterion can make every resulting scorecard N/A. The calculations are correct for the configuration, but the outcome is not what the customer intended.

Customers commonly discover these mistakes only after they publish a template, create scorecards, and see unexpected results. They then depend on Cresta employees to diagnose the configuration and explain how to repair it. The proposed product direction is a two-part safety lifecycle:

1. **Prevent and improve** configuration through simulation, validation, and AI-assisted authoring.
2. **Assess and remediate** affected scorecards through a controlled template-correction workflow.

These proposals are intended to establish a high-level direction and motivate product discovery, not to serve as complete PRDs.

## Initiative 1: AI-Assisted Template Creation and Scorecard Simulation

### Problem

Creating a template is manual, tedious, and error-prone, especially when assigning criterion weights, answer scores, N/A behavior, and failure rules. The builder can accept a configuration that produces unexpected outcomes, but customers have no realistic way to validate those outcomes before applying the template. New template features add further complexity and create more opportunities to accidentally disturb an existing configuration.

### Proposal

Add a template creation and preview experience that lets customers validate scoring behavior before publishing a revision, then use AI to make authoring easier.

The experience should:

- render sample scorecards and explain how criterion, section, and overall scores are calculated;
- preview how simulated scorecards will appear in Performance Insights and Leaderboard;
- flag likely mistakes, such as all scored criteria having zero weight, inverted answer mappings, unreachable outcomes, or surprising N/A behavior;
- accept typed, spoken, or document-based scoring requirements and generate a proposed template, including criterion weights and answer scores;
- automatically run the same simulations against the generated template;
- explain each warning in business terms and suggest a correction; and
- require the owner to review and approve all changes before publication.

AI should improve authoring and explain intent, while a deterministic scoring simulator should remain the source of truth for previews and validation.

### Expected Outcome

Template owners can see the practical result of a configuration before it affects production evaluations or reporting. AI reduces the effort required to translate business requirements into configuration, while simulation provides the verification loop needed to use AI safely.

### Initial Success Signals

- fewer published revisions that are quickly replaced to correct scoring configuration;
- fewer support cases caused by zero or ineffective weights, inverted mappings, or unexpected N/A behavior;
- increased use of pre-publish simulation and resolution of high-confidence warnings; and
- reduced time to create or revise a template.

### Boundaries for Discovery

- The assistant should not publish autonomously or claim to know customer policy better than the template owner.
- Validation should distinguish objectively inconsistent behavior from subjective recommendations.
- Product discovery should define which warnings block publication, require acknowledgment, or remain advisory.

## Initiative 2: Re-Evaluate Historical Scorecards After Template Corrections

### Problem

A customer may discover a template configuration mistake only after weeks of evaluations have been created. Publishing a corrected revision protects future scorecards, but historical scorecards remain tied to the faulty revision. Their answers may still be valuable, yet customers have no supported way to re-evaluate them under the corrected business rules.

[CONVI-7238](https://linear.app/cresta/issue/CONVI-7238/united-leaderboard-manager-scorecards-evaluated-undercount-vs) illustrates the impact: a template revision assigned zero weight to every scored criterion, so affected scorecards had N/A aggregate scores and disappeared from parts of downstream reporting.

### Proposal

Provide an administrator workflow to re-evaluate selected historical scorecards against a corrected template revision.

The workflow should:

- compare the original and corrected revisions, highlighting changes to answer mappings, weights, N/A behavior, and failure rules;
- preview the affected scorecard population and the expected before-and-after impact before any action is taken;
- let an authorized owner select an explicit scope, such as date range, evaluation type, or scorecard state;
- carry existing answers forward and recompute eligible scorecards using the corrected revision; and
- preserve the original revision, original result, corrected result, approval, and remediation event in an auditable history.

For a confirmed configuration error, re-evaluating historical scorecards can be the correct business action and can recover otherwise valuable data. The product should make that action explicit and traceable rather than silently repointing a historical scorecard to another revision. The exact treatment of submitted scorecards, manual overrides, AutoQM outputs, appeals, and downstream analytics requires product, compliance, and technical discovery.

### Expected Outcome

Customers can recover valid historical evaluations after correcting a template instead of discarding the data or relying on one-off engineering intervention. Cresta reduces support and investigation effort while preserving confidence in scorecard history.

### Initial Success Signals

- reduced time and engineering effort required to remediate template mistakes;
- percentage of eligible incidents resolved through the guided workflow;
- low rate of rollback, dispute, or support escalation after remediation; and
- clear reconciliation between remediated scorecards and downstream analytics.

### Boundaries for Discovery

- Re-evaluation must be an explicit, scoped decision; publishing a new revision should not automatically rewrite history.
- Submitted, appealed, calibrated, or manually overridden scorecards may require different policies.
- Authorization, approval, audit, rollback, and downstream recomputation are first-class requirements, not implementation details.

## Relationship and Suggested Sequencing

The initiatives are complementary rather than competing:

- Initiative 1 reduces the likelihood of a mistake.
- Initiative 2 limits the impact when a mistake still occurs.

Both should share a deterministic scoring-simulation and revision-comparison foundation. A practical sequence is to build simulation first, expose it as a pre-publish preview, reuse it for historical impact assessment and re-evaluation, and then layer AI-assisted generation on top of that reliable verification loop.

## Product Discovery Still Needed

- customer research on the most common configuration mistakes and desired safeguards;
- policy for historical correctness versus preserving the originally applied rulebook;
- treatment of submitted evaluations, overrides, appeals, calibration, and compliance records;
- analytics and export behavior during and after remediation;
- permission, approval, notification, rollback, and audit requirements; and
- baseline metrics for incident frequency, support cost, and correction effort.

## Source Notes

This proposal was derived from the validated mixed-revision QA score pattern and the draft initiative text supplied by the author:

- [Use AI to preview and generate template configuration](https://docs.superhuman.com/d/_dycRJj-V1bs/Initiative-Use-AI-to-preview-and-generate-the-template-configura_suE9JRQV)
- [Workflow to update scorecard revision](https://docs.superhuman.com/d/_dycRJj-V1bs/initiative-workflow-to-update-scorecard-revision_sudJgV31)
- [CONVI-7238: United Leaderboard scorecard undercount](https://linear.app/cresta/issue/CONVI-7238/united-leaderboard-manager-scorecards-evaluated-undercount-vs)
