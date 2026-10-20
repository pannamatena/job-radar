## ADR-0040: Run in the cloud with a verified program and a `state` branch

**Status:** Accepted. Confirms [ADR-0016](0016-sqlite-for-state-kept-on-a.md) and builds on [ADR-0015](0015-two-repos-per-user-public-code.md) and [ADR-0023](0023-check-sources-twice-a-day-retrying.md).

**Context:** The scheduled workflow in a user's private repo has to (a) get the program, (b) keep the database between runs, (c) not leak their data into logs, and (d) pick up fixes without the user doing anything, while never running something they didn't choose. The database holds their seen postings, scores, votes and spend, so losing it means a flood of old jobs by email.

**Decisions:**
1. **The program is downloaded from a release and verified.** Before running it, the workflow checks the file against the published checksum **and** checks GitHub's signed build attestation, which proves the file was built by this project's release workflow from the tagged commit. A file that fails either check is never run.
2. **Users follow patch releases.** The workflow has `JOB_RADAR_SERIES: '0.1'` and runs the newest `v0.1.x`. Patch releases are bug and security fixes. A new minor or major version (which can change settings or behaviour) only happens when the user edits that line.
3. **The database lives on a `state` branch**, as one force-pushed commit holding only `radar.db`. The previous copy is kept for one run on `state-previous` for rollback. An `actions/cache` copy was rejected because GitHub can evict it and the history would be lost silently.
4. **All saving and restoring is in the workflow.** The Go program only reads and writes `radar.db` in its home folder, so it also runs unchanged on a laptop or a server.
5. **A missing `state` branch stops the run** unless the user asks to start fresh, because an empty database would email every open job as new.
6. **The state is saved even when the run fails**, because a run that crashed after sending emails must record them or the next run sends them again.
7. **The Actions log holds counts, ids and errors only.** The readable report (titles, scores, reasons) goes to `/dev/null`; the structured log on stderr stays.
8. **One run at a time** (a `concurrency` group), because two runs would write the same branch.
9. **Local runs are allowed, but only through pull and push scripts that check first.** `state-push.sh` overwrites the cloud database only if it is still the version that was pulled (`git push --force-with-lease`, a compare-and-swap); otherwise it refuses. The workflow does the same when it saves, and keeps its database on `state-conflict` if you pushed during the run. The private repo also has guardrails against committing secrets: `.gitignore`, a pre-commit hook with `gitleaks`, a GitHub check that fails if a secret file is tracked, and a refusal to run the cloud job or the scripts in a public repository. We can't forbid local use, so the docs warn clearly and tell anyone not comfortable with `git push` and `git pull` to stick to the schedule.

**Trade-offs:**
- The `state` branch grows no history (one commit, replaced each run), so there is no way back further than one run.
- A local run and the cloud run have separate databases, and spend caps are counted per database. They are not merged. The scripts make a collision visible and harmless to the data, but a collision still costs duplicate emails, and a user who never uses the scripts can overspend a cap across the two copies.
- A config folder that is also the git repository means one copy of the config, but users must be comfortable with `git commit` and `git push`.
- Following patch releases means trusting this project's release workflow for fixes. The checksum and attestation checks make a tampered file fail, but not a malicious change merged into `main` and released, which is what branch protection and review are for.
- GitHub may start a scheduled run late. Scheduled workflows in a **private** repo are not switched off for inactivity (that rule applies to public repos).

**Revisit if:** runs often fail halfway (add retries, [ADR-0023](0023-check-sources-twice-a-day-retrying.md)), the database grows beyond what a branch should hold, or users want to share one database between machines.

**In one sentence:** "Download the program only if it provably came from the release workflow, keep one database in one place, and never let the log or a crash lose or leak the user's data."
