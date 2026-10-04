## ADR-0013: Telegram as the default instant channel, email as the fallback

**Status:** Superseded by ADR-0024 (kept because the comparison of channels is still useful for V2)

**Context:** Strong matches need to reach the user's phone quickly. It must be free, work on iPhone, Android and desktop, and support 👍/👎 feedback buttons.

**Options considered:**
1. **WhatsApp** (official Cloud API): most familiar, but charges per message, needs a Meta business setup and pre-approved templates, and button replies need a webhook server. Unofficial libraries risk the user's personal number being banned.
2. **ntfy:** free and simple, but its tap-to-send buttons don't work on iPhone, and the topic name works as a password.
3. **Slack / Discord:** free to send to, but buttons need more setup.
4. **Email only:** free, but not really instant, and no buttons.
5. **Telegram bot:** free, on every platform, inline buttons work everywhere, and button taps can be collected by **polling at the start of each run**, so there's no server to host.

**Decision:** Telegram by default, email for the daily digest and as the fallback if an instant alert fails, ntfy as an optional extra. All behind a `Notifier` interface so others can add channels.

**Trade-offs:** Users have to install Telegram even if their contacts aren't on it. Polling means 👍/👎 is recorded at the next run, not instantly. Bot chats aren't end-to-end encrypted, so alerts are kept short and job-focused.

**In one sentence:** "I chose the channel that was free, cross-platform and needed no server for feedback, and I kept it swappable."

---
