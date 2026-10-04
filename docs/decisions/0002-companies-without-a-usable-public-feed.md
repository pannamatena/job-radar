## ADR-0002: Companies without a usable public feed are covered by job-alert emails

**Status:** Accepted

**Context:** Some companies on a user's watch-list have no public job API (e.g. Google's own careers site), or use a system whose feed isn't clearly open to third parties. Example: SmartRecruiters (used by Etsy) documents its Posting API for customers building their own career sites, is governed by the SAP API Policy, and its robots.txt disallows automated fetching.

**Options considered:**
1. Scrape the careers pages.
2. Use the API anyway, since the data is "public".
3. Mark the company `ats: none` and cover it with that company's own job-alert emails, plus LinkedIn alerts as a backup.

**Decision:** Option 3. The tool only uses feeds that are clearly meant for this kind of use.

**Trade-offs:** Slower and less detailed than an API, and it depends on the company offering email alerts. `ats: none` companies are listed in `job-radar list --companies` so the gap is visible.

**Revisit if:** a provider clearly permits third-party use of its public feed.

**In one sentence:** "If the terms weren't clear, I treated that as a no and found a sanctioned route instead."

---
