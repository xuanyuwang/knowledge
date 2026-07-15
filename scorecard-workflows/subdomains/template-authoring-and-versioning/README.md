# Template Authoring and Versioning

## Purpose

Own how scorecard templates are modeled, authored, revised, duplicated, evolved, and interpreted historically.

## Semantics and Invariants

- A template is a reusable definition; a scorecard is a runtime record instantiated under template semantics.
- Historical correctness depends on the template revision used by the scorecard, not merely the template's latest shape.
- The template structure, template permissions, and runtime scorecard state are separate policy inputs.
- Schema compatibility and business revisioning solve different problems: normalization must not erase historical meaning.
- Cross-use-case duplication must remap use-case-dependent identifiers and configuration deliberately.

## Architecture and Source Map

- **Frontend:** Director template builder and save/load transforms
- **APIs/backend:** coaching template CRUD, validation, duplication, and revision resolution
- **Storage:** PostgreSQL template records and versioned JSON structure
- **Compatibility:** proposed explicit schema version plus sequential updater chain

## Operational Knowledge

- Diagnose old-template failures by separating malformed current data, historical schema shape, and intended historical semantics.
- Treat FE/BE normalization drift as a compatibility defect.

## Legacy Sources and Cases

- `scorecard-template/`
- `duplicate-template-across-usecase/`
- `template-schema-version-updater/`

## Open Questions

- Choose the authority and rollout strategy for schema normalization.
- Publish a complete duplication/remapping matrix.
