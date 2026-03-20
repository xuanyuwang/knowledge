---
name: wrap-week
description: Wrap up the week's work by creating a weekly summary document under the weekly-summary project
disable-model-invocation: true
allowed-tools: Bash, Read, Write, Glob
---

Create a weekly summary document for the current week.

## Current Context
- Today's date: !`date +%Y-%m-%d`
- Current week: !`date +%Y-W%U`
- Recent project activity: !`cd /Users/xuanyu.wang/repos/knowledge && find . -maxdepth 2 -name "log" -type d | head -20`

## Process

1. **Determine the week range**:
   - Calculate Sunday of current week (start date)
   - Calculate Saturday of current week (end date)
   - Format: `YYYY-MM-DD` (e.g., `2026-03-16` to `2026-03-22`)
   - Include all 7 days (Sunday through Saturday)

2. **Gather weekly activity** by checking all project logs:
   - For each project folder (excluding special folders)
   - Check `log/` directory for entries from this week (Sunday-Saturday)
   - Collect progress summaries from each day's log, including weekend work

3. **Create weekly summary file**:
   - Path: `/Users/xuanyu.wang/repos/knowledge/weekly-summary/weekly-summary-YYYY-MM-DD-to-DD.md`
   - Example: `weekly-summary-2026-03-16-to-22.md` (for week March 16-22)
   - Use this template:
     ```markdown
     # Weekly Summary - Week of YYYY-MM-DD

     **Created:** YYYY-MM-DD

     ## Overview

     [Brief 2-3 sentence overview of the week's work]

     ## Projects Worked On

     ### Project Name 1
     - [Key accomplishment 1]
     - [Key accomplishment 2]

     ### Project Name 2
     - [Key accomplishment 1]
     - [Key accomplishment 2]

     ## Key Learnings

     - [Learning 1]
     - [Learning 2]

     ## Next Week

     - [Priority 1]
     - [Priority 2]

     ## Notes

     [Any additional context or observations]
     ```

4. **Populate the summary**:
   - Group activities by project
   - Highlight key accomplishments (not every commit)
   - Extract learnings or insights from the work
   - Suggest next week's priorities based on current state

5. **Commit the weekly summary**:
   - Stage: `git add weekly-summary/`
   - Commit with message: `weekly summary YYYY-MM-DD to DD` (e.g., `weekly summary 2026-03-16 to 22`)
   - Add co-author line: `Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>`
   - Push: `git push`

## Output Format

Show the weekly summary content and confirm:
```
Created weekly summary for YYYY-MM-DD to DD

[Show summary content]

✅ Committed and pushed weekly summary
```

## Important Notes

- Focus on significant progress, not every detail
- Extract patterns and learnings across projects
- Keep it concise - aim for 1-2 pages max
- Only include projects with actual work this week
- If no work was done this week, create a minimal summary noting that
