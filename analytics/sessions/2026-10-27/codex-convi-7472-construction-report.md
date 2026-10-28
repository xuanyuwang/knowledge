# CONVI-7472 construction report — October 27, 2026

Source repo: `/Users/xuanyu.wang/repos/director`.
Worktree: `/Users/xuanyu.wang/repos/director-convi-7472`.
Branch: `xw/convi-7472-adjusted-appeals`.
Base/HEAD: `76f6b847418f7f287e9b598279348c1a7394c6e1`; reviewed final code is uncommitted.

User requested a construction report. Used review-code-change skill and interactive-report format. Re-read current diff, component/test, shared API hook/filter transformer, and overview consumer. Reused implementation-run test logs and live browser evidence; no new production request or credential use. No source code changed for this report.

Artifact: [standalone construction report](../../deliverables/convi-7472-construction-report.html).

Three explanatory stages: original chart assignment, regression contract and failing cases, corrected array reference with final criteria-distinct fixture. Contains data-path map, before/after simulation, scenario table, validation limits, effects on shared consumers, findings/recommendation, pinned baseline links, and exact captured local source.

Captured local file SHA-256:
- `packages/director-app/src/components/qa/report/appeals/appeal-trends-card/QAReportAppealTrendsCard.tsx`: `ae4063db2add98d7b9f2a2ecd139b42bf892192e915f635f9cc9275407db7969`
- `packages/director-app/src/components/qa/report/appeals/appeal-trends-card/QAReportAppealTrendsCard.test.tsx`: `4c612e191bc2a90173568d7b2efaac4594fe4bb0c9657cbc2ee59ad7ab45db28`

## Report verification

- HTML parsed; IDs are unique and internal anchor targets exist.
- Embedded JavaScript passes Node syntax check.
- In-app browser: exercised baseline/middle/final stages, both code views, Previous/Next, and adjusted values 3/0/omitted. All outputs match the explanatory model.
- Inspected desktop rendering; no browser console errors recorded.
- Attempted 390px viewport override, but the browser continued reporting/rendering 1280px. Reset override; narrow visual rendering remains unverified and the report says so.
- No product source changes or new credential use. Reused prior implementation test results explicitly labeled by date.
