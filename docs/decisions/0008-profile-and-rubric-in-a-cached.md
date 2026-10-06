## ADR-0008: Profile and rubric in a cached system prompt, not fetched with tools

**Status:** Accepted (supersedes the first draft, where the model fetched them with `get_profile` and `get_rubric` tools)

**Context:** In an agent loop, every turn re-sends the whole conversation. The user's profile and rubric are identical for every posting.

**Options considered:**
1. Tools the model calls to fetch the profile and rubric (more "agentic", more turns).
2. Put them in the system prompt, marked for prompt caching (cached input costs about 10% of normal), and keep tools for things that vary per posting.

**Decision:** Option 2. Tools: `get_posting`, `lookup_company_notes`, `lookup_office`, `record_score`.

**Trade-offs:** Slightly less "pure" agent design. In return: fewer turns, fewer re-sent tokens, and cache hits on every posting after the first.

**In one sentence:** "I used tools for what actually changes per item and caching for what doesn't. That's what made the agent affordable."

---
