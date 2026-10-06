## ADR-0014: Open source, with all personal data in user config

**Status:** Accepted

**Context:** The tool started as my personal job radar but should work for anyone.

**Decision:** Nothing personal is hard-coded: profile, tracks, rubric, keywords, location rules, watch-list companies, alert senders and eval labels all live in each user's private config folder. The public repo only ships a **fictional** example persona.

**Trade-offs:** More configuration and validation work, and more docs. In return, anyone can use it, and the code is easier to test because behaviour is driven by inputs.

**In one sentence:** "I separated the engine from the person, so the same code serves anyone and none of my data is in it."

---
