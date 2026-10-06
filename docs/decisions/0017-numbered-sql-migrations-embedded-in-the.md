## ADR-0017: Numbered SQL migrations, embedded in the binary, written by hand

**Status:** Accepted

**Context:** The database structure will change as features are added (for example, feedback in phase 4 and email sources in phase 5). Every user already has a database full of history, and upgrading to a new version mustn't lose it.

**Options considered:**
1. Delete and recreate the database on upgrade: loses history and feedback, and re-sends old alerts.
2. A migration library (e.g. goose, golang-migrate): adds features such as rolling back and a CLI, but it's another dependency.
3. Hand-written migrations: numbered SQL files (`0001_create_postings.sql`, `0002_add_scores.sql`…), applied in order inside a transaction, with the database recording which ones have run. The files are **embedded in the binary** with Go's `embed`, and the database is **backed up** before migrating.

**Decision:** Option 3.

**Trade-offs:** No built-in "undo" for a migration (the backup covers mistakes), and I maintain about 50 lines of code. In return: no extra dependency, users download a single file with nothing to lose or get out of sync, and I understand it completely. Switching to goose later is easy because it uses the same numbered-file approach.

**Revisit if:** migrations become frequent or complex enough to need rollbacks.

**In one sentence:** "Users upgrade by downloading one file, and their history comes with them. The migrations travel inside the binary."

---
