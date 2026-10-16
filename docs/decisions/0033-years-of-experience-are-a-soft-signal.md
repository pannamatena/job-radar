## ADR-0033: Years of experience are a soft signal, not a blocker

**Status:** Accepted

**Context:** My rubric listed "required years I lack" as something that lowers a score, and the built-in instructions repeated it. I tested what that did: I took two real ads that scored 4 and changed only the number of years to 7+ and 8+ (I have about 5). Both fell to a 2 in every repeat. But an ad's number of years is usually a guess, people apply anyway, and a shortfall of a year or two in work you actually do is the softest requirement in an ad.

**Options considered:**
1. Keep "required years" as a blocker: simple and predictable, but it hides roles the user would apply for.
2. Ignore years entirely: simple, but then "8 years as a TPM" for someone who has never been one looks like a fit.
3. Make years a soft signal that depends on the kind of work and the seniority.

**Decision:** Option 3. Where the user does that kind of work, a years shortfall never sinks a role on its own. The more senior the role, the less it matters (7 or 8 asked, 5 held: a small point). For a more junior ask with a big gap (5 asked, 2 held) the score drops by at most a point when the skills fit. It is a blocker only when the user has no experience at all of that kind of work and the other core requirements don't fit either. The model still names the shortfall in `gap`. This is in the built-in instructions and in the example rubric.

**Trade-offs:** More roles reach the user with a years gap that a recruiter may care about; the gap line says so. The rule is judgement, not a number, so the model's call can vary a little between runs.

**Evidence:** the two changed ads went from 2,2,2 to 4,4,4 (and one repeat ran out of its token budget after three invalid score calls). The TPM ad that asks for 8 years in a role I have never held stayed at 2. The full set of 14 showed no 1–2 role rising and no case dropping. These cases were used for tuning, so this is not a measure of unseen ads.

**In one sentence:** "A years gap in work you really do is a note, not a no."
