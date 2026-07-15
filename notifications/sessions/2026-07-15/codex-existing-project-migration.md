# Notifications Existing-Knowledge Migration

**Date:** 2026-07-15
**Tool:** Codex
**Project:** `notifications`
**Goal:** Extract stable notification behavior from existing cross-domain projects into a canonical trigger-to-delivery model.

## Source Context

- **Primary repo:** `go-servers`
- **Repo path:** `/Users/xuanyu.wang/repos/go-servers`
- **Worktree path:** `/Users/xuanyu.wang/repos/go-servers`
- **Branch:** `main`

## Inputs Reviewed

- `oncall/sessions/2026-06-29/codex-team-slack-notification-research.md`
- `oncall/README.md` and `project.yaml`
- notification-related sections in `scorecard-permission-policy/`
- notification searches in `group-calibration/` and `export-appeal-comments/`

## Findings

- Operational job routing, on-call identity, and destination channel selection are separate concerns.
- PagerDuty/oncall-sync should remain the source of truth for who is currently on call.
- GroundCover/Grafana service-health paging should route through service/priority policies; direct Slack is a separate broadcast choice.
- Scorecard notification eligibility depends on template revision, static roles, runtime state, recipient relationship, and submit/publish behavior.
- No notification-specific evidence was found in the current group-calibration and appeal-export artifacts; the catalog records this as a gap.

## Decisions

- Retain `oncall` as the incident operating surface and cross-link its research.
- Retain scorecard permission truth in `scorecard-workflows`; Notifications owns the recipient/delivery consumption view.
- Create an explicit catalog with evidence status instead of presenting incomplete inventory as exhaustive.

## Validation

- Cross-checked each catalog row against migrated source evidence.
- Marked handler/channel and retry/idempotency areas as incomplete where evidence was insufficient.
- Parsed changed YAML and verified relative Markdown links and whitespace.
