# QM reliability infrastructure exploration

**Status:** initial proposal; awaiting prioritization discussion
**Primary domain:** oncall
**Primary subdomain:** none
**Official ticket:** none
**Last updated:** 2026-09-23

## Objective and scope

Develop high-level inspiration for coaching and analytics monitoring, alerts, automated diagnosis/repair, latency improvement, and end-to-end testing. Role: design synthesis. No implementation or external changes requested.

## Source context

- Primary source repo/worktree: `/Users/xuanyu.wang/repos/go-servers`, local branch `main` (read-only context; no source audit).
- Knowledge checkout: `/Users/xuanyu.wang/repos/knowledge`.

## Current understanding

Propose starting with access explainability, scorecard freshness/correctness, and a small end-to-end suite. Add service health and selected journey tracing, actionable alerts, and automated read-only diagnosis; use evidence to prioritize optimization and bounded repair. Priorities are proposed, not accepted commitments.

## Validation and next actions

Proposal draws on user-reported access-question frequency and existing local knowledge. Production state, existing tool coverage, and incident frequency remain unverified. Next: select one coaching read path and one analytics report, confirm recent incident categories and existing instrumentation, and establish baseline success measures and owners.

## Timeline

- 2026-09-23 — Saved [initial proposal](../deliverables/qm-reliability-infrastructure.md) and [session evidence](../sessions/2026-09-23/codex-qm-reliability-inspiration.md). No product changes.
