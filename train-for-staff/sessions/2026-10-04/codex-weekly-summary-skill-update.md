# Weekly Summary Skill Update - 2026-10-04

## Context

- **Objective:** Summarize how weekly summaries are now generated and update the existing repository-local weekly-summary skill to match the compact format now in use.
- **Primary source repo:** `/Users/xuanyu.wang/repos/knowledge`
- **Branch/worktree:** `main` in `/Users/xuanyu.wang/repos/knowledge`
- **Timezone:** America/Toronto

## Inputs Reviewed

- `workflow/ai-operating-model.md`
- `workflow/domain-centered-knowledge-model.md`
- `workflow/skills/weekly-summary/SKILL.md`
- `templates/weekly-summary.md`
- The recent weekly summaries for 2026-09-21 to 2026-09-27 and 2026-09-28 to 2026-10-04
- The bundled `skill-creator` instructions

## Summary of generation process

- Weekly summaries are built from project daily logs first, then reconciled against the canonical work items for tickets that actually moved in the week.
- When a summary bullet depends on whether something is merged, open, blocked, or validating, the current PR or work-item state should be refreshed before writing the final wording.
- The compact output that has now stabilized is `Progress / Problem / Plan`, not the older long-form narrative with evidence-heavy sections.
- `Progress` works best as one ticket or one bounded work atom per bullet, with descriptive labels that say what the ticket is about instead of exposing only the raw ID.
- The final weekly artifact should describe current outcomes, not the implementation process used to reach them, and should omit inline evidence unless the user explicitly asks for it.

## Changes made

- Rewrote `workflow/skills/weekly-summary/SKILL.md` so the default output format is now the compact `Progress / Problem / Plan` structure.
- Added explicit guidance to refresh current ticket or PR status before stating merged/open/blocked outcomes.
- Added explicit guidance to use descriptive ticket labels and to avoid process-only details in the final bullets.
- Updated `templates/weekly-summary.md` to match the compact structure and remove the outdated long-form sections.

## Validation

- `git diff --check` passed after the edits.
- Manual sanity check passed: the skill frontmatter is valid-looking, the body is self-consistent with the template, and the template now matches the default format described by the skill.
- Attempted to run the bundled `skill-creator` validator via `uv`, but it could not resolve `pyyaml` because the configured private package index returned `401 Unauthorized`. No credentials were added or changed for this task.

## Credentials Used

- None.
