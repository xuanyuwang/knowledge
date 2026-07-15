# Evaluation and Scoring

## Purpose

Own how criteria are evaluated and transformed into criterion, chapter, and overall scores for manual and automated workflows.

## Semantics and Invariants

- Option identity, index, raw/numeric value, mapped score, percentage score, and label are distinct.
- N/A/null means no applicable score and must not silently become zero.
- Normal options require a numeric score; an N/A option may have a null or explicitly configured numeric score.
- Excluded or effectively zero-weight criteria can retain evaluation evidence without contributing to aggregates.
- AutoQM outcome/behavior annotations are inputs to scoring; annotation generation and score computation are separate phases.
- Reversal changes lifecycle state and may affect which result is authoritative, but does not redefine the scoring formula.

## Architecture and Source Map

- **Frontend:** scorecard forms and template scoring/AutoQA configuration
- **Backend:** shared scoring helpers, coaching scorecard actions, AutoQM mapping
- **Storage:** template revisions, PostgreSQL scores, and analytics projections
- **Upstream inputs:** Opera behavior/outcome annotations

## Operational Knowledge

- Trace score symptoms through option mapping, `not_applicable`, weight/exclusion, percentage calculation, and revision identity.
- Validate manual and automated paths independently before comparing their outputs.

## Legacy Sources and Cases

- `nascore/`
- `convi-6672-achieve-behavior-na/`
- `outcome/`
- `convi-6709-reversed-scorecard/`
- `alo/` (retained initiative; cross-link when its evaluation model is reused)

## Open Questions

- Publish one authoritative score-field and denominator matrix.
- Document parity requirements between manual and AutoQM evaluation.
