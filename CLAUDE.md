# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

Personal knowledge management repository organized **by project**. Each folder at the root level is a project containing:
- Main documents (README.md, investigation docs, etc.)
- A `log/` subfolder with daily progress files

## Structure

```
<project-name>/
  README.md           # Project overview, latest state, key findings
  log/
    YYYY-MM-DD.md    # Daily progress logs for this project
  <other files>      # Investigation docs, scripts, data files
```

### Special Folders (not projects)
- `.git/`, `.claude/`, `.cursor/` - System folders
- `templates/` - Templates for new projects and logs
- `weekly-summary/` - Weekly progress/problem/plan summaries

### Current Projects

| Project | Description |
|---------|-------------|
| `general-learnings/` | Miscellaneous engineering learnings |
| `dev-environment-tips/` | macOS, PostgreSQL, tooling setup tips |
| `historic-scorecard-missing/` | Scorecard sync investigation |
| `insights-user-filter/` | User filter consolidation for Insights APIs |
| `user-filter-consolidation/` | User filter refactoring project |
| `backfill-scorecards/` | Scorecard backfill scripts |
| `qa-score-popover-fix/` | QA score popover bug fix |
| `productivity-with-ai/` | AI productivity learnings |
| `convi-6192-conversation-source-config/` | Conversation source config |
| `duplicate-template-across-usecase/` | Template duplication investigation |
| `virtual-group-filter/` | Virtual group filter |
| `2025-annual-review/` | 2025 annual review |
| `agent-stats-active-days-fix/` | Agent Leaderboard Active Days N/A bug fix (FULL OUTER JOIN) |
| `convi-6260-team-leaderboard/` | Hilton team leaderboard not breaking out sub-teams (missing child group expansion) |

## Workflow: Updating a Project

**IMPORTANT: When updating main documents of a project, always update the daily log.**

1. Check if `log/YYYY-MM-DD.md` exists for today
2. If NOT exists, create it first with this template:
   ```markdown
   # Project Log - YYYY-MM-DD

   ## Progress

   - [Summary of what was done today]

   ## Details

   [Detailed notes if needed]
   ```
3. Update the main documents
4. Update the daily log with what changed
5. Update the `README.md` log history table if significant

## Creating a New Project

1. Create folder: `mkdir <project-name> && mkdir <project-name>/log`
2. Create `README.md` with:
   - **Created/Updated** timestamps
   - **Overview** section
   - **Log History** table
3. Create first log entry in `log/YYYY-MM-DD.md`

## Documentation Standards

- Always mark creation time and update time in documents
- Log history tables in README.md should list significant dates and summaries
- Daily logs capture what was done, not exhaustive details (those go in main docs)

## Related Codebases

This repo references code investigations in these Cresta repositories (available as additional working directories):
- `go-servers` - Main backend services
- `director` - Director service
- `cresta-proto` - Protocol buffer definitions
- `config` - Configuration management
