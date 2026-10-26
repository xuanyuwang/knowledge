# Team Enable — Claude Code Plugin & Feature Flag Cleanup

**Created:** 2026-03-26
**Updated:** 2026-03-26

## Overview

Created a Claude Code plugin for the team via the [claude-code-marketplace](https://github.com/cresta/claude-code-marketplace/pull/34), adding a shared skill usable across the company. Used the new skill to scan and clean up stale feature flags — something we knew needed doing but couldn't justify the manual cost. The plugin dramatically reduced that cost, making the cleanup feasible.

## Skill: `cleanup-feature-flag`

- Scans the codebase for stale feature flags
- Flags marked as cleanable are tracked and cleaned up via Linear tickets
- Initial scan found **17 cleanable flags**

## Key Findings

- Feature flag cleanup was a known need but too expensive to do manually
- The `cleanup-feature-flag` skill brought the cost down significantly, making it practical
- The skill is reusable across the company, not just for our team

## Status

Active

## Log History

| Date | Summary |
|------|---------|
| 2026-03-26 | Created plugin PR, ran feature flag scan & cleanup |
