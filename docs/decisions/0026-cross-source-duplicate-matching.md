## ADR-0026: Matching the same role across sources — title first, then body; prefer the ATS board

**Status:** Accepted (matching approach decided; the body-similarity mechanism is tuned when built in phase 3)

**Context:** The same role often reaches the tool by more than one path — the company's own ATS board (Greenhouse/Lever/Ashby) and, later, a job-alert email (LinkedIn, Etsy…). They arrive with different stable IDs, so exact-ID dedupe (ADR, phase 2) treats them as separate postings. Without cross-source matching the user would get two alerts for one job and, with the LLM scorer on, pay to score it twice. The naive key "company + title + location" is tempting but wrong in both directions: it over-merges (two genuinely different roles can share a title) and under-merges (the same role can list its location differently on two sites).

**Options considered:**
1. Exact-ID only, no cross-source matching: simple, but duplicate alerts and double scoring.
2. Merge on a fuzzy key (company + normalised title + location): simple, but over-merges same-title-different-role cases and is brittle on location wording.
3. A small decision tree based on how reposted ads actually behave: when a job is syndicated across sites its **title and wording are usually identical**, so use the title as a cheap high-signal gate, and confirm with the body:
   - **Different title → different job.** Keep both. (So only lightly normalise titles — case/whitespace — never strip qualifiers like "(React)", which can distinguish real ads.)
   - **Same title → necessary but not sufficient.** Disambiguate with the body and the structured fields: if work model or location are known and differ, they're different roles (a real example: two ads with the same title, one remote and one on-site); otherwise compare the body text — near-identical means the same ad.
   - **When still uncertain, do not merge.**

**Decision:** Option 3. Match each job once, link the duplicates, and alert once. Where copies disagree, prefer the **ATS board as the canonical, richer copy** (full description, structured fields, stable apply URL) over an alert-email repost, which is only a short snippet (flagged `limited_info`). The email copy is the fallback — and the only source for `ats: none` companies.

**Trade-offs:** Body comparison is weak when one copy is just an email snippet, so ATS-vs-email matching leans on title + company + location agreement rather than body text. Declining to merge when unsure means the user will occasionally see a duplicate alert. Both are deliberate: a wrongly merged role is hidden from the user (the worst outcome, the same asymmetry as ADR-0012), while a duplicate alert is merely mild noise.

**Revisit if:** we add many alert-email senders and duplicates become common, or body-similarity matching proves unreliable in practice — then tune the comparison method/threshold and record it.

**In one sentence:** "The same job shows up in several places with identical wording, so I gate on the title, confirm with the body, prefer the company's own board as the source of truth, and when in doubt I'd rather show a duplicate than hide a role."
