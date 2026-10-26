# CONVI-6192: Conversation sources and adoption metrics

**Record state:** Historical investigation, consolidated 2026-10-25; current ticket status not rechecked.
**Primary domain:** analytics
**Primary subdomain:** active-days
**Official ticket:** [CONVI-6192](https://linear.app/cresta/issue/CONVI-6192)
**Last substantive source update:** 2026-02-06

## Objective and Impact

Allow Alaska Airlines to distinguish real-time Cresta usage from conversations imported by postcall jobs. Counting imported conversations can make an agent without the extension appear active. Role: investigated the source filter and proposed customer configuration.

## Source Context

- Primary repo: `/Users/xuanyu.wang/repos/go-servers`; related repos: `cresta-proto`, `config`.
- Historical paths: `insights-server/internal/analyticsimpl/retrieve_agent_stats_clickhouse.go`, `insights-server/internal/shared/common.go`, and `cresta/v1/config/features/insights.proto`.
- No source branch, PR, deployment, or customer setting was revalidated during consolidation.
- [Original investigation](../sessions/2026-10-25/convi-6192-legacy-evidence/README.md).

## Findings and Decisions

The February record describes a default source set of real-time `SOURCE_UNSPECIFIED` (0) and postcall `INGESTION_PIPELINE` (8), with optional `BOT` (7). It proposes `exclude_ingestion_pipeline_from_agent_stats` to remove source 8 without changing the bot flag. Source inclusion is an analytics eligibility decision; it is not sufficient evidence that an agent actually used Cresta.

The archived note mixes a proposal with configuration instructions and contains conflicting candidate proto field numbers (19 and 20). Treat it as historical design evidence, not an authoritative current contract or proof of rollout. The domain's existing invariant is to retain a valid zero when excluded sources are the only conversations for an agent/day; dropping the row would change zero into N/A.

## Validation and Next Actions

- Consolidation validated the records and links only; no implementation tests or live configuration checks were performed.
- If reopened, inspect current proto fields, source-filter helper callers, customer configuration, and zero-versus-N/A behavior before changing anything.
- Keep ongoing metric semantics in [Active Days](../subdomains/active-days/README.md).

## Timeline

- 2026-02-06 — Investigation recorded; implementation and rollout outcome not established by the saved material.
- 2026-07-15 — Legacy README gained domain navigation.
- 2026-10-25 — Historical evidence moved under Analytics; see [migration session](../sessions/2026-10-25/codex-five-legacy-project-consolidation.md).
