# CONVI-7941 final RCG Chrome test

Primary source repo: Director. Existing worktree `/Users/xuanyu.wang/repos/director-convi-7941-export-errors`, branch `xw/convi-7941-export-errors`, local HEAD `0d6a463736`. GitHub PR #24681 currently at `7b719ce470a46549e6c9691649be23f8fbe9fbb3` after main merge; GitHub diff retains same await-both handler/test. User requests final test on existing RCG Chrome tab.

## Baseline

- URL: https://rcg.cresta.com/director/insights/performance/conversations
- Selected RCI CO Inbound Service; template TEST The Royal Way - Sales & Service [AQM]; all criteria/audiences; Last30days/Weekly; exclude voicemail Yes; Criteria adherence. Progression mode Number of overwritten. Ask for the Sale count 1, October25–31.
- Loaded script `/director/s/release_director_2026-10-29-a631997/assets/index-CdQRXnE9.js`. Deployment page confirms Git hash `a631997a22fa34d2ab1723a4e2a7b660e17394ec`, tag `director-2026-10-29-2`, branch release/director/2026-10-29. Version Override initially empty, Non-release list selected.
- First export reproduced incomplete ZIP: `/Users/xuanyu.wang/Downloads/performance_progression (2).zip`, 389 bytes, contains only performance_progression_by_agent.csv. Single Ask for the Sale row, October25–31, opportunity1. Preserved original copy as `convi-7941-final-browser-test/baseline-release-2026-10-29.zip`.
- Browser download observer timed out after10s but disk file is present and parses; do not call this timeout a product failure.
- Asked user confirmation before session-only version override; user instead chose local yarn devx testing, then opened the local tab. Navigated to Deployment Config and inspected version picker without selecting a version. No setting changes yet.
- PR Chrome tab CI comment identifies production CDN preview suffix `?proxy_override=24681_merge-7b719ce`. This is the current PR commit; latest checks all passed, including both E2E shards. No PR metadata mutations.

## Local branch verification completed

- User opened http://localhost:3100/director/insights/performance/conversations. No need to start another server or install again. The existing server PID 19657 has cwd `/Users/xuanyu.wang/repos/director-convi-7941-export-errors/packages/director-app`; browser navigation shows `xw/convi-7941-export-errors` and banner `CONNECTED TO CUSTOMER DOMAIN: rcg.cresta.com`.
- Clean local HEAD `0d6a4637368fbd112a001844d569140613a8d272`; source handler contains Promise.all awaiting both CSV requests. Console source URLs also point into this worktree. This local commit predates the remote main merge; testing covers the fix at the local commit, not a production deployment or the exact merged remote head.
- Selected RCI CO Inbound Service, TEST The Royal Way - Sales & Service [AQM], all criteria/audiences, Last 30 days / Weekly, all minutes, exclude voicemail Yes, Calculation Criteria adherence, progression Number of overwritten. UI shows Ask for the Sale total1 in October25 bucket.
- First local export saved as Downloads/performance_progression (3).zip. Repeated after browser reload as performance_progression (4).zip. Both are 651 bytes and contain `performance_progression.csv` plus `performance_progression_by_agent.csv`. CRC checks pass. Both CSVs contain the Ask for the Sale row, October25–31, Num Opportunities1, Num Done0, % Done0. By-agent row matches Mikaela Almodovar / Andrea Kate Wong. Preserved as `local-first-export.zip` and `local-after-reload.zip`; manifest includes SHA256 and entry checks.
- Both browser exports completed and re-enabled the download control. Screenshot captured in the browser tool showing local branch, RCG banner, selected filters and overwritten table.
- Existing console errors: scorecard-template permissions fetch exceeds response-size limit (81580372 vs 52428800). It did not prevent either progression export. No export error observed. This unrelated request was not changed.
- No product code edits, data mutations, PR changes, version override, merge, deployment, or newly selected credentials. Used existing authenticated Chrome session. Existing local server and user tab left available.
- Result: local fix passes two real RCG browser downloads, including after reload, versus the earlier current-release baseline missing summary. Controlled regression tests remain the evidence for deliberately forced response orders; this live test did not control network ordering.

## Merge and documentation handoff

User reported merging the fix. GitHub readback confirms PR #24681 MERGED at 2026-11-06T15:29:32Z, commit `f158f92549d76f84ad6e13aaba68e2205e2f3b78`. User requested committing related documents and pushing knowledge. Preserving unrelated working changes and staging only CONVI-7941 material, including partial staging for shared README/project/daily logs. No new production verification or explicit credential selection.
