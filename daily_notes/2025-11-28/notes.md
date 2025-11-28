# Daily Engineering Notes – 2025-11-28

## 1. Fixes (Bugs / Issues Resolved)
### Problem:
### Symptoms:
### Root Cause:
### How I Diagnosed It:
### Final Fix:
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
We need to support an options of filtering user with specific roles ONLY.

Current filter API is support `OR` for roles, like `Roles = [A, B]` will return all users have either A or B.
### Options considered:
1. Add another flag to the API like `RolesOnly`
2. Add post-process tools
### Decision made:
Add post-process tools.
### Tradeoffs:
Another function call, instead of a flag.
But the real problem of option 1 is breaking orthogonality.
Existing filter options are orthogonal: each filter controls one indepent aspect.
We’d better keep orthogonality of flags in an API, which is: not using multiple flags to control the same attribute, especially when there could be conflicts.

There are already `UserFilterConditions.Roles` and `UserFilterConditions.GroupRoles`, which controls “users that have any role in Roles/GroupRoles”. If we add a flag like `ListAgentOnly`, then it’s potentially conflict with `Roles/GroupRoles` : what’s the expected behavior when `Roles = [Agent, Manager]` and `ListAgentOnly = true`? Of course, we can define a behavior for that combination. We can define behaviors for any combination. The point is that it’s not intuitive, low readability, error prone, and high maintenance cost.
### Why this matters at a system level:
We've seen the problem of not having orthogonality in many APIs: there are so many flags that potentially overlapping with each other. You really have zero confidence when use them. Each time use them, you need to read the doc again and test again, to make sure there is no surprise for a speicific combination of flags
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
