# CONVI-7785 PR review evidence

- **Source repo:** `/Users/xuanyu.wang/repos/director`
- **Branch/worktree:** `xw/convi-7785-exclude-simulator-rules` at `/Users/xuanyu.wang/repos/director-convi-7785`
- **PR:** [director#23461](https://github.com/cresta/director/pull/23461), deployed preview head `d9de9e190265d5cb304182cc61f7304959161cc1`
- **Date:** 2026-10-07

## Live comparison

On Walter Dev, the activated Opera rule `CONVI-7785 Simulator-only QA rule` owns behavior `CONVI-7785 Simulator-only QA behavior` and is scoped to **Only Training Simulator**. The rule is at `/director/opera/rule/01a11823-8d7f-7e9e-9c0d-21a178241ecd/edit`.

The baseline Director build offered 176 behavior choices in the conversation template AutoQA picker and allowed selection of that behavior. The PR preview banner identified `23461_merge-d9de9e1`; that build offered 175 choices and omitted the Simulator-only behavior while retaining other choices. The navigation was Admin → Performance Config → Performance templates → Add new → Conversation Template → Add Criterion → Automated scoring → Behavior. The baseline/preview contrast was verified in the browser but the first screen recordings ended before the picker result, so they are unsuitable PR evidence.

The preview override is sticky. Removing `?proxy_override=` alone did not restore baseline. The preview banner's **here** button cleared it. On a subsequent reload, an override mismatch banner showed cookie `23461_merge-d9de9e1` while the viewed build was `main-aa0f0c8`; clicking **here** removed the banner and restored baseline.

## PR state and remaining work

At the start of this continuation, PR #23461 remained draft at the deployed preview SHA. `qa-metadata-check` failed because no QA request was selected; all other completed required CI jobs succeeded except a pending Codeowner status. On 2026-10-07, the live PR description was updated after a fresh `updatedAt` check to select `smoke-functional` and add the Walter Dev environment, feature flag `none`, exact steps, expected behavior, and failure case. The PR remains draft and has no video attachments. The new QA metadata run `37690206999` failed only because the Before and After fields lack videos.

The recording skill requires full-screen Before/After videos showing URL, navigation, and the readable picker result. A replacement recording attempt was stopped when native Chrome control stopped responding consistently (`noWindowsAvailable` for coordinate clicks, stale focused elements after index clicks and keyboard input). No incomplete recording was uploaded. Continue with verified clips, upload each to the matching PR field as a bare GitHub attachment URL, choose `smoke-functional`, fill exact steps/expected/breakage/flag/environment, check rendered players and QA metadata, then mark ready.

No specific credential or profile was selected. `gh`, `git`, and the existing Chrome session authenticated themselves.
