# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Repository Purpose

Personal knowledge management repository for:
- **Daily engineering notes** (`daily_notes/YYYY-MM-DD/notes.md`) - structured capture of fixes, learnings, surprises, and code patterns
- **Project investigations** (dedicated folders like `user-filter-consolidation/`, `historic-scorecard-missing/`) - deep-dive analysis documents for complex technical problems
- **Annual reviews** (`2025-annual-review/`) - work summaries and self-evaluations

## Commands

### Create a new daily note
```bash
./newnote.sh              # Creates today's note
./newnote.sh 2026-02-04   # Creates note for specific date
```

This creates `daily_notes/YYYY-MM-DD/notes.md` from `templates/daily.md` with an `images/` subfolder.

## Structure

```
daily_notes/
  YYYY-MM-DD/
    notes.md      # Daily notes using template
    images/       # Screenshots and diagrams
templates/
  daily.md        # Daily note template with 8 capture sections
<project-name>/   # Investigation folders with analysis documents
```

## Working with This Repo

- When creating investigation documents, organize related files in a dedicated folder with a `README.md` summarizing the project
- The daily template has 8 structured sections (from CAPTURE_CHECKLIST.md): Fixes, Learnings, Surprises, Explanations, Confusing Things, Repeated Googles, Code Patterns, Design Decisions
- Investigation folders may contain Go scripts for data analysis (see `historic-scorecard-missing/compare_scorecard_sync.go`)

## Related Codebases

This repo references code investigations in these Cresta repositories (available as additional working directories):
- `go-servers` - Main backend services
- `director` - Director service
- `cresta-proto` - Protocol buffer definitions
- `config` - Configuration management
