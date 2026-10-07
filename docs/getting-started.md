# Getting started

This guide takes you from nothing to a list of real job postings from the
companies you choose, running on your own computer, for free. No AI, no email
setup, no account sign-ups — that comes later.

> **Please read [security-and-privacy.md](security-and-privacy.md) first.** It's
> short and explains what job-radar does with your data. `job-radar init` will
> ask you to confirm you've read it.

New to a term? See the [glossary](glossary.md).

---

## 1. Install

You have two options.

### Option A — download a prebuilt binary (no Go needed)

> 🚧 Prebuilt downloads arrive with the first release (phase 6). Until then,
> use Option B.

### Option B — build from source (needs Go)

1. Install **Go** (1.27 or newer) from <https://go.dev/dl/>, and **git**.
2. Clone this repository and build:
   ```sh
   git clone https://github.com/pannamatena/job-radar.git
   cd job-radar
   make build        # produces ./bin/job-radar
   ```
3. Check it runs:
   ```sh
   ./bin/job-radar version
   ```
   You should see something like `job-radar 0.1.0-dev`.

---

## 2. Create your config

Run:

```sh
./bin/job-radar init
```

You'll see a short security-and-privacy summary and a link to the full notice.
Type `y` to confirm you've read it. job-radar then creates a `private/` folder
containing:

```
private/
  config.yaml   your companies and location rules
  profile.md    your career profile (starts as an example persona)
  rubric.md     how roles should be scored for you
  evals/        example test cases (for a later phase)
```

> **`private/` is never committed to git.** That's where all your personal data
> lives. job-radar's `.gitignore` and a pre-commit hook keep it out of the repo.

**You should now see:** `✓ Created your config in private`.

---

## 3. Add companies you want to watch

Open `private/config.yaml` in any text editor. Replace the example companies
with ones you're interested in. Each needs an `ats` (which job board it uses)
and a `slug` (its identifier on that board):

```yaml
companies:
  - name: Acme
    ats: greenhouse        # greenhouse | lever | ashby | none
    slug: acme             # the company's board token
    filters:
      departments: ["Engineering"]
```

Not sure which board a company uses or what its slug is? Let `discover` work it
out from a careers URL:

```sh
./bin/job-radar discover --name "Acme" https://acme.com/careers
```

It prints a ready-to-paste config entry. See
[adding-companies.md](adding-companies.md) for details.

> **Greenhouse, Lever and Ashby** are all supported. Companies with no public
> API (`ats: none`) are covered by job-alert emails in phase 5; `list` tells you
> which it's skipping and why.

---

## 4. See your postings

```sh
./bin/job-radar list
```

You'll get a table of current roles, with roles new since your last run marked
`NEW`:

```
     COMPANY  TITLE                               LOCATION        REMOTE
NEW  Acme     Senior Frontend Engineer            Manchester, UK  hybrid
NEW  Acme     Staff Software Engineer, Platform    Manchester, UK  unknown

Fetched 2 kept, 0 dropped by pre-filter · 2 new, 0 already seen.
```

Run it again and those same roles appear without the `NEW` marker
(`0 new, 2 already seen`) — job-radar remembers what it has shown you.

To just list your watch-list (including companies with no public API):

```sh
./bin/job-radar list --companies
```

**It didn't work?**
- *"no config found"* — run `job-radar init` first, or set `JOB_RADAR_HOME` to
  your config folder.
- *A company shows `unexpected HTTP 404`* — the slug is probably wrong. Check
  the company's careers-page address.
- *No postings* — your `filters` may be too narrow; try removing them.

---

## What's next

- **Scoring** (which roles actually fit you) arrives with the free rules scorer
  in phase 2b — see [rules-scorer.md](rules-scorer.md).
- **Email alerts** arrive in phase 4 — see [email-setup.md](email-setup.md).
- **Running automatically** (no computer needed) arrives in phase 6 — see
  [run-automatically.md](run-automatically.md).
