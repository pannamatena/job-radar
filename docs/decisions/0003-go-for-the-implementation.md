## ADR-0003: Go for the implementation

**Status:** Accepted

**Context:** The tool needs to run unattended on a schedule, be easy for non-developers to install, and be a learning project for me.

**Options considered:**
1. TypeScript/Node (my strongest language).
2. Python (rich scraping and AI ecosystem).
3. Go.

**Decision:** Go.

**Trade-offs:** Slower for me to write at first, and a smaller ecosystem for some tasks. In return: a **single self-contained binary** per OS (users download one file; no runtime to install), strong standard library for HTTP and concurrency, fast start-up in CI, and it closes a real gap in my experience.

**Revisit if:** the learning cost blocks progress badly. (Unlikely to change now.)

**In one sentence:** "I picked Go deliberately: it gives users a single download with no runtime, and it was a skill I wanted to build properly, on something real."

---
