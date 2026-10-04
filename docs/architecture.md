# Architecture

> 🚧 Written in full in **phase 3**, when the scoring agent loop exists to
> explain. It will include the pipeline diagram and a walkthrough of the
> hand-written tool-calling loop, linking to the relevant
> [decision records](decisions/).

## The pipeline (overview)

```
sources (Greenhouse, Lever, Ashby, IMAP*) → normalise → dedupe (SQLite)
  → pre-filter (title + location rules) → rules scorer (free)
  → [optional] LLM scoring agent → store → notify (email) → read 👍/👎 replies
```
`*` IMAP job-alert emails arrive in phase 5.

For now, the code layout is described in [CONTRIBUTING.md](../CONTRIBUTING.md),
and the reasoning behind the design is in the
[decision records](decisions/).
