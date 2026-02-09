# Clean Up Feature Flags Skill

**Created:** 2026-02-09
**Updated:** 2026-02-09

## Overview

A reusable process (skill) for identifying and cleaning up stale feature flags in Go services. Feature flags are implemented as environment variables and controlled per cluster. A flag is considered "cleanable" when it is either enabled on all prod clusters or disabled on all prod clusters — meaning the flag no longer gates differential behavior.

### Scope

- **Initial target:** Coaching service in `go-servers`
- **Goal:** Produce a repeatable skill/process that other engineers can follow

## Approach

1. **Scan** the coaching service code for environment-variable-based feature flags
2. **Inventory** each flag: name, default value, code paths affected
3. **Check prod state** — determine which flags are uniformly enabled or disabled across all prod clusters
4. **Classify** flags as cleanable (uniform across prod) or still active (varies by cluster)
5. **Distill** the process into a reusable skill document

## Status

- [x] Scan coaching service for feature flags
- [x] Inventory flags with prod cluster states
- [x] Identify cleanable flags
- [x] Write reusable skill document

## Key Findings

- **18 envflags** found, **0 raw os.Getenv** calls
- Config service is used for customer-level ACL settings, not feature flags
- **6 flags cleanable** (3 uniformly enabled, 3 uniformly disabled/unused)
- **3 flags still vary** by cluster (not cleanable yet)
- **8 config tuning values** — optional cleanup, low priority
- See [flag-inventory.md](flag-inventory.md) for full details

## Skill Location

The reusable skill is in `go-servers` on branch `clean-up-feature-flags-skill`:
- `go-servers/.claude/skills/clean-up-feature-flags/SKILL.md`

## Log History

| Date | Summary |
|------|---------|
| 2026-02-09 | Project created; scanned all 18 flags; classified by prod state; wrote skill |
