# CONVI-7777 historical scan and time-semantics investigation

- Date: October 21, 2026, America/Toronto.
- Primary domain: Analytics; primary subdomain: Assistance Insights.
- Source repo: /Users/xuanyu.wang/repos/go-servers.
- Branch/worktree context: read-only comparison of pinned main b6a2c65bdb6ba679a3bdf955b266cfd22f1a9ceb and PR #33993 head 0d1b75da684ee0a9237020d6d500e117db6f7b4e; compiler overlays in /Users/xuanyu.wang/repos/go-servers-convi-7777-assistance-usage use saved snapshots rather than altering concurrent source edits. GitHub head was rechecked at closure and still matched.
- Canonical work item: [CONVI-7777](../../work-items/CONVI-7777.md).
- Deliverables: [interactive report](../../deliverables/convi-7777-assistance-scan-audit.html), [Markdown report](../../deliverables/convi-7777-assistance-scan-audit.md).
- Evidence directory: [saved snapshots, generators, SQL and results](convi-7777-assistance-scan-audit/).

## Request and boundary

Xuanyu asked to extend the historical-scan review comment across other assistance features and investigate other missing considerations. This is investigation only. No product code, PR content, Linear status, deployment or production data was changed. Existing unrelated knowledge/source edits were preserved.

## Outcome and reasoning

All ten inspected existing feature/adjacent APIs apply source time bounds; both Hint implementations and email Suggestion branches were included. All seven new API evidence families omit source timestamp bounds. Their final eligible conversation/agent join ensures output eligibility but does not itself ensure bounded source reads. A single-node MergeTree fixture returns the identical pair with 180,002 reads unbounded, 2,050 with exact bounds and 16,386 with seven-day padding.

Equal bounds do not prove equal history or counts. Eight findings in the reports cover: unbounded new evidence; no established safe correction window; Summary's exact copied-time generation/use join; GenAI/Summary bucket anchoring; Notes/Workflow cluster switching and duplicate conversation dependency; filter-before-latest population selection and message dependency; fractional request-bound truncation; and adjacent Smart Compose attribution/adherence metadata contracts. Main and PR GenAI/Summary readers agree in focused comparisons. Known metadata-exclusion, Summary payload and Notes enum issues remain separate.

Copied conversation start is not event arrival time. An inspected upsert path permits earlier start changes without a local maximum-delta guard; this does not establish customer frequency or prove every ingestion/reindex path. Synthetic +/- fourteen-day examples are boundary tests, not observed production deltas. The neighboring 24-hour scan allowance and LateEvidence's seven-day offset do not establish a global retention/correction guarantee.

## Verification

Two passing Go query-generation configurations each capture 83 SQL variants. The final local ClickHouse 24.2.3.70 run executes 381 SQL queries with zero query errors; 107 explicit semantic/mechanism assertions pass. The diagnostic compiles pinned query builders through overlays, then executes generated SQL; most feature probes supply eligible pairs to isolate lookup behavior, while population probes execute the actual population query. This is not a full RPC/production-load benchmark.

The report was rendered in the Codex browser. All three design steps, both snapshot/change views, direct step selection, previous/next boundary behavior, and scenario/feature selectors were checked; the report console had no errors. Static HTML/JavaScript syntax, unique IDs, companion links and all sixteen source anchors were checked. Evidence: report-qa.json, verification.json, generate-queries.log, generate-main-queries.log, probe-run.log and scan-pruning-results.json.

## Next action and limits

Agree four separate contracts: exact population interval, evidence scan span, output bucket clock, and availability/retention by source. Establish correction/reindex invariants or explicitly accept a product cutoff before choosing padding; otherwise assess indexed pair/conversation lookup. Apply one justified scan policy to every physical participating source while retaining final pair intersection. Preserve legacy predicates in scan-only work; handle inherited semantic corrections as explicit separate decisions.

Production schema/version plans, source-retention equality, missing-copy prevalence, copied-time delta distribution, latency, remote/network volume, memory and concurrent load remain unverified. Measure representative 1/7/30-day windows and narrow/broad audiences before broad rollout. No follow-up Linear issues or owners were created by this investigation.

## Resource and access closure

Only synthetic local SQL was used. The disposable convi-7777-scan-audit container was stopped and removed; final unique databases were cleaned by the runners. A native read-only check confirmed neither prior assistance_scan_audit_7590be4c database remains in the shared local server; the shared container was left running. Browser QA reused an existing loopback-only report server at port 8768; that existing process was left untouched. No additional HTTP server was started. GitHub reads used existing gh/git self-authentication. No AWS, Okta, Azure or SSH credential/profile was selected or read.

## User direction after report review

Xuanyu rejected the fixed-day cutoff/expansion discussion because the existing assistance APIs do not use an additional day-based window. The report now uses requested [S,E) bounds as the source-scan baseline, removes the padded scan comparison and neighboring allowance from the proposed direction, and keeps timestamp-copy inconsistencies as separate correctness findings. The raw diagnostic results remain intact as historical evidence. The revised third design step reflects this direction; no product implementation was performed.
