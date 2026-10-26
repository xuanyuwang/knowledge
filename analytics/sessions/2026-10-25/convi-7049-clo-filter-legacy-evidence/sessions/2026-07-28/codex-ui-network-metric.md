# CLO UI network metric follow-up

**Date:** 2026-07-28
**Primary source repo:** `/Users/xuanyu.wang/repos/go-servers`
**Branch/worktree context:** no source-code changes; follow-up to the performance-plan artifact

## Update

The user identified the longest network request named `qaScoreStats:retrieve` as the simplest page-load performance signal. Confirmed that the in-app browser can be controlled, although no Performance Insights tab was open during this session.

Updated the performance plan to define:

```text
qa_score_stats_critical_path_ms = max(duration of matching qaScoreStats:retrieve requests per iteration)
```

The maximum is used rather than the sum because requests may execute concurrently. The plan also records matching-request count and click-to-stable-render, excludes zero-request cache iterations from the database-backed network distribution, and retains ClickHouse/query-log measurements for root-cause diagnosis.

## Attempted execution

The user opened the NCLH Performance Insights page in Chrome and authorized filter interaction. Chrome listed the expected tab, but the browser security policy rejected claiming/controlling `https://nclh-us-east-1.cresta.com` because that site is marked as disallowed for browser automation. The restriction was not bypassed and browser control was released.

No UI measurement or database query was run. Execution requires the user to allow this site for the Codex Chrome browser integration and then retry.
