# job-radar

**Watch company job boards, score each new posting against *your own* career
profile and rules, and get told the same day when something fits.**

> ⚠️ **[Read security-and-privacy.md before you set up.](docs/security-and-privacy.md)**
> It's short and explains exactly what data goes where.

job-radar is a free, open-source command-line tool written in Go. It checks the
job boards you choose, drops the obvious non-matches, scores the rest against a
profile and rubric you write, and (soon) emails you the strong fits with an
honest "why it fits" and "what the gap is". Everything personal stays in your
own files — nothing about you is hard-coded, and the public repo ships only a
fictional example persona.

It's built to be **free to run**: the scoring, the job-board access, and (later)
notifications and hosting all have a free option. An optional AI scorer is the
only part that can cost money, and only if you choose to switch it on with your
own API key.

---

## What it looks like

> 🖼️ TODO(screenshot): a match-alert email and a digest email, from the
> fictional persona. Added in phase 4 when email lands.

For now, `job-radar list` prints current postings for your watch-list:

```
COMPANY  TITLE                               LOCATION    REMOTE   URL
Acme     Senior Frontend Engineer            Dublin, IE  unknown  https://...
Acme     Staff Software Engineer, Inference  Dublin, IE  unknown  https://...

2 postings.
```

---

## What it does and doesn't do

**It does:**
- Read jobs from public job-board APIs (Greenhouse now; Lever and Ashby next).
- Score roles against *your* tracks, keywords and location rules.
- Stay a polite guest of every site: identifies itself, rate-limits, backs off
  and caps its own requests ([ADR-0022](docs/decisions/0022-hard-limits-on-outbound-requests-enforced.md)).

**It does not:**
- Scrape LinkedIn or any site without a public feed, or log in anywhere as you
  ([ADR-0001](docs/decisions/0001-never-scrape-linkedin-or-sign-in.md)). LinkedIn
  and other sites without an API are covered by *your own* job-alert emails.
- Auto-apply or write applications for you.
- Send your data anywhere except the services you switch on. No analytics, ever.

---

## Cost, honestly

- **Completely free** with the default rules scorer: no API key, no account, no
  spend. Fetching, filtering, scoring, (soon) email and automation all work for
  free.
- The **optional AI scorer** reads each ad like a person would and writes a more
  useful why/gap. It uses your own API key and costs money per posting. The
  measured cost per posting and per month — and what the AI actually adds over
  the free scorer — will be published here with real numbers from the
  evaluation (phase 3b). No guessing.

---

## Quick start

See **[docs/getting-started.md](docs/getting-started.md)** — from nothing to a
list of real postings in about 15 minutes, on your own computer, for free.

```sh
make build
./bin/job-radar init     # creates your private config from an example
./bin/job-radar list     # prints current postings for your companies
```

---

## Full guides

- [getting-started.md](docs/getting-started.md) — install, `init`, first run.
- [configuration.md](docs/configuration.md) — every config field *(phase 2)*.
- [adding-companies.md](docs/adding-companies.md) — supported boards, `discover` *(phase 2)*.
- [rules-scorer.md](docs/rules-scorer.md) — how free scoring works *(phase 2b)*.
- [writing-your-profile.md](docs/writing-your-profile.md) — profile & rubric *(phase 2b)*.
- [ai-scorer.md](docs/ai-scorer.md) — the optional AI scorer *(phase 3)*.
- [evaluation.md](docs/evaluation.md) — measuring quality *(phase 3b)*.
- [email-setup.md](docs/email-setup.md) — alerts & digest *(phase 4)*.
- [job-alert-emails.md](docs/job-alert-emails.md) — LinkedIn/Google/Etsy *(phase 5)*.
- [run-automatically.md](docs/run-automatically.md) — GitHub Actions *(phase 6)*.
- [troubleshooting.md](docs/troubleshooting.md) · [faq.md](docs/faq.md) · [glossary.md](docs/glossary.md)

---

## Security and privacy

job-radar runs on *your* computer or *your* GitHub account. There's no
job-radar server or database; nobody behind the project can see your profile,
your alerts or your job search. Your data only goes to services you switch on.

👉 **Read the full notice: [docs/security-and-privacy.md](docs/security-and-privacy.md)**
before you set up. To report a security issue, see [SECURITY.md](SECURITY.md).

---

## Project status

Built in phases. **Phase 1 is complete:** project skeleton, the shared
rate-limited HTTP client, config loading, the Greenhouse source, and the `init`
and `list` commands.

| Phase | What | Status |
|---|---|---|
| 1 | Skeleton + Greenhouse source + `init`/`list` | ✅ done |
| 2 | Lever & Ashby sources, store, dedupe, `discover` | planned |
| 2b | Free rules scorer | planned |
| 3 | LLM scoring agent (optional) | planned |
| 3b | Evaluation + published cost numbers | planned |
| 4 | Email alerts & digest, reply-to-vote feedback | planned |
| 5 | Job-alert emails (LinkedIn, Google, Etsy) | planned |
| 6 | Open-source release: binaries, hosting template, full docs | planned |

Why it's built this way: see the **[Architecture Decision Records](docs/decisions/)**.

Roadmap (V2): push notifications, a graphical setup page, more job boards, a
free local-model scorer. See [ADR-0024](docs/decisions/0024-email-only-notifications-in-v1-push.md)
and the decision records.

---

## Contributing

Contributions are welcome — new job boards, alert-email parsers, notifiers and
LLM providers are all designed to be small, documented additions. See
[CONTRIBUTING.md](CONTRIBUTING.md) *(fuller version in phase 6)* and the
[code of conduct](CODE_OF_CONDUCT.md).

## Licence

[MIT](LICENSE). Free software, provided "as is", without warranty — see the
LICENSE and the security-and-privacy notice.
