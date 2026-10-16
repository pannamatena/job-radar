## ADR-0032: The AI scorer coaches by default ("you can do this, and here's how")

**Status:** Accepted

**Context:** Left alone, a language model reads a job ad like a gatekeeper: every unmet requirement counts against you, and "essential" means a no. My own testing showed it dropping a role I would clearly apply for to a 2 because it asked for a business domain and SQL, both things I can learn or already half-know. Most job seekers under-apply, so a harsh scorer hides roles they'd have got. I fixed this in my own rubric first, but a new user won't think to write that, and the tool should work well for them without it.

**Options considered:**
1. Leave it to each user's `rubric.md`: flexible, but most people won't write it, so the default experience stays harsh.
2. Put the coaching stance in the built-in instructions, and keep the hard blockers in the user's rubric.
3. Make every score generous by lowering the thresholds: simple, but it throws away the honest "this really isn't for you".

**Decision:** Option 2. The built-in system prompt tells the model to act as a coach who believes the candidate can do the job: ask "could they credibly do this after a few weeks of preparation?", treat quickly learnable gaps as small even when the ad says "essential", and in `gap` say plainly what's missing and, in a few words, how to close it or talk about it using only what the profile says. Hard blockers (eligibility, location, the main language of an IC role, specialisms that take years; see [ADR-0033](0033-years-of-experience-are-a-soft-signal.md) for years) stay in the user's rubric and still score 1–2. The example rubric and the setup guide say the same, so users understand why scores read this way and can tighten it.

**Trade-offs:** More borderline roles reach the user, so a few more "wrong alerts" are expected (on my 14 cases: 0 to 3 over the tuning, with missed good roles going from 3 to 0). A user who wants a strict scorer must say so in their rubric. The wording is one more thing that changes the prompt version of everyone's scores.

**Why:** a missed good role is never seen, a role scored a little generously costs one look (the same reasoning as [ADR-0012](0012-pre-filter-rule-when-in-doubt.md) and [ADR-0028](0028-bare-region-hybrid-is-neutral.md)). Honesty is unchanged: the model still never invents experience.

**Evidence:** on the 14 cases, with the built-in stance and a rubric that had no coaching section at all: exact match 75%, no wrong alerts, one missed good role, and the role that had been scored 2 now scored 3 in two of three repeats. These cases were used for tuning, so this shows the stance works without the rubric text, not how it does on unseen ads.

**In one sentence:** "The scorer's default is 'you can do this, and here's how', and hard limits are yours to set."
