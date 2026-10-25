# CONVI-7777 — Smart Compose P2 and Silence/Hold flag guard

**Date:** 2026-10-09
**Primary source repo / context:** `/Users/xuanyu.wang/repos/go-servers`, `origin/main` at `48d308822e90e17d2ca190987220601601ee9d97`, read-only. Related Director `origin/main` at `da2e210926b60faefcdef8ede940ff40575d0e50`.

## Scope direction

Xuanyu directed that chat Smart Compose be excluded from CONVI-7777 v1, like email assistance, and that `enableSilenceHoldHintsTab` guard Silence/Hold hints. Record chat Smart Compose as P2. Any Silence/Hold contribution must require the flag to be enabled for the profile; the qualifying-use predicate remains unresolved. No product code changed.

## Source evidence and boundary

- [Director tab gate](https://github.com/cresta/director/blob/da2e210926b60faefcdef8ede940ff40575d0e50/packages/director-app/src/components/insights/assistance/assistance-insights-statistics/assistance-insights-carousel-menu/AssistanceInsightsCarouselMenu.tsx#L484-L488): the frontend checks `enableSilenceHoldHintsTab` and calculated nonzero sent volume to show the Silence/Hold tab. The flag is a visibility setting, not a use event.
- [Director adherence adapter](https://github.com/cresta/director/blob/da2e210926b60faefcdef8ede940ff40575d0e50/packages/director-app/src/components/insights/hooks/useGetSilenceHoldHintStats.tsx#L29-L39): selects active Silence policies configured with a HINT action; [conversion](https://github.com/cresta/director/blob/da2e210926b60faefcdef8ede940ff40575d0e50/packages/director-app/src/components/insights/hooks/useGetSilenceHoldHintStats.tsx#L103-L129) derives sent/followed totals from adherence counts.
- [Adherence query](https://github.com/cresta/go-servers/blob/48d308822e90e17d2ca190987220601601ee9d97/insights-server/internal/analyticsimpl/retrieve_adherences_clickhouse.go#L32-L75): does not join a delivered hint action to each adherence moment. The existing aggregate therefore cannot establish the CONVI-7777 per-conversation qualifying-use rule.
- Chat `USED_SMART_COMPOSE` conversation events exist, but the current [Smart Compose stats query](https://github.com/cresta/go-servers/blob/48d308822e90e17d2ca190987220601601ee9d97/insights-server/internal/analyticsimpl/retrieve_smart_compose_stats_clickhouse.go#L39-L99) joins legacy events to agent/hour rather than using conversation ID. P2 scope is a product prioritization decision, independent of technical event linkage.

## Handoff

Posted a [CONVI-7777 comment](https://linear.app/cresta/issue/CONVI-7777/create-a-new-metric-on-the-page-assistance-insights-to-reflect-the) mentioning EA to confirm the chat Smart Compose and email P2 proposal. It also records the Silence/Hold flag guard and the still-open qualifying-use signal. Comment ID `431fb8ad-5da1-493d-9226-fc5cab82ba5d`. Xuanyu was separately asked whether Silence/Hold requires a delivered-hint and positive-action link or may use the existing adherence-derived signal when the flag is enabled; no answer was available at handoff.
