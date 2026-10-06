## ADR-0023: Check sources twice a day, retrying only what failed

**Status:** Accepted

**Context:** The first plan checked every source hourly from 07:00 to 22:00 (about 16 times a day). Company job boards rarely change more than once or twice a day, so most of those checks would find nothing new. That puts load on sites for no benefit, uses more of the free CI minutes, and makes the tool look less considerate.

**Options considered:**
1. Hourly checks during the day.
2. Twice a day, with no retries.
3. Twice a day (e.g. 08:00 and 16:00), plus a short retry run an hour later that only re-fetches sources that failed (at most twice per source per day).

**Decision:** Option 3 as the default. Users can change the times, but code enforces at least 60 minutes between fetches of the same source and at most 4 main runs a day.

**Trade-offs:** A role posted just after a check may reach the user a few hours later rather than within the hour. For job applications, same-day is fast enough. In return: about 8× fewer requests to each site, roughly 240 CI minutes a month instead of about 1,000, and failures still get a second chance without re-checking everything.

**Revisit if:** users show that being first within the hour materially matters for certain sources (it could become a per-source setting, still within the hard limits).

**In one sentence:** "I matched how often we check to how often the data actually changes, and only retried what failed."

---
