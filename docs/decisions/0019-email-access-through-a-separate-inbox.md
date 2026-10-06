## ADR-0019: Email access through a separate inbox, with alerts auto-forwarded

**Status:** Accepted

**Context:** Reading job-alert emails needs an app password. Google describes it as access to the Google Account. In practice it can read **and send** email, the inbox can be used to reset other passwords, and it bypasses 2-Step Verification.

**Options considered:**
1. Use the user's main inbox.
2. Use OAuth with Gmail's API: needs Google's app verification for sensitive scopes, which is heavy for a small open-source tool, and the scopes still cover the whole mailbox.
3. A separate free inbox just for job-radar, with a filter in the main inbox that **auto-forwards copies** of job-alert emails to it.

**Decision:** Recommend option 3, and allow option 1 with the risk stated plainly in the user notice.

**Trade-offs:** About 5 minutes of extra setup. In return, the tool never holds the keys to the user's main account, and nothing changes day to day (alerts still arrive in the main inbox).

**In one sentence:** "I limited what a leaked credential could reach, without changing anything about how the user reads their email."

---
