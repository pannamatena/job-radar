## ADR-0035: What each email says, and when it is sent

**Status:** Accepted

**Context:** Phase 4 sends three kinds of email. If they are noisy, get duplicated, or fail silently, the tool is worse than no alerts. I also run job-radar by hand now and on a schedule later, so the rules can't assume it runs at fixed times.

**Decision:**
- **Match alert:** one email per run listing strong matches (score at or above `thresholds.alert`) that have never been emailed before. Each posting is alerted once per channel, forever; the `notifications` table is the record. If sending fails, nothing is recorded and the next run tries again.
- **Daily digest:** sent by the first run after `email.digest_time` (default 08:00, local time), once per day. It lists postings first seen since the last digest (at most the last 72 hours, 24 hours if none was ever sent) whose latest score is at or above `thresholds.digest`, grouped by track using the track's `label` or its tidied-up name. On Mondays it adds "still open from earlier weeks", so long-listed roles aren't missed. If there is nothing new it sends nothing. It ends with a "Just below the line" section: up to `email.near_misses` (default 5) new jobs that scored one point under the digest line, each with its reason, so a wrong low score can be corrected with a 👍 reply. That section never causes a digest on its own. `job-radar digest` sends it on demand.
- **Problem notice:** when a run fails, a job board has failed three runs in a row, or the AI monthly cap is reached. At most one a day, however many problems there are. A failure to send a problem email is only logged.
- Every email has numbered jobs, a plain-text and an HTML version, and carries a `Message-ID` that is stored with the numbering, so a reply can be matched back ([ADR-0034](0034-read-email-replies-over-imap-read-only.md)).
- Anything that came from a job ad or the scorer is escaped in the HTML, and only `http(s)` links are made clickable.
- An email failure never fails a run, and `--dry-run` prints instead of sending and records nothing.

**Trade-offs:** The digest follows runs rather than the clock. Run by hand, that means it goes out at the first run after the digest time; once the scheduled cloud run exists (phase 6, [ADR-0015](0015-two-repos-per-user-public-code.md)), the computer no longer needs to be on. A first run on a big watch-list can send one long alert email, because every strong match is new to the notifications table. Alerts can include an old posting the first time it reaches the alert threshold. Staying silent on an empty digest means a quiet day looks the same as a broken tool, which is what the problem notices are for.

**Why:** one email per run and one per day keeps the volume low enough to read, and recording every send is what makes "never twice" and "reply matches the right email" simple.

**In one sentence:** "You hear about a good job once, a summary once a day, and a problem at most once a day."
