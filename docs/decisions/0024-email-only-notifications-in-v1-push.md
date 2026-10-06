## ADR-0024: Email-only notifications in V1; push notifications deferred to V2

**Status:** Accepted (supersedes ADR-0013)

**Context:** ADR-0013 picked Telegram for instant alerts with 👍/👎 buttons. Then:
- In 2026, security researchers reported serious Telegram issues: a disputed zero-click flaw affecting Android and Linux desktop, and an unencrypted device identifier enabling tracking. Bot chats are never end-to-end encrypted.
- Every alternative push channel had a catch: WhatsApp costs money per message; ntfy's feedback buttons don't work on iPhone; Slack, Discord and Matrix add setup for non-technical users.

Meanwhile, alerts for job postings aren't urgent to the minute: sources are checked twice a day (ADR-0023).

**Options considered:**
1. Keep Telegram as the default, with a warning.
2. Switch the default to another push channel (ntfy, Slack…).
3. Email only for V1: a match alert after each run, a daily digest, and feedback by replying to the email. Push channels added in V2 behind the existing `Notifier` interface.

**Decision:** Option 3.
- Emails are sent **from** the user's separate job-radar mail account **to** their chosen address (normally their main inbox).
- Replies go back to the job-radar account and are read on the next run, so feedback needs no server.

**Trade-offs:**
- No dedicated push app. Phone mail notifications act as push for most people, but they can be noisier or get filtered to spam (the docs cover adding the sender to contacts).
- Feedback by reply is a few more taps than a button.
- In return: works on every device, no extra app or account, no third-party messaging risk, and one less integration to build, secure and document in V1.

**Revisit in V2:** add push channels as user choices: Slack/Discord/Matrix (feedback via reactions, no server), ntfy (alerts only), and Telegram only with a clear security warning.

**In one sentence:** "When every push option came with a security, cost or platform catch, I shipped V1 on email, which works everywhere, and kept the interface ready for push in V2."

---
