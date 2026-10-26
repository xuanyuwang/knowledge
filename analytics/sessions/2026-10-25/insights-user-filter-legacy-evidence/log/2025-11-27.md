# Daily Engineering Notes – 2025-11-27

## 1. Fixes (Bugs / Issues Resolved)
### Problem:
We have multiple sets of user filtering tools. Each set deals with pecific set of conditons.

For example, some deal with user & group filter only, some deal with user type.

The problem is, given that many APIs are having similar requirements, it could be a good time to unify the user filter tools in one place
### Symptoms:
Hard to update the bebaviors of user filtering, because it's been patch on patch
### Root Cause:
New requirements on different APIs but eventually merged into a unified set of requirements.
### How I Diagnosed It:
Investigate the existing behaviors for each API, identify patterns, unify requirements, and propose solutions
### Final Fix:
first phase: pilot on one API to use new filter utility
2nd: rollout to other APIs that has the same pattern, also guarded by flag
### Preventative Ideas:

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
