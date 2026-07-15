# Domain-centered model implementation

**Date:** 2026-07-14
**Tool:** Codex
**Project:** `workspace`
**Goal:** Establish the four initial domains, a whole-repo migration plan, and shared capture/synthesis skills.

## Inputs Reviewed

- `workflow/ai-operating-model.md`
- `workflow/domain-centered-knowledge-model.md` assessment source in `workspace/sessions/2026-07-14/`
- root project/folder inventory and selected README/project files
- existing `templates/`
- existing `.claude/skills/wrap-day` and `.claude/skills/wrap-week`
- `train-for-staff` performance framework and historical weekly summaries
- skill-creator instructions and UI metadata reference

## Decisions

- Use four initial domains: `analytics`, `scorecard-workflows`, `scorecard-data-sync`, and `notifications`.
- Treat tickets as `work-items/` inside one primary domain.
- Keep initiatives only for multiple workstreams, independent coordination, or a distinct design/rollout lifecycle; duration alone is insufficient.
- Make daily capture part of substantial-task completion and prohibit implicit stage/commit/push behavior.
- Use `workflow/skills/` as the canonical cross-tool skill location. The protected `.agents/skills` path could not be created in this environment.
- Keep legacy migration synthesis-first; do not move or delete ticket folders in this change.

## Changes

- Added `workflow/domain-centered-knowledge-model.md`.
- Updated the shared operating model, root README, AGENTS/Claude adapters, and project/daily templates.
- Added `templates/work-item.md` and `templates/weekly-summary.md`.
- Created four domain project scaffolds with explicit scopes and boundaries.
- Added `workspace/deliverables/domain-reorganization-plan.md` with a disposition for every existing top-level project folder and phased exit gates.
- Created and validated `daily-capture` and `weekly-summary` skills under `workflow/skills/`.
- Converted Claude `wrap-day` and `wrap-week` to compatibility aliases without automatic Git mutation.

## Safety Findings

- `virtual-group-filter/bearer-token.txt` and `virtual-group-filter/db-connections.txt` are tracked credential/config-shaped files. Their contents were not read or copied. They require a separate authorized security review.
- `historic-scorecard-missing` contains tracked Go code and module files. Product/tool code should be relocated to an appropriate source repo or explicitly approved archive during migration.
- `clean-deprecated-user-fetcher` and `qa-metadata-check-bug` appear empty and should be removed only after reference/ignored-file verification and human approval.
- The worktree already contained extensive unrelated user changes; this implementation did not alter those project files.

## Validation

- Ran `git diff --check` during model/template updates.
- Confirmed every existing non-system top-level folder is represented in the reorganization plan.
- The skill-creator `quick_validate.py` could not run because PyYAML was unavailable and temporary dependency download was denied.
- Performed local structural validation of both skill frontmatter files and `agents/openai.yaml` metadata using Ruby YAML; both passed name, key, description, prompt, and UI-length checks.

## Follow-ups

1. Review and approve the migration matrix and security-remediation queue.
2. Pilot Analytics consolidation beginning with `agent-stats-analytics-behaviors`.
3. Exercise `daily-capture` and `weekly-summary` on the pilot and refine the templates based on actual output.
4. Create the 2026 annual evidence artifact when the first real weekly candidates are ready to promote.
