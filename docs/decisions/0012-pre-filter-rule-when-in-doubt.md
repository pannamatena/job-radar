## ADR-0012: Pre-filter rule: when in doubt, keep the posting

**Status:** Accepted

**Context:** A cheap, deterministic filter runs before any scoring, to drop obvious misses (junior titles, non-software engineering, locations clearly out of range).

**Decision:** The filter only drops postings it's sure about. If in doubt, it keeps them. A posting that doesn't state its work model is **never** dropped for that reason.

**Trade-offs:** More postings reach the scorer, which costs a little more when the AI is on.

**Why:** the two mistakes aren't equal. A wrongly dropped role is never seen, while a wrongly kept one costs one cheap check.

**In one sentence:** "I matched the filter's errors to their real cost: missing a role is far worse than scoring one extra."

---
