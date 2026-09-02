# Training Simulator statistics API boundary reopened

Date: 2026-08-20
Source repo: `/Users/xuanyu.wang/repos/knowledge`
Branch/worktree context: main checkout; no knowledge worktree

## Goal

Refactor the local API design after reviewer discussion without changing the live Superhuman document.

## Updated review state

- The API boundary is open again.
- Reviewers currently prefer option 2 (add optional statistics to the lesson/module list APIs); option 1 is the other reuse choice.
- Option 3 (two dedicated statistics APIs) retains the cleanest boundaries.
- Complexity risk is lower than in broad Analytics APIs because the expected consumers remain within the Training Simulator page.
- Every option continues to use direct, filtered database aggregation rather than aggregating a list response.

## Documentation changes

- Replaced the selected-Option-3 framing with three equal option subsections.
- Each option now contains a high-level implementation plan, protobuf example, pros, and cons.
- Updated the backend, frontend, local Superhuman draft, project README, work item, and decision record so they no longer describe Option 3 as final.
- Kept the frontend plan transport-neutral: option 2 combines content and statistics; options 1 and 3 retain batch statistics loading.
- Did not modify the live Superhuman/Coda page.

## Validation

- Markdown code fences are balanced in both canonical API-design copies.
- `git diff --check` passes for the Training Simulator knowledge files.

