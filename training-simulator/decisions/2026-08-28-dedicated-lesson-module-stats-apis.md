# Dedicated lesson and module statistics APIs

Date: 2026-08-28
Status: Accepted; supersedes the Option 2 list-extension direction

## Decision

Add two dedicated batch reporting RPCs:

- `RetrieveTrainingSimulatorLessonStats`
- `RetrieveTrainingSimulatorModuleStats`

Keep `ListTrainingLessons` and `ListTrainingModules` as content-list APIs. Do not add `include_stats`, reporting filters, or parallel statistics arrays to their requests and responses.

Both reporting RPCs accept a required profile parent, a bounded list of lesson or module resource names, and a reporting time range. They return statistics aligned to the requested resource-name order, including a zero-valued entry for a valid requested item with no matching assignments. The module request intentionally has no agent selector: the current Figma reports one aggregate per module across the reporting population and exposes no assignee filter on that surface. The two public contracts remain separate while reusing the same internal assignment/result loading and aggregation primitives.

## Why the direction changed

Option 2 was chosen primarily to avoid a second Director request, especially the cold start when switching between the Lesson and Module tabs. The Slack review identified stronger long-term API concerns:

- Statistics are a separate reporting responsibility that the content-list APIs were not designed to own.
- Engineers naturally look for an explicit statistics method; hidden `include_stats` and `stats_*` fields make the behavior harder to discover.
- Repeatedly adding unrelated optional behavior to list APIs weakens API conventions and creates maintenance cost for future engineers who lack the original context.
- Dedicated APIs give reporting independent authorization, caching, latency, error, monitoring, and evolution boundaries. A statistics failure does not need to fail content authoring/listing.
- The frontend cost is one batch request per active tab, not one request per row. Cached content names, React Query caching, and inactive-tab prefetch can mitigate cold switching. The team did not consider the extra request a major enough performance concern to outweigh the cleaner boundary.

The data path does not change: backend handlers still query assignments, current lesson/module definitions, task runs, and conversation-score rows directly. They do not aggregate a `ListTrainingSimulatorTaskRuns` response.

## Tradeoff accepted

The dedicated boundary adds two service methods, generated clients, hooks, and query keys. A first visit normally needs the content list before the batch statistics request can be issued, and content/statistics can observe slightly different snapshots. We accept those costs and require cold/cached tab-load measurement during implementation. If latency misses the agreed target after caching and prefetch, optimize from measured evidence rather than merging reporting back into content lists by default.

## PR impact

- Milestone 3 / [cresta-proto#9676](https://github.com/cresta/cresta-proto/pull/9676): retain `TrainingSimulatorModuleStats` and `TrainingSimulatorCriterionStats`; replace the `ListTrainingModules` extensions with `RetrieveTrainingSimulatorModuleStats` request/response messages and RPC.
- Milestone 2 / [cresta-proto#9677](https://github.com/cresta/cresta-proto/pull/9677): retain `TrainingSimulatorLessonStats` and its reuse of `TrainingSimulatorModuleStats`; replace the `ListTrainingLessons` extensions with `RetrieveTrainingSimulatorLessonStats` request/response messages and RPC. Keep the PR stacked on #9676 while it imports the shared module statistics contract.
- Remove the AIP-132 suppressions that existed only to permit parallel statistics fields on list responses.

## Decision evidence

[Slack thread in `private-qm-coaching`](https://crestalabs.slack.com/archives/C05L7FBAGRF/p1787928318264329):

- 2026-08-28 21:38 — Xuanyu summarized the three options and stated that avoiding additional frontend calls, especially cold tab-switch startup, was the main reason for selecting Option 2.
- 2026-08-28 21:46–21:47 — Kurt Choi explained that statistics are outside the list API's original responsibility, violate the dominant API convention, and are less discoverable than an explicit stats method.
- 2026-08-28 21:48 — Tinglin Liu agreed that Option 3 is cleaner if the frontend call is not a material issue.
- 2026-08-28 22:39 — Xuanyu agreed that Option 3 has the cleaner mental model and that it should be used when performance is not the major concern.

The direction supplied on 2026-08-28 confirms the switch to Option 3.

## Supersedes

- The working Option 2 selection in the Milestone 2 and Milestone 3 implementation records.
- The open/reopened status in [the earlier API-boundary record](2026-08-18-lesson-module-stats-api-boundary.md).
