## ADR-0020: Untrusted job ads can't make the AI do anything harmful

**Status:** Accepted

**Context:** A job ad could contain hidden instructions aimed at the AI ("prompt injection"), for example asking it to reveal the profile or to give a perfect score.

**Decision:**
- Ad text is labelled as untrusted data in the system prompt.
- The AI's tools are read-only except `record_score`, and no tool can send data anywhere.
- `lookup_office` only fetches pages on company domains listed in config; the model never supplies a URL.
- Model output is never executed.
- An eval case with an injected instruction checks that the score and output aren't affected.

**Trade-offs:** The office lookup is less flexible (allowlisted domains only).

**In one sentence:** "I assumed the input could be hostile and limited what the model could do, so the worst case is a wrong score, not a data leak."

---
