---
name: wrap-day
description: Wrap up a day's work by creating daily logs for all projects with uncommitted changes, committing everything, and pushing to remote
disable-model-invocation: true
allowed-tools: Bash, Read, Write, Glob
---

Wrap up today's work across all projects in the knowledge repository.

## Current State
- Today's date: !`date +%Y-%m-%d`
- Projects with uncommitted changes: !`cd /Users/xuanyu.wang/repos/knowledge && git status --porcelain | awk '{print $2}' | cut -d'/' -f1 | sort -u | grep -v '^\..*' | grep -v '^weekly-summary' | grep -v '^templates' | grep -v '^blog' | grep -v '^train-for-staff'`

## Process

For each project directory with uncommitted changes:

1. **Check if daily log exists** for today (`log/YYYY-MM-DD.md`)
   - If it does NOT exist, create it with this template:
     ```markdown
     # Project Log - YYYY-MM-DD

     ## Progress

     - [Summary of what was done today]

     ## Details

     [Add detailed notes if needed]
     ```

2. **Update the daily log** with a summary of changes:
   - Run `git diff` for the project directory
   - Summarize what changed in 1-3 bullet points
   - Add the summary to the "Progress" section

3. **After processing ALL projects**, create a single commit:
   - Stage all changes: `git add -A`
   - Create commit with message: `wrap up today`
   - Add co-author line: `Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>`

4. **Push to remote**:
   - Run `git push`

## Output Format

Show a summary at the end:
```
Wrapped up work for YYYY-MM-DD:
- Project 1: [brief summary]
- Project 2: [brief summary]
...

✅ Committed and pushed all changes
```

## Important Notes

- Only process project folders (ignore `.git`, `.claude`, `templates`, `weekly-summary`, `blog`, `train-for-staff`)
- If there are no uncommitted changes, report "No uncommitted work to wrap up"
- Always create logs BEFORE committing
- Use a single commit for all changes with message "wrap up today"
