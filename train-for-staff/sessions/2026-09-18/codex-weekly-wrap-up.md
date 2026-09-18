# Weekly wrap-up — 2026-09-14 to 2026-09-20

- **Objective:** Synthesize this week's work into a concise PPP update with evidence-backed outcomes, open risks, priorities, and performance evidence candidates.
- **Primary home:** `train-for-staff` system project; promoted output in the established `weekly-summary` destination.
- **Source repo:** `/Users/xuanyu.wang/repos/knowledge`
- **Branch/worktree:** `main`, main checkout; no worktree created.
- **Coverage cutoff:** September 18, 2026, approximately 10:32 a.m. America/Toronto. Week is September 14–20; later work is not inferred.

## Inputs and method

- Read the operating model, domain model/catalog, weekly-summary and daily-capture skills/templates, workspace repo registry, and `train-for-staff/project.yaml`. Used its declared `staff-project.md` human-facing home because this legacy project has no README.
- Recursively inventoried 11 daily logs and 21 session/decision Markdown files in range, including a nested review-evidence README. Read all daily logs; reconciled relevant work items, source-backed sessions, the applicability decision, and the #32440 review deliverable.
- Used the previous weekly summary as a compact PPP precedent and to avoid repeating prior-week delivery. Memory informed the workflow lookup only; substantive claims were grounded in current repository records and GitHub.
- Read GitHub state for director #22772/#22809/#22763, go-servers #31335/#32304/#32440/#32443/#32447/#32384/#32494, and cresta-proto #9876/#9890/#9918. Searched Xuanyu-authored PRs updated in the period to check for omissions.

## Reconciled current state

- #31335 and #22772 merged September 15; #9876 merged September 15; #32443 merged September 17. The latter supersedes the local review-time open snapshot and retains implementation credit for `xingweiy`.
- #22809 and #32304 are now non-draft open PRs. #32304 required CI passes with the code-owner check pending. #22809 has no observed failing/pending checks, but is not merged.
- #32440 remains open at `e7704f6637`, with required CI failing across three insights-server shards and code-owner check pending. Only one recorded ES EOF failure is proven on the exact base.
- #32447, authored by Kurt Choi, is open with required CI passing. Diagnosis/review does not imply source-data repair or full concurrent-update protection.
- #9890/#32384 remain open; #32384 required CI fails. #9918/#32494 remain drafts with generation/required failures. Local tests with generated proto overrides do not establish ordinary dependency readiness.
- #22763 merged September 13 and is excluded from this week's new delivery. Deployment/customer closure was not freshly verified; the September 17 RCG release exclusion is attributed to its dated source analysis.

## Synthesis decisions and boundaries

- Organized by analytics correctness, scorecard reliability, and Training Simulator delivery. Kept implementation, review, merge, deployment, and historical repair distinct.
- Retained RCG tail latency, prior auto-heal recommendation, and reporting integration as carryovers with no new completion evidence.
- Saved current PR snapshot in the synthesis rather than rewriting historical domain sessions or official ticket workflow status.
- Preserved the dirty working tree and all unrelated work. No source-code or external-system changes.

## Outputs

- [Weekly summary](../../../weekly-summary/weekly-summary-2026-09-14-to-2026-09-20.md)
- [Daily movement](../../log/2026-09-18.md)

## Validation and credentials

- New Markdown passed whitespace and local-link validation, including explicit checks of all three untracked files. Initial Markdown hard-break spaces were removed to satisfy `git diff --check`.
- Existing GitHub CLI HTTPS authentication used for read-only metadata. No AWS, Okta, Azure, SSH, or live database credentials read or used.
- No stage, commit, push, publish, or message send performed.
