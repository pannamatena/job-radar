## ADR-0018: Catch personal data and secrets before they're pushed, not after

**Status:** Accepted

**Context:** On a public repo, a CI check runs after the push, when the data is already public. The build plan itself contained personal details, and could easily have been committed.

**Decision:**
- `.gitignore` covers private paths from the very first commit.
- A pre-commit hook blocks secrets and private files on the developer's machine.
- GitHub secret scanning and push protection are on.
- The CI check stays as a second line of defence.
- `SECURITY.md` explains what to do if something leaks: rotate the secret, scrub the history, and assume it was copied.

**Trade-offs:** A little friction for contributors (a hook to install).

**In one sentence:** "I moved the safety check to before the mistake can happen, because on a public repo 'after' is too late."

---
