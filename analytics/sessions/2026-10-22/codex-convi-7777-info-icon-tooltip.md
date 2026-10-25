# CONVI-7777 info icon tooltip revision

- Source repo: `/Users/xuanyu.wang/repos/director`.
- Worktree: `/Users/xuanyu.wang/repos/director-convi-7777-assistance-usage`.
- Branch: `xw/convi-7777-assistance-usage`, PR #24020.
- User approved the definition followed by ten feature bullets, using table feature names without percent/per-day suffixes. Explicitly remove calculation and exclusion details.
- Match Overall Engagement's Flex + InfoIcon (size xs), share content between Agent and Team tables, keep headerDisplayName for column settings. Remove now-unused whole-header tooltip support from DirectorTable.
- Focused existing hook tests and app TypeScript pass. Initial test invocation hit Node experimental localStorage behavior before collecting tests; rerun with NODE_OPTIONS=--no-experimental-webstorage passes.
- Translation extraction wrapper could not locate rimraf; invoke the same Babel full extraction directly. Full Babel extraction passes (7,321 files); only the intended Insights locale changes remain. All commit hooks pass (lint, i18n lint, extraction, formatting). Pushed `be80b16b00` to #24020. Browser visual QA not performed; remote CI pending.

- Used self-authenticating git/gh; no explicit credential/profile selected.

## Column width follow-up

- User reported info icon spilling into the next column. Chrome DOM measured the old Team column at 80px with icon right=801.37 beyond cell right=796.
- Increased only the conversation usage metric width to 120px in Agent and Team, including sticky group width accounting. Chrome confirms info and sort icons contained in both table headers; tooltip contents open correctly.
- Updated existing hidden-width test from 80 to 120 to match the new metric width. All 14 focused tests and commit hooks pass. Pushed `3276e18c20` to #24020. Screenshot: `deliverables/convi-7777-tooltip-width-fixed.jpg`.
