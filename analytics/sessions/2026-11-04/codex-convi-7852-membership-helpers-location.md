# CONVI-7852 — consolidate membership helpers

Source repo: `/Users/xuanyu.wang/repos/go-servers`.
Branch/worktree: `xw/convi-7852-analytics-membership-roles`, `/Users/xuanyu.wang/repos/go-servers-convi-7852-analytics-roles`.
Starting PR #35094 head: `f05c8789a133d01cb7ee67b1382eee679124f8b0`; worktree clean and remote matched. User requested moving both functions from conversation_memberships.go into common_user_filter.go with a one-line comment for each. No behavior change. Preserve function bodies, run gofmt/Gazelle and existing parser/focused conversation tests, commit and push the existing PR. Source instructions and knowledge operating model already read this conversation.

## Outcome

Moved both helpers into common_user_filter.go immediately after convertLiteUsersToUsers, added a one-line function comment to each, moved the proto import, and deleted conversation_memberships.go. Verified the function bodies match the originals exactly. Gazelle removed only the deleted source from the library's srcs; no manual BUILD edits. Formatting and diff checks passed.

Full parser target passed in 1.6s. All five focused conversation methods passed in 8.0s with verbose confirmation, covering role metadata, cached responses, candidate-group selection, stale metadata, and unchanged Team aggregation. No new tests needed for the move. Prior full conversation suite memory limitation remains unchanged; no full rerun requested or performed for this structural edit.

Commit `1d0fc06feb61453362eedf4756e73619fec7b3c8` pushed to existing analytics PR #35094, preserving stacked base on auth. Push fast-forward with explicit lease against starting head; source worktree clean. No auth/Director changes. Git/gh used existing authentication; no AWS, Okta, Azure or SSH credential explicitly selected. No customer/production access or external messages. Post-push CI not monitored.
