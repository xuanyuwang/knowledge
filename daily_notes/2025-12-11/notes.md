# Daily Engineering Notes – 2025-12-11

## 1. Fixes (Bugs / Issues Resolved)
### Problem:
For a customer, the notification message displayed wrong team name for an agent
### Symptoms:
As above
### Root Cause:
Fixing the wrong team name would be complex, since the team membership is complex, like an agent can belong to many teams, directly or indirectly, with different roles.

However, we can fix it easily: doesn't display team name at all.
### How I Diagnosed It:
The tricky part is diagnosing. Since I can't reproduce it on prod because I can't generate extra data on prod, I have to reproduce it on staging.

However, I can't reproduce it on staging!

I read the code, and didn't even find where the team name is extracted and used!

I tried many ways.
1. read the code on where the notification payload is created. Nothing found about team info
2. however, when I check customer's DB, I did see that the team info is in the notification payload!
2. read the code of notification service. Nothing found about team info. I thought it was injected in the notification service.
3. thought that maybe prod has a newer version of release? Checked that prod was released last Friday (I saw the slack message was Friday)
5. there was no newer code since last Friday.
6. I read the code of related files and tried to find if there is any history code that injected team info. There is one from last month.

Finally, I checked the exact docker image running on prod, and found it was a build two monthes ago! That's why I can't reproduce it on staging, since staging has the latest `main` branch code.

At the end, investigated why last Friday's release was an image from 2 monthes ago: turned out I didn't see the slack message date close enough. It is friday, but a Friday from 2 monthes ago!

### Final Fix:
So I just need to make another release.
### Preventative Ideas:
- When the behaviors on prod and staging is different, check the release version
- release frequently. The latest code was one monthes ago and there was no release for that

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
