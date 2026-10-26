# Inactive scorecard workflow evidence consolidation

Date: 2026-10-25

This synthesis covers the historical roots below. Their meaningful content predates 2026-09-25; later capture commits and navigation-only edits are not new engineering activity. The source notes were read locally. No live ticket, PR, deployment, credential, database, or archived executable was used. The table gives each topic one parent-domain home and points to the preserved originals, including plans, recorded validation, and unresolved questions.

| Former root | Canonical summary | Primary subdomain | Last meaningful content | Date qualification |
|---|---|---|---|---|
| `auto-backfill-missing-scorecards` | [opera-dependent-scorecard-generation](../work-items/opera-dependent-scorecard-generation.md) | `process-scorecards-and-generation` | 2026-06-29 | 2026-07-03 is an explicit wrap-up with no new investigation. |
| `convi-6672-achieve-behavior-na` | [achieve-excluded-criteria-na-investigation](../work-items/achieve-excluded-criteria-na-investigation.md) | `evaluation-and-scoring` | 2026-04-26 | Dated README and log agree. |
| `convi-6709-reversed-scorecard` | [nrg-option-value-display-mismatch](../work-items/nrg-option-value-display-mismatch.md) | `evaluation-and-scoring` | 2026-05-02 | Dated investigation and log agree. |
| `convi-6841-process-scorecard-update-race` | [process-scorecard-create-autosave-race](../work-items/process-scorecard-create-autosave-race.md) | `scorecard-lifecycle` | 2026-05-27 | Later monitoring interpretation is meaningful beyond the May 19 implementation. |
| `convi-6862-disable-editing-on-submitted-scorecard` | [submitted-scorecard-permission-history](../work-items/submitted-scorecard-permission-history.md) | `permissions-and-visibility` | 2026-09-02 | Latest dated PR review; earlier rollout proposals are superseded within the source. |
| `convi-7237` | [scorecard-access-advanced-layout](../work-items/scorecard-access-advanced-layout.md) | `permissions-and-visibility` | 2026-07-09 | Single dated session/log. |
| `convi-7379-pure-focus-criteria-display` | [CONVI-7379](../work-items/CONVI-7379.md) | `template-authoring-and-versioning` | 2026-08-28 | Product suffix decision and local validation are the latest substantive record. |
| `duplicate-template-across-usecase` | [cross-usecase-template-duplication](../work-items/cross-usecase-template-duplication.md) | `template-authoring-and-versioning` | 2026-02-06 | Implementation summary has this date; log/2025-02-11.md conflicts with the 2026 implementation and is not a reliable latest date. |
| `export-appeal-comments` | [appeal-resolution-comments-export](../work-items/appeal-resolution-comments-export.md) | `appeals` | 2026-04-16 | Log records April 15 implementation and April 16 capture. |
| `group-calibration` | [group-calibration-csv-comment-parity](../work-items/group-calibration-csv-comment-parity.md) | `group-calibration` | 2026-07-06 | Latest investigations include deliberately reverted template-order work. |
| `nascore` | [na-score-representation-and-fallback](../work-items/na-score-representation-and-fallback.md) | `evaluation-and-scoring` | 2026-06-01 | Later dated CONVI-6955/6960/6962 notes extend the May 13 manual checklist. |
| `outcome` | [opera-email-autoqa-outcome-semantics](../work-items/opera-email-autoqa-outcome-semantics.md) | `evaluation-and-scoring` | 2026-07-08 | Single dated reference/investigation session. |
| `scorecard-permission-policy` | [scorecard-permission-evaluator-design-history](../work-items/scorecard-permission-evaluator-design-history.md) | `permissions-and-visibility` | 2026-06-16 | Updated design-plan supersedes the draft RPC naming in README. |
| `template-schema-version-updater` | [template-schema-updater-proposal](../work-items/template-schema-updater-proposal.md) | `template-authoring-and-versioning` | 2026-05-25 | Dated draft plan and log; no implementation evidence. |
| `scorecard-template` | [scorecard-template-historical-reference](../work-items/scorecard-template-historical-reference.md) | `template-authoring-and-versioning` | 2026-09-22 | Oct 4 commit 39db3b4 bulk-records late-September notes; Sep 22 dated work item/session/log contains the last substantive change. |

## Existing ticket items continued at the parent domain

- `scorecard-workflows/sessions/2026-10-25/convi-6862-disable-editing-on-submitted-scorecard-legacy-evidence/work-items/CONVI-6862.md`, `CONVI-7350.md`, `CONVI-7598.md`, and `CONVI-7612.md`: full-content canonical continuations now live in `scorecard-workflows/work-items/`, with permissions-and-visibility as primary subdomain. Dated history is preserved; primary-domain metadata and evidence links now point to the retained source. The new submitted-permission topic summary provides shared multi-ticket context and links to those canonical ticket items.
- `scorecard-workflows/sessions/2026-10-25/convi-7379-pure-focus-criteria-display-legacy-evidence/work-items/CONVI-7379.md`: a full-content canonical continuation now lives in the parent work-items directory, preserving the accepted read-only label decision and evidence links. Its primary subdomain is template-authoring-and-versioning. The original labels its domain coaching workflow; the current catalog has no coaching domain. This placement captures its template-identity compatibility lesson and should be reassessed if coaching becomes a durable domain.
- `scorecard-workflows/sessions/2026-10-25/scorecard-template-legacy-evidence/work-items/CONVI-7663.md` and `CONVI-7671.md`: a full-content canonical continuation now lives in the parent work-items directory, preserving the complete timelines. Use template-authoring-and-versioning for the template response incident and scorecard-lifecycle for the export consumer, respectively; both have cross-service API robustness implications. They are not new work items.

## Lessons retained across the roots

- Distinguish option identity, array index, mapped score, scored N/A, and no-score N/A. Several apparent scoring failures were representation or display mismatches.
- Label recovery from a historical revision is not evidence of the exact revision that created a coaching target. Keep revision-independent identity compatible until a storage/filter migration is deliberately designed.
- Template content, latest permissions, acknowledged locks, and reset authority are separate decisions. A permitted submitted-editor result does not establish that the entire UI must be editable.
- Generation requires source annotations; reindexing repairs a derived projection. A ClickHouse count gap does not itself prove that a new operational scorecard should exist.
- Empty persisted scorecards and score-bearing analytical facts have different meanings. Live AutoQA and backfill use shared persistence paths outside the coaching create endpoint.
- Fixed page counts do not bound serialized bytes. Scope and response projection remove observed overfetch but do not solve an oversized single text record.
- Export parity is explicit: appeal reasons, collaboration comments, criterion comments, label mapping, template order, and visibility choices differ by export path. Do not assume one CSV builder defines all behavior.

## Date and status cautions

[scorecard-template-historical-reference](../work-items/scorecard-template-historical-reference.md) has substantive Sep 22 work recorded in its log, session, and CONVI-7671 item; Git commit `39db3b4` on Oct 4 is titled "Record late-September knowledge updates". The later commit is capture time, not the event date. `scorecard-workflows/sessions/2026-10-25/auto-backfill-missing-scorecards-legacy-evidence/log/2026-07-03.md` explicitly says no new investigation since June 29. Cross-use-case duplication has a Feb 6, 2026 implementation summary and a conflicting 2025-named log; use the dated implementation as the latest evidenced content rather than silently repairing history.

Status words in older READMEs (including active, shipped, and PR open) are historical snapshots. Group Calibration even combines shipped and PR-open wording. This consolidation does not decide current shipment or close outstanding technical work. The next-action paragraphs in the canonical summaries say what should be refreshed if work resumes.

## Preserved source destination

Every original root in the table is retained under `scorecard-workflows/sessions/2026-10-25/<exact-former-root>-legacy-evidence/`. Original internal files remain evidence, while the parent-domain work items above provide the maintained synthesis. Removing former roots requires the main migration to finish preservation, link rewriting, and verification first.
