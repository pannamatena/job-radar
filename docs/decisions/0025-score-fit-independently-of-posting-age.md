## ADR-0025: Score fit independently of posting age; alert on "new to you", not "newly posted"

**Status:** Accepted

**Context:** On its first Monday, the interim scheduled radar found a role it described as a strong fit (remote EU, TypeScript/React/Node, 4–6 reports, Ireland eligible). It filed it under "near misses" only because the ad had been open since March 2024. The instruction "report new postings only" had quietly mixed two different questions, so a strong fit was effectively hidden. Long-open ("evergreen") ads usually mean a company hires for that role continuously, so they're often good leads.

**Options considered:**
1. Only report postings published recently (the original rule).
2. Report everything open every time (noisy, repeats).
3. Treat **fit** and **freshness** as separate: score fit on the role alone, and alert when a strong fit is **new to the user** (first time seen), whatever its posting date. Show the posting date and flag evergreen ads.

**Decision:** Option 3. In the Go tool, "new to the user" comes from the `postings` table, so each job is alerted once. The interim radar, which has no memory, keeps a list of roles already reported and has a separate "Strong fits that aren't new" section.

**Trade-offs:** The first run for a new user or a newly added company surfaces many older roles at once, so it's sent as one batch summary. Some old ads may be closed but not taken down; the alert shows the posting date so the user can judge.

**In one sentence:** "A real-world miss showed I'd tied relevance to recency, so I separated 'is it a good fit?' from 'is it new to me?', and turned the miss into a test case."
