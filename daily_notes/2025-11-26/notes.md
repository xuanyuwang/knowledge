# Daily Engineering Notes – 2025-11-26

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
In Go, what's the difference between pointer to slice and a slice
### Why it was confusing:
I've always heard that slice is a pointer
### How I figured it out:
Asked AI and run an example code
### Clean explanation (my future-self will thank me):
A slice is basically a struct. If pass a slice to a function and made modification, Go will copy a struct and update the struct. Therefore, the change on the slice will be lost
### Mental model:
Slice is just a struct, which Go assigned many syntax sugar to it

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
