## ADR-0006: A free rules scorer first; the AI scorer is an optional upgrade

**Status:** Accepted

**Context:** The project is open source and must be free to run. LLM APIs cost money, and not everyone will have a key.

**Options considered:**
1. AI-only scoring (every user needs an API key).
2. Rules-only scoring.
3. Both behind one `Scorer` interface: a free keyword-and-weights rules scorer, and an optional LLM scorer.

**Decision:** Option 3. With no API key, the whole pipeline still works. The rules scorer was built first (phase 2b), so there was a working free version before any AI was added.

**Trade-offs:** Two scorers to maintain, and the rules scorer is cruder: it can't tell a rail "Engineering Manager" from a software one without explicit keywords. It does, though, give the AI scorer an honest **baseline** to beat in the evaluation.

**In one sentence:** "The free scorer made the tool usable by anyone, and it doubled as the baseline that proved what the AI actually adds."

---
