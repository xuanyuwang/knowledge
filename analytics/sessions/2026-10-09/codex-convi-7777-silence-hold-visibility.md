# CONVI-7777 Silence/Hold Assistance Insights visibility

**Question:** EA sees only some hint categories in staging and asks whether Silence/Hold hints are part of Assistance Insights.

**Source context:** `/Users/xuanyu.wang/repos/director` `origin/main` `da2e210926b60faefcdef8ede940ff40575d0e50`; related `/Users/xuanyu.wang/repos/go-servers` `origin/main` `48d308822e90e17d2ca190987220601601ee9d97`. Read-only source inspection; no staging session or customer profile was provided or inspected.

**UI evidence:** `AssistanceInsightsStatistics.tsx:214-216` renders `SilenceHoldHintsPage` for a distinct tab. `AssistanceInsightsCarouselMenu.tsx:484-488,803-825` exposes “Silence/Hold hints” when `enableSilenceHoldHintsTab` is enabled and, after loading, `silenceHoldHintStats.data.totalHintSentCount` is nonzero. `useGetSilenceHoldHintStats.tsx:29-39` selects active current-use-case policies with SILENCE behavior and a HINT action, and only calls RetrieveAdherences when such policies exist (`:82-90`). The adapter computes sent from SDX count and followed from SDX count multiplied by average adherence (`:103-129`). Thus a missing tab can result from a disabled flag, no matching active policies, or zero filtered sent count. The exact reason for EA's staging view is unknown.

**Metric boundary:** A policy being configured to send a hint and an adherence moment occurring do not prove a hint was actually delivered in that conversation. The current UI's “followed rate” is an adherence-derived approximation; it is not a qualifying-use record for CONVI-7777. Keep Silence/Hold on hold for per-occurrence delivery/action linkage validation rather than excluding it because one staging tab is absent.
