## ADR-0015: Two repos per user (public code, private config), run on GitHub Actions

**Status:** Accepted

**Context:** The tool has to run on a schedule without the user's computer, for free, without anyone (including me) hosting other people's data.

**Options considered:**
1. A hosted web service people sign up to: easiest for users, but it costs money to run and means holding other people's CVs, with the privacy and legal responsibility that brings.
2. Users run it on their own computers: free, but it stops when the laptop is off.
3. GitHub Actions in a **public fork**: free, but it would expose each user's profile and database to the world.
4. A **public code repo** plus each user's **private config repo** created from a template, whose scheduled workflow downloads the released binary and runs it.

**Decision:** Option 4.

**Trade-offs:**
- Setup needs a GitHub account and a few steps (a V2 setup page will help).
- Private repos have a free monthly Actions allowance (measured to fit a typical schedule).
- Scheduled runs can be delayed when GitHub is busy.
- GitHub disables schedules after 60 days without repo activity, so there's a safeguard and a warning.

**In one sentence:** "Every user runs their own copy in their own private repo, so it's free, there's no server, and I never hold anyone's data."

---
