# Scorecard Configuration Safety Proposals

**Date:** 2026-07-27
**Tool:** Cursor
**Primary domain:** Scorecard Workflows
**Primary subdomain:** Template Authoring and Versioning
**Source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree:** `main` at `/Users/xuanyu.wang/repos/go-servers`

## Objective

Turn two early initiative ideas into brief, formal product feature proposals suitable for raising a systemic customer problem with product and engineering leadership:

1. AI-assisted preview and generation of scorecard template configuration.
2. A workflow to update or remediate scorecards after a template revision correction.

## Inputs Reviewed

- `analytics/deliverables/mixed-revision-qa-score-semantics.md`
- `analytics/deliverables/convi-7378-scan-consent-qa-score-investigation.md`
- Scorecard Workflows domain model and operating guidance
- Two linked Superhuman Docs initiative pages

The linked pages redirected to authentication. The Coda connector was unavailable, and enterprise search did not find indexed copies. Their existing text therefore could not be reviewed directly on 2026-07-27. The author supplied the draft text on 2026-07-28; see the [refinement session](../2026-07-28/cursor-refine-scorecard-initiative-proposals.md).

## Product Framing

The central customer problem is not incorrect arithmetic. Customers can publish technically valid template configurations that do not match their intended scoring policy, and the product neither helps them detect those mistakes before publication nor provides a safe self-service correction path afterward.

The two initiatives form one safety lifecycle:

- **Prevention and improvement:** deterministic preview and validation, with AI assisting generation and explanation.
- **Impact mitigation and repair:** revision comparison, blast-radius preview, scoped remediation, and complete audit history.

## Key Design Judgments

- Lead with the customer outcome rather than AI as the product goal.
- Keep deterministic scoring simulation as the verification source of truth; use AI for intent translation, suggestions, and explanation.
- Do not frame historical revision behavior as simply “wrong.” A correction may reflect changed intent, and not every historical scorecard should be rewritten.
- Do not propose silently changing pinned revisions. Submitted evaluations, overrides, appeals, calibration, compliance, and downstream analytics make auditability and explicit approval essential.
- Reuse a common revision-comparison and scoring-simulation foundation across prevention and remediation.

## Output

- [Scorecard Configuration Safety: Initiative Proposals](../../deliverables/scorecard-configuration-safety-initiative-proposals.md)

## Follow-Up

- Product discovery should validate customer workflows, correction policy, permissions, audit requirements, and success baselines before either idea becomes a PRD.
