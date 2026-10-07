# Adding companies

job-radar watches the companies you list in `config.yaml`. This guide shows how
to add one, how to find its job board automatically, and what to do for
companies that don't have a public one.

New to a term here? See the [glossary](glossary.md). For every config field, see
the [configuration reference](configuration.md).

---

## Supported job boards

job-radar reads jobs directly from three public job-board APIs:

| Board (ATS) | `ats:` value | Where the slug comes from |
|---|---|---|
| Greenhouse | `greenhouse` | `boards.greenhouse.io/`**`acme`** |
| Lever | `lever` | `jobs.lever.co/`**`acme`** |
| Ashby | `ashby` | `jobs.ashbyhq.com/`**`acme`** |

The **slug** (or "board token") is the company's short identifier in its board's
web address — the bold part above.

> Adding support for another board (Workable, Teamtailor, Personio…) is a small,
> contained change — see [CONTRIBUTING.md](../CONTRIBUTING.md).

---

## The easy way: `discover`

Give `discover` the company's careers-page URL (or its board URL) and it works
out the ATS and slug, checks it, and prints a ready-to-paste config entry:

```sh
job-radar discover --name "Acme" https://acme.com/careers
```

If it finds a board you'll see:

```
✓ Detected the greenhouse board "acme" with 42 current postings.

Add this to the `companies:` list in your config.yaml:

  - name: Acme
    ats: greenhouse
    slug: acme
```

Copy those lines into the `companies:` list in your `config.yaml`.

**If `discover` can't detect a board:** some careers pages build their job list
with JavaScript, so the board link isn't in the page's HTML. Open the careers
page in your browser, click through to the actual job list, and run `discover`
on *that* URL (the one containing `greenhouse.io`, `lever.co` or `ashbyhq.com`).

---

## Adding one by hand

A company entry looks like this:

```yaml
companies:
  - name: Acme                 # a label for you
    ats: greenhouse            # greenhouse | lever | ashby | none
    slug: acme                 # the board token
    filters:                   # optional — narrow which roles are kept
      departments: ["Engineering"]
```

### Narrowing with filters

Filters are optional and keep the list relevant:

```yaml
    filters:
      departments: ["Engineering", "Product"]   # keep these departments
      location_names: ["Manchester"]                 # keep roles mentioning Manchester
      include_remote_open_to: ["United Kingdom"]        # ...and remote roles open to the UK
      office_ids: [4006509008]                   # Greenhouse only: keep this office
```

See the [configuration reference](configuration.md#company-filters) for exactly
how each one behaves. If you're unsure, start with no filters, run
`job-radar list`, and add filters once you see what comes back.

---

## Companies without a public board (`ats: none`)

Some companies have no public job-board API — for example Google, or companies
on SmartRecruiters (like Etsy). **job-radar never scrapes careers pages**
([ADR-0001](decisions/0001-never-scrape-linkedin-or-sign-in.md),
[ADR-0002](decisions/0002-companies-without-a-usable-public-feed.md)). Instead,
list them as `ats: none` with their careers link:

```yaml
  - name: Google
    ats: none
    careers_url: "https://www.google.com/about/careers/..."
```

These are covered by **that company's own job-alert emails**, which job-radar
reads in phase 5. Listing them keeps the gap visible: they show up in
`job-radar list --companies` so you can see they're not fetched directly.

---

## Checking it worked

```sh
job-radar list --companies   # shows your whole watch-list and each board
job-radar list               # fetches and prints current postings
```

On the first run, every matching posting is marked `NEW`. Run it again and those
same postings show without the marker — job-radar remembers what it has already
seen (`0 new, N already seen`).
