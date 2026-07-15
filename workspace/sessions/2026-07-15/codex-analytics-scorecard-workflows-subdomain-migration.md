# Analytics and Scorecard Workflows Subdomain Migration

## Objective

Apply the approved two-level domain model to Analytics and Scorecard Workflows, synthesize existing knowledge, and preserve ticket/project evidence.

## Source Context

- Primary project: `workspace`
- Source repo: `/Users/xuanyu.wang/repos/knowledge`
- Branch/worktree context: `main` in the primary checkout
- Date/time zone: 2026-07-15, America/Toronto

## Decisions Applied

- Broad ownership surfaces remain top-level domain families.
- Durable capability and workflow knowledge lives under `subdomains/<name>/`.
- Work items, sessions, logs, and decisions remain at the parent domain.
- A future work item may name one optional primary subdomain.
- Cross-cutting truth stays at the nearest common parent.
- Legacy folders remain evidence and receive pointers rather than being deleted.

## Outcomes

- Added eight Analytics subdomains: Shared Analytics Platform, Performance Insights, Leaderboard, Active Days, Quintiles, QA Score, Insights User Filter, and Conversation Volume.
- Added seven Scorecard Workflows subdomains: Template Authoring and Versioning, Evaluation and Scoring, Scorecard Lifecycle, Permissions and Visibility, Appeals, Group Calibration, and Process Scorecards and Generation.
- Added a legacy-source index, parent-domain log, and migration session for each domain.
- Added navigation pointers to the retained legacy sources, including minimal READMEs where none existed.
- Updated the shared operating model, domain model, templates, tool adapters, root navigation, and migration plan to make subdomains a supported repository convention.

## Impact

- Domain mastery can grow by stable product capability/workflow rather than by an expanding list of ticket projects.
- Multi-week and cross-subdomain tickets keep one recoverable execution history while still promoting durable findings into the right semantic reference.
- Analytics count/filter/score distinctions and Scorecard Workflow boundaries are now discoverable without reconstructing every ticket chronologically.

## Safety

- No legacy content was deleted.
- Existing unrelated `workspace/repos.yaml` changes were preserved.
- Credential-shaped files under `virtual-group-filter` were not read or copied.
- No files were staged, committed, or pushed.

## Follow-up

- Validate YAML, Markdown links, whitespace, and scoped diffs.
- Expand the open semantic/state matrices incrementally from future ticket evidence.
- Review security-sensitive credentials and misplaced product code only as separately authorized work.
