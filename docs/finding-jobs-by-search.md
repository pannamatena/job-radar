# Finding jobs by search

Your watch-list covers the companies you already know. **Search** also looks for
jobs on the web, so you hear about companies you didn't think of. It is off by
default because every search costs money (about a cent each, plus a few tokens).

## How it works

1. job-radar asks a model (Haiku by default) to run your **queries** with
   Anthropic's web search tool. The model sees only the query, never your profile.
2. Each link that comes back is checked **strictly**. It counts only if it is an
   `https` link, with no username or port, whose host is *exactly* one of
   `jobs.ashbyhq.com`, `boards.greenhouse.io`, `job-boards.greenhouse.io`,
   `job-boards.eu.greenhouse.io` or `jobs.lever.co`, and whose path looks like
   `/company/job-id`. A lookalike such as `jobs.ashbyhq.com.example.net` fails.
3. job-radar **never opens a link from a search.** From a valid link it takes the
   company name and job id and asks that board's official API for the posting.
   Postings are then filtered, scored and reported like any other. A job the board
   no longer lists is dropped: search results are often stale (about half were
   closed in testing), and this is how you know a job is still open.
   Greenhouse and Lever are asked for the one job (a "not found" answer means it
   is closed); when three or more jobs were found at one board, or the board is
   Ashby, the board's list is read once instead.
4. Every other link is a **lead**: it is not read and not scored. Leads appear at
   the end of the digest, written so they can't be clicked by accident (see below).

The reasoning is in [ADR-0037](decisions/0037-find-jobs-by-search-read-them-through-board-apis.md).

## Switching it on

In `config.yaml` (the worked example is in [examples/config.yaml](../examples/config.yaml)):

```yaml
search:
  enabled: true
  tracks: [manager, lead_ic]      # the tracks to search the web for
  queries:
    manager:
      - 'site:jobs.ashbyhq.com "engineering manager" Manchester OR "remote UK"'
      # …one per job-board site works best
    lead_ic:
      - 'site:jobs.ashbyhq.com "tech lead" OR "staff engineer" React Manchester OR "remote UK"'
  queries_per_run: 5              # rotate: every query still runs about once a day
  monthly_spend_cap_usd: 5
  ignore_companies: ["Your Employer"]
```

You need `ANTHROPIC_API_KEY` in `.env` (as for the AI scorer), web search switched
on for your organisation in the [Claude Console](https://platform.claude.com/settings/capabilities)
(it is on unless an administrator turned it off), and a price for the search model
under `llm.pricing.models`. All fields are in
[configuration.md](configuration.md#search-fields).

### Which tracks are searched

Only the tracks in `search.tracks`. Any other track you've defined is still found
if it appears on a watch-list board, and that costs nothing. This keeps the bill
to the roles you most want searched for.

### Writing queries

- **Restrict a query to one job-board site** with `site:`. In testing, three such
  queries found postings at 21 different companies; general queries found none on
  those sites and mostly returned aggregators (which become leads).
- **Put the place in the query.** The search tool can't be set to some countries
  (it rejected a country code in testing), so write the place yourself, for example
  "Manchester OR UK OR EMEA".
- **One search returns about ten links.** More distinct queries mean more jobs found.
  Describe what a job is about as well as its title, for example `"design system" OR
  "UI engineer" React`.

## What it costs

A search is 1 cent plus tokens: about 1.4 cents per query on Haiku. Nine queries a
day is about **$3.80 a month**. `monthly_spend_cap_usd` stops searching for the rest
of the month when it is reached, `job-radar report` shows the spend, and a notice
is emailed (at most once a day). If search brings in many postings, raise
`llm.max_postings_per_run` too, because postings over that limit keep only their free
rules score, which is a weak judge of location.

## Leads and the warning

A job found on a site job-radar doesn't read appears like this:

> **Leads from other sites (not scored)**
> ⚠️ job-radar does not know the sites marked "unknown site" and has not opened them.
> Check that a site is safe before you open it…
> • Engineering Manager — news[.]example[.]com (unknown site)

The address is written with `[.]` so that email programs don't turn it into a link;
type it yourself if you want to look. Only sites in `search.link_domains` (LinkedIn
by default) are shown as normal links. job-radar still never opens them: showing a
link is not the same as reading a page ([ADR-0001](decisions/0001-never-scrape-linkedin-or-sign-in.md)).
The digest also lists **new sites** it saw, so you can sign up for their email job
alerts if you want them covered.

To block a site completely, add it to `search.blocked_domains`.

## Using another provider

The web search tool belongs to one provider's API, so the search sits behind a small
interface, `Searcher` (`internal/search/search.go`): it takes a query and returns
the links found. Everything after that, the strict link check, reading the job
boards, scoring and the digest, works on links and doesn't know which provider
found them. The Anthropic web search is the only one that ships today.

To add a provider (a Go change, about one file):

1. Write a type that implements `Searcher` in its own file, next to
   `internal/search/anthropic.go`. It reads its own credentials (for example an API
   key from the environment) and returns a clear error when they are missing.
2. Add one line naming it to the `providers` map in `internal/search/provider.go`.
3. Test it with a fake server, as `anthropic_test.go` does.
4. Users then set `search.provider: <name>`, `search.model`, and
   `search.price_per_search_usd` in `config.yaml`, and add the model's token prices under
   `llm.pricing.models` (that table is just model id → price, for any provider).

`TestAnotherProviderPlugsInWithOneRegistration` shows this works without touching
anything else.

**One provider at a time, no fallback (yet).** `search.provider` names a single
provider, and every search in a run uses it. job-radar never searches with several
providers at once, and it doesn't switch if one fails, for example when its credit
runs out. The failed searches are reported as problems, the watch-list boards and
scoring carry on, and search finds nothing until you top up or change the config.

When a second provider exists, a fallback is a small addition, to be built then:

- The config takes an ordered list, and a run uses the first provider that works.
  Searching with all of them would only double the cost for mostly the same links.
- It switches only on "out of credit" or "key rejected" errors, not on one failed query.
- Spend and the monthly cap are recorded per provider, because each has its own price.

## Commands

```sh
job-radar run                # searches too, when search.enabled is true
job-radar run --no-search    # skip search for this run (it costs money)
job-radar run --dry-run      # searches (and records the spend), but sends no email and saves no leads
job-radar report             # shows this month's search spend
```

## Safety, in short

- The strict link check is code, with tests for lookalike hosts, usernames, ports,
  look-alike letters and path tricks.
- A search result is never fetched, so a hostile link can only fail the check.
- Job ads are untrusted text everywhere ([ADR-0020](decisions/0020-untrusted-job-ads-can-t-make.md));
  the scorer can't browse.
- Leads from unknown sites can't be clicked from your email.
