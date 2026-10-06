# Contributing to job-radar

Thanks for your interest! This is an open-source tool designed so that common
additions — a new job board, an alert-email parser, a notifier, or an LLM
provider — are small, well-contained changes.

> This is a first version. A fuller guide (with a worked example for each kind
> of extension) lands with the phase-6 release.

## Development setup

You'll need **Go 1.27+** and **git**.

```sh
git clone https://github.com/pannamatena/job-radar.git
cd job-radar
make hooks     # install the pre-commit hook (blocks secrets & private data)
make test      # run the test suite
make build     # build ./bin/job-radar
```

Run `make help` to see all tasks. Run `make check` (format + vet + test) before
you commit.

## Important rules

- **Never commit personal data or secrets.** Everything under `private/`, any
  `.env` file and any `*.db` file is git-ignored on purpose. `make hooks`
  installs a pre-commit hook that blocks them; please keep it installed.
- **All outbound HTTP must go through `internal/httpclient`.** No source, tool
  or notifier may create its own HTTP client — this is how we guarantee we stay
  a polite client of every site ([ADR-0022](docs/decisions/0022-hard-limits-on-outbound-requests-enforced.md)).
- **Record significant decisions as ADRs** in `docs/decisions/`, following
  [DECISIONS.md](DECISIONS.md), in the same commit as the change.
- **No scraping, ever**, and no logging in anywhere as the user
  ([ADR-0001](docs/decisions/0001-never-scrape-linkedin-or-sign-in.md)).
- Keep the public repo free of personal data: examples use the fictional
  persona only.

## Code layout

See [docs/architecture.md](docs/architecture.md) *(added in phase 3)* for the
pipeline. In short: `cmd/job-radar` is the CLI; `internal/` holds the pieces
(config, sources, posting, httpclient, logging, and later store, scorer, agent,
notify, eval).

## Pull requests

Keep PRs focused, include tests, and make sure `make check` passes. By
contributing you agree your work is licensed under the project's
[MIT licence](LICENSE).
