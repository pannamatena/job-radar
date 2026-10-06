## ADR-0004: A hand-written tool-calling loop, no agent framework

**Status:** Accepted

**Context:** The AI scorer is an agent: the model calls tools (read the posting, look up the office, record the score) until it's done. Frameworks exist that hide this loop.

**Options considered:**
1. An agent framework.
2. A single prompt with no tools.
3. A hand-written loop against the Messages API: send → handle `tool_use` → return `tool_result` → repeat, with a turn limit.

**Decision:** Option 3.

**Trade-offs:** More code to write and test (turn limits, invalid tool input, retries). In return I understand and control every step, can test it with a fake model client, and there's no framework dependency to keep up with.

**Revisit if:** the agent grows many more tools or multi-step planning.

**In one sentence:** "I wrote the agent loop myself so I could test every branch, cap cost per item, and explain exactly what the model can and can't do."

---
