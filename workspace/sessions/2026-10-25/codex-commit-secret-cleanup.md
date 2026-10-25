# Commit secret-warning cleanup

- Date: 2026-10-25 (America/Toronto).
- Primary project: workspace.
- Source repo: /Users/xuanyu.wang/repos/knowledge.
- Branch/worktree: main, existing main checkout.
- Request: remove values flagged by the commit secret scanner and complete the pending commit.
- Initial state: 2,363 staged additions; pre-commit runs `betterleaks protect --staged`.
- Findings: 39 scanner findings across 19 analytics evidence snapshots, including copied Go source and review-comments JSON. These were generic password/API-key detections, without live credential validation.
- Change: removed the flagged values from the snapshots; retained the surrounding evidence. These copies are sanitized references rather than byte-identical source snapshots. Product source repositories were not edited.
- Validation: staged betterleaks scan passed after cleanup; edited review-comments JSON still parses. The normal commit hook remains enabled.
- Credentials used: none; local Git and scanner only. No push requested.
