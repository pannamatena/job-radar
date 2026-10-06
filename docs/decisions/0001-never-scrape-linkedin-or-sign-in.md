## ADR-0001: Never scrape LinkedIn or sign in as the user

**Status:** Accepted

**Context:** LinkedIn is where many roles appear first. The obvious approach is to scrape it, but LinkedIn's terms ban automated access and it actively detects it. Users are mid-job-search, so their LinkedIn account matters more than usual.

**Options considered:**
1. Scrape LinkedIn pages or automate a logged-in browser.
2. Use a third-party "LinkedIn jobs scraper" service.
3. Use LinkedIn's own job-alert emails, and read company job boards directly through their public APIs.

**Decision:** Option 3. LinkedIn content only arrives through the user's own alert emails. No browser automation, no sign-in as the user, anywhere.

**Trade-offs:** Alert emails contain less detail than the full ad, so scoring has less to work with (flagged as `limited_info`). They also arrive on LinkedIn's schedule, not ours.

**Revisit if:** LinkedIn offers an official, permitted API for personal job alerts.

**In one sentence:** "I designed around the platform's rules instead of fighting them: public job-board APIs for speed, and the user's own alert emails for LinkedIn, so nobody's account is put at risk."

---
