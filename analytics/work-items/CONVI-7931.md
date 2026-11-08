# CONVI-7931: Oportun process scorecard count mismatch

- [Linear ticket](https://linear.app/cresta/issue/CONVI-7931): created October 29, 2026, assigned to Xuanyu Wang; Todo / Medium / Customer Issue.
- [Slack report](https://crestalabs.slack.com/archives/C04NB5AMV0F/p1793224226297459): QM 338 / 99.6%, Performance Insights 275 / 100%, July 1–31.
- Related: CE-8103 (formerly CONVI-7667), CONVI-7674 / Director #22772, Zendesk #24673.

## Current working attribution — November 4

Updated the Linear description at the user's request with two separate explanations: the earlier 275 visible count most likely reflects issue 7's temporary parent/criteria visibility window; the persistent 12-card loss is attributed to issue 6's stale-write ordering. The first remains a hypothesis because the original request-time state is unavailable. The second is supported by the live source-version/build-time evidence, while the original worker sequence is still unverified. The measured 338 → 326 gap is distinct from the unverified original 63-ID gap. No repair or new database investigation in this ticket-update follow-up.

## October 29 evidence

The previous Director fix is included in today's served Oportun release. For the July 2026 UTC window, PostgreSQL and ClickHouse contain the same 338 parent IDs, but 12 ClickHouse parents have stale source timestamps relative to their criterion rows. The analytics ID-and-version join drops those 12: direct SQL and the live daily PI view both return 326. Weekly grouping expands the range to June 28–August 2 and both SQL and UI return 335.

QM's 99.6% matches the mean whole-card score; the current PI Criteria adherence calculation is 99.7071% on the 326 joined cards and displays 100%. The score difference alone does not establish a defect.

The reported 275 has not been reproduced. The original year, timezone, filters, user and payload remain unknown; the confirmed 12-card loss does not explain all 63 reported missing cards. User requested continuing with available evidence. No product code or customer data was changed.

## November 4: compare with the recently documented sync delay

Fresh raw and FINAL ClickHouse reads found the same 12 source-version mismatches. `clusterAllReplicas` returned all 12 parents and all 38 criteria per card on each of three nodes; every node agrees with the October 29 versions. This is not explained by a single stale read replica. Every stale parent has a later projection build timestamp than its newer criteria (11.779–1654.746 ms later), while the source versions differ by 339.341–504.044 ms.

The immediate join failure is the same as issue 7 in the [reviewed sync/race notes](../../scorecard-data-sync/deliverables/scorecard-races-corrected-recommendations.md), but the data points more strongly to issue 6's stale projection ordering. Issue 7 describes a temporary newer-parent/older-criteria window; these cards have older parents/newer criteria and the persisted mismatch remains unchanged. Historical worker order is not proven, so this is evidence consistent with stale snapshot replacement rather than a confirmed reconstruction of July writes. The timestamp gap is a source-version gap, not measured network lag.

A latest-common-version query cannot recover these cards from the surviving raw data: no matching source version exists in both tables. No reindex or customer-data changes performed. [Live evidence and comparison](../sessions/2026-11-04/codex-convi-7931-sync-delay-comparison.md).

## Next steps after the November 4 comparison

Trace the stale parent writes and sync-monitor deployment/window/configuration; prepare a targeted reindex of the 12 identified cards and verify parent/criterion version parity and daily count 338 afterward. The strict join protects all-N/A overwrites and should not be casually removed. Original-session attribution remains separate from the confirmed current data defect.

## Evidence

- [Investigation and exact source references](../sessions/2026-10-29/codex-convi-7931-oportun-process-count.md)
- [Reproduction SQL](../sessions/2026-10-29/convi-7931-latest-parent-join.sql)
- [Affected IDs and timestamps](../sessions/2026-10-29/convi-7931-stale-parent-versions.csv)

Primary repos: go-servers and Director; placement from current config. Investigation used main checkouts, no implementation worktree.
