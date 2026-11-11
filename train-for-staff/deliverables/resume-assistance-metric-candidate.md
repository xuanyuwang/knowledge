# Resume candidate: product judgment from an aggregate metric request

Date: 2026-11-10. Status: suggested wording for the November 7 Markdown resume; not yet selected for the main draft.

## Recommended resume bullet

- **Turned a one-line product request into a conversation-level AI adoption metric**, defining qualifying usage, investigating counting and attribution differences across seven analytics APIs, and designing the aggregation to support future feature-level drill-downs.

The important evidence is the sequence of decisions: infer the customer question, define what counts, investigate existing query behavior, and choose a design that can answer a useful follow-up question. The ticket's size and internal names are unnecessary in the resume. Keep “designing … to support future” rather than implying a delivered drill-down.

## Optional second bullet

- **Proposed a unified analytics API** after identifying overlapping data queries across a dashboard, with the aim of sharing query work between aggregate metrics and feature details.

This is secondary to the first bullet because implementation, adoption, and measured savings are not established. Use it when the target role values API/product architecture, or retain it for the interview. The user reports that another team owns the page and that this was a proposal made while helping with the feature; do not imply ownership of that team's roadmap.

## Placement

Place the main bullet near the top of the Cresta section, beside a strong measured infrastructure/correctness result. For a short resume, consider replacing the user-filter bullet, which overlaps with the semantic-investigation theme; preserve Group Calibration as evidence of a larger product workflow. Final portfolio selection remains open.

## Interview expansion

The request was essentially to add an aggregate metric above existing assistance tabs. I first established that the customer question was how many conversations used assistance. I investigated the tabs and underlying queries to determine which signals counted as use and where counting, eligibility, and attribution differed. The aggregation needed conversation and feature evidence, so I recognized that preserving that evidence could support a future explanation of which features had been used. I also noticed overlapping query work across the page and proposed a unified API to the owning team. That broader change remained a proposal.

“Free” means the feature identity is already available during the computation. Returning and supporting details can still add payload, authorization, testing, and maintenance work. No zero-cost implementation or customer demand already received is claimed.

## Evidence and limits

- User's 2026-11-10 account supplies the one-line assignment, anticipated customer question, page ownership, and page-wide API proposal.
- [Canonical CONVI-7777 work item](../../analytics/work-items/CONVI-7777.md): seven feature helpers, dedicated aggregate API, implementation checkpoint, and explicit pending details/production-performance work. This resume pass did not refresh live PR or deployment state.
- [API decision rationale](../../analytics/deliverables/convi-7777-assistance-usage-api-decision-rationale.md): different populations, attribution, counting units, and why existing aggregate responses cannot produce a distinct-conversation union.
- [Optional details session](../../analytics/sessions/2026-10-15/codex-convi-7777-optional-conversation-details.md): a designed opt-in contract that derives feature details from the same cohort/evidence; a contract is not a shipped detail endpoint.
- [Latest Markdown resume](resume-draft.md): current draft and project portfolio remain unchanged in this recommendation pass.
