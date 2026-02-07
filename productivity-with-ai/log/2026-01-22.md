# Daily Engineering Notes – 2026-01-22

AI tools are handy but also dangerous.

It's very handy so I started to use it more and more.

Recently, I start to throw it more tools like DB query tools so that it can anlyze a bug by querying the DB directly.

But, it's not the first time that the AI tool concluded a wrong cause. It seems plausible and the PR's AI review even auto approved it.

Cautiously, I don't feel it's correct. So I aksed more questions like how the bug was introduced. Also running the new unit tests with old code. It turned out, the fix is irrelavent with the bug.

------

Another important thing is, sometimes, you can't rely on simply AI to analyze a bug from a broad context.

You need to try to narrow down the bug's possible causes, and let AI to verify your guesses.

AI is good at start a very loose start, or verify guesses. But it's not very good at analyzing bugs.