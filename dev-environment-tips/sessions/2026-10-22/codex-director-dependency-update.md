# Director dependency update workflow

Date: 2026-10-22 (America/Toronto)
Primary source repo: /Users/xuanyu.wang/repos/director
Branch/worktree: main, existing main checkout (read-only source investigation)
Knowledge domain: dev-environment-tips

## Request
Identify how to update Director dependencies, especially an action triggered from another repository.

## Investigation
- Glean search was attempted but its MCP transport failed; no Glean results were available.
- Director `.github/renovate.json` verified both locally and through GitHub: extends `local>cresta/renovate-config`.
- Shared `cresta/renovate-config/default.json` verified live: public dependencies generally have a five-day minimum release age; Cresta dependencies excluded from that rule.
- Live repository search found `cresta/action-renovatebot-execute`, described as running Renovate for Cresta repositories.
- Director Dependency Dashboard: https://github.com/cresta/director/issues/6936.
- Authentication used: gh's existing self-managed authentication. No AWS, Okta, Azure, or SSH credential was selected or read.

## Confirmed workflow
- Source: https://github.com/cresta/action-renovatebot-execute/blob/main/README.md
- Workflow: https://github.com/cresta/action-renovatebot-execute/actions/workflows/single-run.yaml
- Verified input: `repos` (required string), with optional `configfile` defaulting to `selfhosted-config/config.js`.
- Command: `gh --repo cresta/action-renovatebot-execute workflow run single-run.yaml --field repos=cresta/director`.
- UI: workflow page → Run workflow → set repos to cresta/director → leave configfile at default.
- This scans all eligible dependencies in Director; there is no package-name input in this workflow.
- For an individual rate-limited update, use its checkbox on Director Dependency Dashboard before triggering the run.
- All-repositories workflow is scheduled every two hours at minute 35.
- Director config allows automerge for @cresta packages, eslint, vite, @types and lockfile maintenance; @tiptap and prosemirror-model updates are disabled.
- At inspection, dashboard listed an existing @cresta/web-client update PR: https://github.com/cresta/director/pull/23970. This is a snapshot, not a claim of future PR state.

## Outcome and boundaries
Workflow and inputs verified from live GitHub source. No workflow dispatched, dependency changed, issue edited, or PR created. Follow-up: select the package/version if an actual update is requested.

