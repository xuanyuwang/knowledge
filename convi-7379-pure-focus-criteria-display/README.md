# CONVI-7379 - Pure Focus Criteria Display

## Status

Investigation complete. Root cause identified as stale focus-criteria IDs in Pure coaching plans/sessions after scorecard template revisions removed the referenced criteria.

## Summary

Pure coaching plan 1:1 sessions display raw criterion IDs and scorecard template resource paths because the frontend resolves criterion display names only from **current** scorecard templates. Jeff Sykes' active coaching plan still references criterion IDs that were removed from the Security/MS Security template in December 2025.

## Affected Pure Data

- Agent: Jeff Sykes (`a53848957ec81e6b`)
- Coaching plan: `019952c4-501b-7485-8fdc-becca2fadda0`
- Example session (2026-07-24): `019f9542-63d0-708f-827f-06cb00e926b2`
- Scorecard template: `0196dac5-6c3b-717e-bc9a-d95bb54c51b2` (Security / MS Security)
- 3 active Pure coaching plans reference stale criterion IDs

## Next Steps

1. Decide fix path: backend enrichment (`criterionDisplayName`), frontend historical lookup, and/or Pure data migration.
2. Implement product fix so stale criterion IDs never render as raw resource paths.
