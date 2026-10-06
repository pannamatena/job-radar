## ADR-0022: Hard limits on outbound requests, enforced in one shared HTTP client

**Status:** Accepted

**Context:** The tool contacts job-board APIs, company websites, Telegram and email providers on a schedule, and it's open source, so many people may run it with their own settings. A misconfigured schedule, a retry loop or a bug must never turn it into something that floods a site. Being a good citizen also keeps users from getting blocked.

**Options considered:**
1. Trust each source to behave, and document good practice.
2. Rely on the schedule alone (e.g. hourly).
3. One shared HTTP client that every part of the code must use, enforcing limits in code, whatever the user's config says.

**Decision:** Option 3. The limits:
- identify ourselves with a User-Agent;
- one request per board per run, and at least 60 minutes between fetches of the same source (see ADR-0023 for the schedule);
- per-host rate limit (about 1 request per second) and at most 2 requests in flight;
- a hard cap on requests per run;
- timeouts and response size limits;
- respect `Retry-After`, with capped exponential backoff;
- a circuit breaker for failing hosts;
- conditional requests where supported;
- robots.txt respected for ordinary web pages;
- request counts per host logged and reported.

**Trade-offs:** Users can't fetch a source more than once an hour. A busy run may stop early at the cap. In return, the tool can't harm the sites it depends on, and that can be shown with tests.

**In one sentence:** "I put the politeness rules in one place that every request has to go through, so no config mistake or bug can turn the tool into a nuisance."

---
