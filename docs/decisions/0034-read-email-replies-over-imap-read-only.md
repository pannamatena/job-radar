## ADR-0034: Read vote replies over IMAP, read-only, from your own address only

**Status:** Accepted

**Context:** Email is the only notification channel in V1 ([ADR-0024](0024-email-only-notifications-in-v1-push.md)), and I wanted a way to tell the tool "that one was good, that one wasn't" without running a server or opening a web page. Replying to the email is the simplest thing a person can do. That means job-radar must read mail from the job-radar account. Reply text is written by a person but arrives through a channel anyone can send into, so it is untrusted.

**Options considered:**
1. No feedback by email; only a terminal command: nothing to secure, but nobody votes from their phone.
2. A link in each email that opens a small web page: needs a server and a public address, which the project avoids ([ADR-0016](0016-sqlite-for-state-kept-on-a.md)).
3. Read replies over IMAP using the `go-imap` v2 library, copied into the repo.
4. Read replies over IMAP with a small client I write myself from Go's standard library: no third-party code at all, but the protocol code, and its bugs, would be mine.

**Decision:** Option 3. The reader opens the inbox read-only, searches only the last 14 days for messages from the addresses in `email.to`, and acts only on a message that answers one of job-radar's own emails (matched by the `In-Reply-To` or `References` header against the Message-ID job-radar sent). From the body it takes only the part the person wrote (quoted text is dropped) and only recognised votes: a number followed by 👍/👎, `up`/`down`, `+1`/`-1`, `yes`/`no`. Anything else is ignored and never acted on. A vote is recorded once per reply and posting. Votes do not change any score by themselves.

The libraries (`go-imap`, `go-message`, `go-sasl`, all by one maintainer, about 17,000 lines of which job-radar uses a few hundred) are copied unchanged into `third_party/` and used through `replace` lines in `go.mod`. An upgrade is therefore a visible diff in a pull request. Upgrades are by hand, wait at least a week after an upstream release, and Dependabot is set to wait the same before proposing any Go update. Sending email uses only Go's standard library.

## What a compromised library could do

The strict parser and the read-only inbox are job-radar's own code. A malicious library runs inside job-radar with its full access and would ignore them. It could:
- **Steal the app password**, which passes through the library at login. With it an attacker can read the job-radar mailbox and send mail from that address. The mailbox holds only job-radar mail, and an app password cannot change the account's settings. It can be revoked in one click.
- **Phish you from a trusted sender.** Mail from that address lands in your main inbox, in a contact you have marked as safe.
- **Reach everything else the program can**: the Anthropic API key (and spend money with it, limited only by the limit set in the Anthropic Console), `profile.md`, the database, the config, any file your user account can read, and any network connection. The outbound-request limits in [ADR-0022](0022-hard-limits-on-outbound-requests-enforced.md) only govern job-radar's own HTTP client.
- **In the cloud (later):** read the Actions secrets and repo token available to the run.
- **Record wrong votes or hide replies.**

This is not new in kind: the Anthropic SDK, the SQLite driver and the YAML parser already run in the same process. What is new is that the email password also passes through third-party code.

**How it could happen:** a malicious release by the maintainer or someone who takes over the account, or a flaw in the version chosen now (I have not read all of its code). A pinned version with a checksum cannot change by itself, so the risk sits at the moments of choosing and of upgrading, which is what copying the code in, the manual upgrade and the one-week wait address. They reduce the chance of an unnoticed bad release; they do not remove the risk. Go has no built-in "minimum release age" setting like yarn's; the wait is enforced by the Dependabot `cooldown` setting and by the upgrade steps in `third_party/README.md`.

**Trade-offs:** `go-imap` v2 is still in beta, so its API may change; the use here is small (login, read-only select, search, fetch), which keeps an upgrade cheap. The "from" check trusts the sender header, which can be forged: the worst a forger can do is record a wrong vote, and Gmail's own spam and spoofing checks apply first. The mailbox login is the same app password used to send, so one leak exposes both (see the notice in [security-and-privacy.md](../security-and-privacy.md)); the separate job-radar account limits what that exposes, and that address must not be used for anything else. **If this risk is judged too high, option 4 is the way out:** it needs only four IMAP commands and would remove all three libraries.

**Why:** a vote is a number and a thumb, so a strict parser that ignores everything else gives a reply nothing to exploit. Doing less with untrusted text is the safest design ([ADR-0020](0020-untrusted-job-ads-can-t-make.md) applies the same idea to job ads).

**In one sentence:** "Replying '1 👍' works, and nothing else in a reply can make the tool do anything; the third-party code that reads your mail is copied in and upgraded only by hand."
