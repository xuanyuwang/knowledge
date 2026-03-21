---
name: commit-go-servers
description: Commit and push code for go-servers repo with linting checks. Runs gofmt and buildifier before committing.
disable-model-invocation: true
allowed-tools: Bash, Read, Glob
---

Commit and push code for the go-servers repository with pre-commit linting checks.

## Current Context
- Current directory: !`pwd`
- Git status: !`cd /Users/xuanyu.wang/repos/go-servers && git status --short`

## Process

1. **Verify go-servers repo exists**:
   - Check that `/Users/xuanyu.wang/repos/go-servers` exists
   - If not, report error and exit

2. **Check for changes**:
   - Run `cd /Users/xuanyu.wang/repos/go-servers && git status --porcelain`
   - If no changes, report "No changes to commit" and exit

3. **Run linting checks**:

   **a) Go file linting with gofmt:**
   ```bash
   cd /Users/xuanyu.wang/repos/go-servers

   # Find all modified/added Go files
   MODIFIED_GO=$(git diff --name-only HEAD | grep '\.go$' || true)
   STAGED_GO=$(git diff --name-only --cached | grep '\.go$' || true)

   # Run gofmt on each Go file
   for file in $MODIFIED_GO $STAGED_GO; do
     if [ -f "$file" ]; then
       gofmt -l -w "$file"
     fi
   done

   # Check if gofmt made changes
   git diff --name-only | grep '\.go$'
   ```

   **b) Bazel BUILD file linting with buildifier:**
   ```bash
   cd /Users/xuanyu.wang/repos/go-servers

   # Find all modified/added Bazel files
   MODIFIED_BAZEL=$(git diff --name-only HEAD | grep -E '(BUILD|WORKSPACE|\.bazel|\.bzl)$' || true)
   STAGED_BAZEL=$(git diff --name-only --cached | grep -E '(BUILD|WORKSPACE|\.bazel|\.bzl)$' || true)

   # Run buildifier on each file
   for file in $MODIFIED_BAZEL $STAGED_BAZEL; do
     if [ -f "$file" ]; then
       buildifier -mode=fix "$file"
     fi
   done

   # Check if buildifier made changes
   git diff --name-only | grep -E '(BUILD|WORKSPACE|\.bazel|\.bzl)$'
   ```

4. **If linting made changes**:
   - Report which files had linting errors and were auto-fixed
   - Show the diff of auto-fixed changes
   - Ask user to review changes before committing
   - Exit without committing

5. **If linting passes (no changes made)**:
   - Run `git status` to show current state
   - Run `git diff` to show staged and unstaged changes
   - Run `git log --oneline -10` to see recent commit style
   - Draft commit message following repo conventions
   - Stage all changes: `git add -A`
   - Create commit with message ending in `Co-Authored-By: Claude Sonnet 4.5 <noreply@anthropic.com>`
   - Run `git status` to verify commit succeeded

6. **Push to remote**:
   - Run `git push`
   - Confirm push succeeded

## Linting Tools

### gofmt (Go formatting)
- Standard Go formatter, should be available in PATH
- Automatically formats Go code to standard style
- Run with `-l` (list files) and `-w` (write changes)

### buildifier (Bazel formatting)
- Bazel BUILD file formatter
- Install if needed: `brew install buildifier` (macOS) or `go install github.com/bazelbuild/buildtools/buildifier@latest`
- Run with `-mode=fix` to auto-fix formatting issues

## Output Format

If linting passes:
```
✓ Verified go-servers repository
✓ Found changes to commit

Running linting checks...
✓ Go files: gofmt passed (N files checked)
✓ Bazel files: buildifier passed (M files checked)

[Commit message]

✓ Committed all changes
✓ Pushed to remote
```

If linting fails:
```
✗ Linting made auto-fixes

Go files with formatting issues (auto-fixed):
- path/to/file1.go
- path/to/file2.go

Bazel files with formatting issues (auto-fixed):
- path/to/BUILD

Changes made:
[show git diff of auto-fixed files]

⚠️  Auto-fixes applied. Please review changes before committing.
Run /commit-go-servers again after reviewing.
```

## Important Notes

- This skill operates on the go-servers repo at `/Users/xuanyu.wang/repos/go-servers`
- Only checks **modified files** (staged + unstaged), not the entire repo
- Auto-fixes are applied when possible (gofmt always auto-fixes, buildifier auto-fixes in `-mode=fix`)
- If auto-fixes are applied, the skill exits without committing so you can review
- After reviewing auto-fixes, run `/commit-go-servers` again to commit and push
- Never use `--no-verify` flag to skip hooks
- Follow git safety protocol: no force push, no amend unless explicitly requested
