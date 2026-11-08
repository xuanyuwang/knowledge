# CONVI-7810 bulk export requests

Source repo: `/Users/xuanyu.wang/repos/cresta-proto`; worktree: `/Users/xuanyu.wang/repos/cresta-proto-training-simulator-csv`; branch: `xw/convi-7810-training-simulator-export-protos`.

PR: https://github.com/cresta/cresta-proto/pull/10413

The user clarified that the download button exports multiple items loaded by a table. The frontend supplies those selected session, lesson, or module resource names; the backend resolves matching sessions and module attempts. Replace the three singular target fields with required repeated names, require at least one nonempty name, and keep current filters. Preserve all retakes, historical rule-column union, blank cells for absent rules, and distinct recorded N/A values. This supersedes the earlier per-row download assumption. Contract only; no backend or UI implementation.

Pushed commit `a079fd34e85b90e1dd10864288f53f0250ab1a17` to PR #10413. Repository-wide `mage -v lint` and `mage -v apiLint`, focused Buf lint/build, focused compatibility with imports excluded, formatting, Gazelle, and the Training Simulator Bazel build passed. Compatibility with imports included flags newer permission enum entries from main absent on the older branch; those imported files are unchanged. Discarded unrelated Gazelle output. GitHub confirms only the service proto and package BUILD file are in the PR, and reports no merge conflict. CI is pending on the new head. Git and gh used existing authentication; no credential was explicitly selected.
