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

     ## Progresses

     ### Project Name 1
     - [Key accomplishment 1]
     - [Key accomplishment 2]
     - [Current status]

     ### Project Name 2
     - [Key accomplishment 1]
     - [Key accomplishment 2]
     - [Current status]

     ## Problems

     ### Technical Issues
     - [Issue 1 and resolution status]
     - [Issue 2 and resolution status]

     ### Blockers
     - [Blocker 1 and mitigation plan]
     - [Blocker 2 and mitigation plan]

     ### Learnings from Failures
     - [What went wrong and why]
     - [What was learned]

     ## Plan

     ### Next Week Priorities
     1. [High priority item 1]
     2. [High priority item 2]
     3. [High priority item 3]

     ### Follow-ups Required
     - [Follow-up action 1]
     - [Follow-up action 2]

     ### Pending Reviews/Decisions
     - [Pending item 1]
     - [Pending item 2]
     ```

4. **Populate the summary**:
   - **Progresses**: Group activities by project, highlight key accomplishments (not every commit), note current status
   - **Problems**: Document technical issues, blockers, and learnings from failures encountered this week
   - **Plan**: List next week's priorities, required follow-ups, and pending reviews/decisions

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

- **Progresses**: Focus on significant accomplishments, not every detail. Only include projects with actual work this week.
- **Problems**: Be honest about issues encountered. Include what went wrong, root causes, and resolutions (or current mitigation plans).
- **Plan**: Prioritize realistically. Distinguish between high-priority work, follow-ups, and items awaiting decisions.
- Keep it concise - aim for 1-2 pages max
- If no work was done this week, create a minimal summary noting that
