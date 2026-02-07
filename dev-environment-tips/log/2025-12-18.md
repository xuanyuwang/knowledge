# Daily Engineering Notes – 2025-12-18

## 1. Fixes (Bugs / Issues Resolved)

# macOS PostgreSQL Shared Memory Issue

## Problem Summary

When running Go tests that use embedded PostgreSQL on macOS, you may encounter this error:

```
FATAL: could not create shared memory segment: No space left on device
DETAIL: Failed system call was shmget(key=9728364, size=56, 03600).
```

**This is NOT a disk space issue** - it's a macOS kernel limitation on System V shared memory segments.

## Root Cause

### macOS Shared Memory Limits

macOS has strict, **hardcoded kernel limits** that cannot be changed:

```bash
sysctl kern.sysv.shmmni kern.sysv.shmmax kern.sysv.shmall
```

- `kern.sysv.shmmni: 32` - **Only 32 shared memory segments allowed system-wide**
- `kern.sysv.shmmax: 4194304` - Max 4MB per segment
- `kern.sysv.shmall: 1024` - Very low total pages

### Why Tests Fail

1. **Each embedded PostgreSQL instance needs multiple shared memory segments** for:
   - Shared buffers
   - Lock tables
   - Transaction state
   - Internal data structures

2. **Parallel test execution multiplies the problem**:
   - Bazel runs 6 test shards in parallel
   - 6 PostgreSQL instances × ~5 segments each = 30+ segments
   - Easily exceeds the 32 segment limit

3. **Orphaned processes accumulate**:
   - Failed test runs leave PostgreSQL processes running
   - These hold onto shared memory segments indefinitely
   - Eventually exhausts all 32 available slots

## Solutions

### Quick Fix (Clean Up Orphans)

```bash
# 1. Kill orphaned PostgreSQL processes
pkill -u $(whoami) -f "embedded-postgres-go"

# 2. Clean up orphaned shared memory segments
ipcs -m | grep $(whoami) | awk '{print $2}' | xargs -n1 ipcrm -m 2>/dev/null

# 3. Restart colima (if using for Docker/ClickHouse)
colima restart
```

### Run Tests Sequentially

Instead of Bazel (which parallelizes), use `go test` directly:

```bash
cd apiserver/internal/coaching
export DOCKER_HOST=unix:///Users/$(whoami)/.colima/default/docker.sock
export REUSABLE_CLICKHOUSE_CONTAINER_NAME=go-servers-local-clickhouse-node1
go test -v -run "^TestCreateScorecard$" -timeout 10m
```

This creates only one PostgreSQL instance at a time.

### Use External PostgreSQL (Recommended)

Set up a local PostgreSQL server once and reuse it:

```bash
# 1. Install PostgreSQL (if not already installed)
brew install postgresql@15

# 2. Start PostgreSQL service
brew services start postgresql@15

# 3. Create test database
createdb -U postgres cresta_test

# 4. Set environment variable
export TEST_DATABASE_URL=postgres://postgres@127.0.0.1:5432/cresta_test

# 5. Add to your shell config
echo 'export TEST_DATABASE_URL=postgres://postgres@127.0.0.1:5432/cresta_test' >> ~/.zshrc

# 6. Add to Bazel config
echo 'test --test_env=TEST_DATABASE_URL=postgres://postgres@127.0.0.1:5432/cresta_test' >> ~/.bazelrc
```

This avoids embedded PostgreSQL entirely and bypasses the shared memory limit.

## Prevention

### Add to Your Shell Startup

Add this function to `~/.zshrc` or `~/.bashrc`:

```bash
# Clean up orphaned PostgreSQL test processes and shared memory
cleanup_postgres_tests() {
    echo "Killing orphaned PostgreSQL processes..."
    pkill -u $(whoami) -f "embedded-postgres-go"

    echo "Cleaning up shared memory segments..."
    ipcs -m | grep $(whoami) | awk '{print $2}' | xargs -n1 ipcrm -m 2>/dev/null

    echo "Cleanup complete!"
}
```

Run `cleanup_postgres_tests` before running tests or when you encounter the error.

### Monitor Shared Memory Usage

Check current usage:

```bash
ipcs -m | wc -l
```

If this shows close to 32 segments, clean up before running tests.

## Why This Only Affects macOS

- **Linux**: `SHMMNI` is typically 4096+ and can be adjusted via sysctl
- **macOS**: Limits are hardcoded in the kernel and cannot be changed without kernel recompilation

## Technical Details

### What is System V Shared Memory?

- Legacy IPC mechanism from Unix System V
- PostgreSQL uses it for inter-process communication
- macOS inherited strict limits from BSD
- Modern alternatives (POSIX shared memory) aren't used by PostgreSQL for historical compatibility reasons

### Segment Allocation

When PostgreSQL initializes, it calls:
```c
shmget(key, size, IPC_CREAT | 0600)
```

This fails when all 32 segment IDs are taken, even if each segment is small.

## Related Resources

- [PostgreSQL Shared Memory Documentation](https://www.postgresql.org/docs/current/kernel-resources.html#LINUX-MEMORY-OVERCOMMIT)
- [macOS sysctl Documentation](https://developer.apple.com/library/archive/documentation/System/Conceptual/ManPages_iPhoneOS/man3/sysctl.3.html)
- [go-servers CLAUDE.md - Database Setup](../CLAUDE.md#database-and-integration)

## Last Updated

2025-12-18


## 2. Learnings (New Knowledge)
### What I learned:
### Context:
### Why it's important:
### Example:
### When to apply:

## 3. Surprises (Unexpected Behavior)
### What surprised me:
### Expected vs actual behavior:
### Why it happened:
### Takeaway:

## 4. Explanations I Gave
### Who I explained to (team / code review / slack):
### Topic:
### Summary of explanation:
### Key concepts clarified:
### Possible blog angle:

## 5. Confusing Things (First Confusion → Later Clarity)
### What was confusing:
### Why it was confusing:
### How I figured it out:
### Clean explanation (my future-self will thank me):
### Mental model:

## 6. Things I Googled Multiple Times
### Search topic:
### Why I kept forgetting:
### Clean “final answer”:
### Snippet / Command / Example:

## 7. Code Patterns I Used Today
### Pattern name:
### Situation:
### Code example:
### When this pattern works best:
### Pitfalls:

## 8. Design Decisions / Tradeoffs
### Problem being solved:
### Options considered:
### Decision made:
### Tradeoffs:
### Why this matters at a system level:
### Future considerations:

---

## Screenshots
(Drag & paste images here)

## Raw Snippets / Logs
\`\`\`
Paste raw logs, stack traces, or snippets here
\`\`\`

## Blog Potential
### Short post ideas:
### Deep-dive post ideas:
