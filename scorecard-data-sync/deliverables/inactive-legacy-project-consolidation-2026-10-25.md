# Scorecard sync legacy consolidation, 2026-10-25

The four legacy collections below belong to `scorecard-data-sync`, with no subdomain. Existing domain architecture, failure taxonomy, monitoring guidance, repair playbook and ordering analysis remain the durable references. This document preserves what each collection contributes, which historical claims need qualification, and what remains to verify if the work is resumed.

## Selection and canonical homes

The inactivity cutoff is 2026-09-25. Dates below are the latest identifiable meaningful authored work in the reviewed curated evidence, excluding migration banners or bulk commit dates. No collection requires retention at its old root on this evidence.

| Original root | Latest meaningful authored date | Canonical work item | Preserved original README |
|---|---|---|---|
| `backfill-scorecards` | 2026-03-10 | [CONVI-6209](../work-items/CONVI-6209.md); process follow-through in CONVI-6298 | [Evidence](../sessions/2026-10-25/backfill-scorecards-legacy-evidence/README.md) |
| `convi-6298-reindex-process-scorecards` | 2026-03-28 | [CONVI-6298](../work-items/CONVI-6298.md) | [Evidence](../sessions/2026-10-25/convi-6298-reindex-process-scorecards-legacy-evidence/README.md) |
| `convi-5565-scorecard-ch-pg-sync` | 2026-09-19 | [CONVI-5565](../work-items/CONVI-5565.md); continuing design in [ui-pg-ch-ordering](../work-items/ui-pg-ch-ordering.md) | [Evidence](../sessions/2026-10-25/convi-5565-scorecard-ch-pg-sync-legacy-evidence/README.md) |
| `pg-ch-scorecard-sync-investigation` | 2026-09-21 | Existing [ui-pg-ch-ordering](../work-items/ui-pg-ch-ordering.md) and [CONVI-7186](../work-items/CONVI-7186.md); broader theory remains domain reference | [Evidence](../sessions/2026-10-25/pg-ch-scorecard-sync-investigation-legacy-evidence/README.md) |

Commit `39db3b451c94a761d5f7db538372ae36e1105837` (2026-10-04) changed only the READMEs in the last two collections: a correction explicitly dated September 19 for CONVI-5565, and September 19/21 follow-up links for the investigation. It contains no additional October-authored investigation in either root. The September 21 explainer and September 19 source analysis already live in canonical domain deliverables. The July 15 migration commit `3f712aa` likewise added only a migration banner to the first two collections; their meaningful March dates are supported by their dated logs and results.

## Backfill operations: recorded success and operational cost

The January run completed 260 customer jobs across eight clusters after retries. Marriott and United East succeeded on simple retry; CVS and Oportun needed 31 daily sequential jobs each. Launching all days concurrently stressed PG and led to cancellation. This is evidence for using conversation-read population, measured worker capacity and resumable tracking to choose windows, not a generic rule that large parallel backfills are safe.

February appeal cleanup removed and reprojected 95 customers after PR #25653 excluded appeal-request scorecards. Approximate removal totals were 70.5 million scorecard rows and 650.5 million score rows. Oportun's huge mutation blocked queues and was replaced by daily deletion chunks and sequential backfill. Other large customers needed 5–10 day windows and polling allowances of several hours. A filter change does not delete previously eligible analytical rows; insert-only replay alone is insufficient.

March's manual process backfills wrote 2,120 scorecards / 30,464 scores for Spirit and 43,953 / 501,456 for Oportun. Historic-table indexing differed between customers, so Oportun reconstruction read director scores and template revisions. The 1,784-score comparison had zero unexplained mismatches, with 183 explained N/A/excluded-criterion differences. That validation supports those runs, not universal equivalence of a historical Python implementation with current Go scoring. Empty process scorecards can correctly produce a parent row without criterion rows.

The archive's advice to run parallel windows on weekends and to proceed while deletion mutations finish was based on particular executions. Mutation ordering, parts created after a mutation, delayed async inserts and replica/Distributed delivery all matter. Do not turn those observations into unconditional current operating instructions. Local-table cleanup across the cluster and verification of rows and fields remain necessary. Very large ID lists plus `FINAL` also exceeded query-size or memory limits in these runs.

Canonical reference: [Repair and backfill playbook](repair-and-backfill-playbook.md). Historical scripts are preserved evidence and must not be executed from this archive without current source/configuration review.

## Process reindex: implemented recovery and unfinished exceptions

CONVI-6298 closed a real selection gap: conversation-driven recovery cannot select process scorecards without conversation identity. The implemented scorecard-centric workflow reconstructs from director data using the production projection builder. It keeps conversation recovery separate, defaults empty type selection to process, and uses heartbeat progress without OFFSET resume. The initial cleanup implementation targeted distributed tables and cron forced cleanup; PR #26417 corrected local-table cluster cleanup and made cleanup configurable. Staging validated 196 scorecards / 721 scores for write and cleanup paths; production sandbox validation was recorded March 24.

The March 28 results supersede the README's unchecked Phase 1/2 tasks and old staging-only headline. Both phases ran, each completing 275 of 276 dispatched workflows. Brinks Care failed on duplicate criteria in different scorecards in each phase. Schwab's old image dispatched zero process workflows and needs separate accounting; it is not included in a universal-completion claim. Ten initial East heartbeat failures completed after serial redispatch; the archive attributes the failures to activity-worker overload under 90 simultaneous workflows. Pre-2026 East execution completed without those retries.

Resume only if the unresolved exceptions matter: find subsequent evidence of Schwab deployment/reruns and the Brinks Care data-quality decision before initiating new work. A successful job launcher or Temporal completion alone is weaker than projection convergence validation.

## Ordering fixes: preserve the correction, retire overclaims

CONVI-5565 introduced transactional historic writes and fresh PG re-reads for async CH work. GORM omission of submission columns addressed a separate PG lost update. Neither guarantees that the most recent source snapshot wins when CH versions use worker write time.

Spirit's March 11 shared-ID population of 2,996 had zero score/submitter mismatches but 31 zero-submit-time rows. March 12's different-range follow-up found 26 such rows, four also with stale scores; 24/26 came from Closed Conversations. An initial attribution to auto-scoring was corrected: enum value 1 represents the human Closed Conversations submission path. Auto-scoring in the examined path wrote directly and did not call the racing Update/Submit APIs.

The stale-source and later-write timestamps support the race mechanism. One surviving CH row cannot establish whether submit output was merged away or failed before insertion; preserve that uncertainty rather than repeating the archive's universal causal wording. Likewise, historical near-zero test failures at 100 ms spacing do not guarantee eventual convergence. “PG is always correct” and “acceptable limitation” are historical assertions, not current invariants or policy. Later PG parent/child consistency work exists in [CONVI-7784](../work-items/CONVI-7784.md).

The September 19 correction replaces the old explanation of the reverted PG timestamp proposal. Stale closure data alone cannot make a lower replacement version beat a higher one for the same key. The demonstrated incompatibility is cleanup: `update_time` also serves as indexing time, so a reindex row carrying an old PG timestamp can be deleted as old indexed data. Exact historical incident attribution remains unverified. The [September ordering options](ui-pg-ch-ordering-options.md) also correct earlier frontend assumptions: the shared form already serializes queued mutations; the gap includes pending debounce completion and save-success dependency. [The timestamp explainer](why-pg-updated-at-breaks-ch-cleanup.md) gives concrete source/engine examples. Keep continuing decisions in the existing ordering work item.

## General investigation: concrete cases beyond the theory scaffold

The investigation separates omission, stale replacement, partial projection, coverage gaps, cleanup mismatch and observability failure. Its theory/solution documents are useful historical scaffolds; the domain deliverables already contain the synthesized model. Prevention, detection and repair solve different problems: transactional source correctness does not guarantee durable delivery, and a repair mechanism cannot compensate for a monitor that cannot finish or cannot see an existing stale row.

The June 8 Pack Rat case proved the distinction: conversation/messages aligned, but a scorecard's submit metadata was stale. Targeted `JOB_TYPE_REINDEX_SCORECARDS` repaired parent and emitted score submit times. Six additional PG chapter aggregate rows were intentionally not emitted by the builder, so raw row-count equality would have misclassified a valid transformation as missing data.

The June 30 timeout investigation identified expensive unsubmitted inventory/count queries after #29273. Oportun's excluded-count plan scanned scorecards in parallel and a bounded analyze timed out at 45 seconds; Groundcover timing supported PG inventory work as the bottleneck, rather than result collection or Slack. The July implementation compared the broad inventory with CH first, then checked score existence only for missing unsubmitted IDs in batches of 5,000, removing empty shells from numerator, denominator and repair candidates. [CONVI-7186](../work-items/CONVI-7186.md) already records the implementation, focused test, merge and main-image publication; do not duplicate that work item or treat an image build as observed production runtime recovery. Its historical existence-monitor limitation must also be read alongside later timestamp-staleness monitoring work in this domain.

## Verification and limits of this consolidation

Reviewed root READMEs, dated curated logs/results, domain references, existing work items and relevant local Git patches. This was a historical document review; no customer database, source build, old script or verification binary was run, and no credentials were used. Large raw log collections and connection wrappers were not opened. The preserved evidence paths above are the agreed destinations for the repository consolidation; this synthesis does not itself perform source moves, metadata/index updates or Git publication. No memory-derived system fact is needed for these conclusions.
