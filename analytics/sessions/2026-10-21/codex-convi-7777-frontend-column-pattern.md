# CONVI-7777 frontend column pattern review

- Date: 2026-10-21 (America/Toronto)
- Primary domain: Analytics / Assistance Insights
- Source repo: `/Users/xuanyu.wang/repos/director`
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7777-assistance-usage`
- Branch: `xw/convi-7777-assistance-usage`
- Base: `cd7b46d052c694ec6cd689b8f6ab9f01e9690104`; reviewed head is the uncommitted source snapshot, frozen by SHA256 manifest.
- Request: apply review-code-change to explain how to add a column using the two remembered Assistance Used columns and CONVI-7777 implementation.

## Evidence and conclusions

Confirmed local history: Kurt Choi added the two remembered columns in `1c5299b2810625de1dc00e2ca9002e4e11e9ecd8` (CONVI-3323 / Director #8535), then added their CSV mappings in `fb335d6b30307cf6ccb1282bcbf5cbac3062f6da` (#8576). Xuanyu's analogous own change is `da2050af5d67fb375e0c60f289c2281b017c114f` (submitted scorecard column; CONVI-6968 / #19346).

The reusable pattern is data contract and numeric unit → request/transform → props → stable-resource-name row map → accessor/format/sort → eligible-column set plus user visibility → CSV mapping and translations. New RPCs are needed only when existing data cannot express the metric. CONVI-7777 uses per-group counts from its dedicated API; it must not reconstruct a distinct-conversation union from per-feature totals.

Rows from the new API must be seeded before existing enrichment loops. Those loops can return a temporary missing row without inserting it, so late insertion loses Active Days, conversation volume and AHT. The implementation already fixes this and includes a regression; the report's broken intermediate is explicitly reconstructed for teaching, not a real commit.

New percentages are numeric 0–100 (`3/8*100=37.5`). Old powered-by-AA uses a fraction and multiplies in the cell. CSV reads raw row values, not formatted cells. A new column must choose its unit explicitly.

## Validation

Fresh Node 24 test runs: API 11/11 tests across new and existing InsightsApi suites; Agent/Team hook and CSV tests 3/3. Existing dependency sourcemap warnings occurred. Prior Oct 20 type/lint/format checks are historical, not rerun here. Product UI, deployment and live endpoint integration are not verified. No product edits or Git publication in this review.

## Deliverable

Interactive report and frozen source evidence are in `analytics/deliverables/`. Report verified in local headless Chrome using Playwright: 9 steps, both views (18), Previous/Next boundaries, 32 source links, all 25 captured-source selections, and no JavaScript errors. Desktop and 390px screenshots visually inspected. Initial narrow-screen table overflow was corrected with horizontally scrolling table wrappers; final page width equals viewport width (390px). The conclusion and construction snapshots are present with JavaScript disabled. Product source hashes match the captured manifest (15 changed/new files).

Artifacts: [report](../../deliverables/convi-7777-frontend-column-pattern.html), [manifest](../../deliverables/convi-7777-frontend-column-evidence/manifest.json). No credential/profile was explicitly selected or used.
