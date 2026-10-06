## ADR-0021: Terminal and config files in V1; a graphical setup page in V2

**Status:** Accepted

**Context:** Non-developers will find a terminal, YAML files and GitHub Actions hard.

**Options considered:**
1. Terminal only, with a guided `init` command and detailed docs.
2. A local setup page in the browser (`job-radar setup`) that writes the same config files.
3. A hosted web app (ruled out, see ADR-0015).

**Decision:** Option 1 for V1, keeping the config format stable and documented so option 2 can be added in V2 without changing the engine.

**Trade-offs:** V1 is harder for non-developers. To compensate, there are beginner-level docs with screenshots, and a "friend sets it up from the docs alone" test before release.

**In one sentence:** "I shipped the engine first and designed the config so a friendlier interface could sit on top later without a rewrite."

---
