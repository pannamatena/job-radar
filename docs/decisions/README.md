# Architecture Decision Records

Each record explains one decision: the problem, the options, the choice and what it costs. See [`DECISIONS.md`](../../DECISIONS.md) for the format and the rules on adding new records.

| # | Decision | Status |
|---|---|---|
| [ADR-0001](0001-never-scrape-linkedin-or-sign-in.md) | Never scrape LinkedIn or sign in as the user | Accepted |
| [ADR-0002](0002-companies-without-a-usable-public-feed.md) | Companies without a usable public feed are covered by job-alert emails | Accepted |
| [ADR-0003](0003-go-for-the-implementation.md) | Go for the implementation | Accepted |
| [ADR-0004](0004-a-hand-written-tool-calling-loop.md) | A hand-written tool-calling loop, no agent framework | Accepted |
| [ADR-0005](0005-structured-output-through-a-validated-record.md) | Structured output through a validated `record_score` tool | Accepted |
| [ADR-0006](0006-a-free-rules-scorer-first-the.md) | A free rules scorer first; the AI scorer is an optional upgrade | Accepted |
| [ADR-0007](0007-rules-first-ai-second-the-gate.md) | Rules first, AI second (the gate) | Accepted |
| [ADR-0008](0008-profile-and-rubric-in-a-cached.md) | Profile and rubric in a cached system prompt, not fetched with tools | Accepted |
| [ADR-0009](0009-cost-controls-built-in-not-bolted.md) | Cost controls built in, not bolted on | Accepted |
| [ADR-0010](0010-model-choice-decided-by-evaluation-escalation.md) | Model choice decided by evaluation; escalation only if the data shows it's needed | Proposed |
| [ADR-0011](0011-evaluate-with-a-small-hand-labelled.md) | Evaluate with a small hand-labelled set, measured at the alert threshold | Accepted |
| [ADR-0012](0012-pre-filter-rule-when-in-doubt.md) | Pre-filter rule: when in doubt, keep the posting | Accepted |
| [ADR-0013](0013-telegram-as-the-default-instant-channel.md) | Telegram as the default instant channel, email as the fallback | Superseded by ADR-0024 |
| [ADR-0014](0014-open-source-with-all-personal-data.md) | Open source, with all personal data in user config | Accepted |
| [ADR-0015](0015-two-repos-per-user-public-code.md) | Two repos per user (public code, private config), run on GitHub Actions | Accepted |
| [ADR-0016](0016-sqlite-for-state-kept-on-a.md) | SQLite for state, kept on a `state` branch | Proposed |
| [ADR-0017](0017-numbered-sql-migrations-embedded-in-the.md) | Numbered SQL migrations, embedded in the binary, written by hand | Accepted |
| [ADR-0018](0018-catch-personal-data-and-secrets-before.md) | Catch personal data and secrets before they're pushed, not after | Accepted |
| [ADR-0019](0019-email-access-through-a-separate-inbox.md) | Email access through a separate inbox, with alerts auto-forwarded | Accepted |
| [ADR-0020](0020-untrusted-job-ads-can-t-make.md) | Untrusted job ads can't make the AI do anything harmful | Accepted |
| [ADR-0021](0021-terminal-and-config-files-in-v1.md) | Terminal and config files in V1; a graphical setup page in V2 | Accepted |
| [ADR-0022](0022-hard-limits-on-outbound-requests-enforced.md) | Hard limits on outbound requests, enforced in one shared HTTP client | Accepted |
| [ADR-0023](0023-check-sources-twice-a-day-retrying.md) | Check sources twice a day, retrying only what failed | Accepted |
| [ADR-0024](0024-email-only-notifications-in-v1-push.md) | Email-only notifications in V1; push notifications deferred to V2 | Accepted |
| [ADR-0025](0025-score-fit-independently-of-posting-age.md) | Score fit independently of posting age; alert on "new to you", not "newly posted" | Accepted |
| [ADR-0026](0026-cross-source-duplicate-matching.md) | Matching the same role across sources — title first, then body; prefer the ATS board | Accepted |

## In one sentence each

- **[ADR-0001](0001-never-scrape-linkedin-or-sign-in.md)** — I designed around the platform's rules instead of fighting them: public job-board APIs for speed, and the user's own alert emails for LinkedIn, so nobody's account is put at risk.
- **[ADR-0002](0002-companies-without-a-usable-public-feed.md)** — If the terms weren't clear, I treated that as a no and found a sanctioned route instead.
- **[ADR-0003](0003-go-for-the-implementation.md)** — I picked Go deliberately: it gives users a single download with no runtime, and it was a skill I wanted to build properly, on something real.
- **[ADR-0004](0004-a-hand-written-tool-calling-loop.md)** — I wrote the agent loop myself so I could test every branch, cap cost per item, and explain exactly what the model can and can't do.
- **[ADR-0005](0005-structured-output-through-a-validated-record.md)** — Making the output a validated tool call turned model answers into data I could test and measure.
- **[ADR-0006](0006-a-free-rules-scorer-first-the.md)** — The free scorer made the tool usable by anyone, and it doubled as the baseline that proved what the AI actually adds.
- **[ADR-0007](0007-rules-first-ai-second-the-gate.md)** — I put a free filter in front of the paid model, and used the eval to check it wasn't throwing away good roles.
- **[ADR-0008](0008-profile-and-rubric-in-a-cached.md)** — I used tools for what actually changes per item and caching for what doesn't. That's what made the agent affordable.
- **[ADR-0009](0009-cost-controls-built-in-not-bolted.md)** — Every way the tool could overspend has a cap, and every saving is measured rather than assumed.
- **[ADR-0010](0010-model-choice-decided-by-evaluation-escalation.md)** — I didn't guess the model; I compared them on my own labelled data and chose on wrong-alert rate per euro.
- **[ADR-0011](0011-evaluate-with-a-small-hand-labelled.md)** — I measured the thing that matters to the user, wrong alerts and missed roles, against my own labels, and I publish the sample size with the numbers.
- **[ADR-0012](0012-pre-filter-rule-when-in-doubt.md)** — I matched the filter's errors to their real cost: missing a role is far worse than scoring one extra.
- **[ADR-0013](0013-telegram-as-the-default-instant-channel.md)** — I chose the channel that was free, cross-platform and needed no server for feedback, and I kept it swappable.
- **[ADR-0014](0014-open-source-with-all-personal-data.md)** — I separated the engine from the person, so the same code serves anyone and none of my data is in it.
- **[ADR-0015](0015-two-repos-per-user-public-code.md)** — Every user runs their own copy in their own private repo, so it's free, there's no server, and I never hold anyone's data.
- **[ADR-0016](0016-sqlite-for-state-kept-on-a.md)** — A single-file database was all this needed, and I chose storage that can't be silently evicted.
- **[ADR-0017](0017-numbered-sql-migrations-embedded-in-the.md)** — Users upgrade by downloading one file, and their history comes with them. The migrations travel inside the binary.
- **[ADR-0018](0018-catch-personal-data-and-secrets-before.md)** — I moved the safety check to before the mistake can happen, because on a public repo 'after' is too late.
- **[ADR-0019](0019-email-access-through-a-separate-inbox.md)** — I limited what a leaked credential could reach, without changing anything about how the user reads their email.
- **[ADR-0020](0020-untrusted-job-ads-can-t-make.md)** — I assumed the input could be hostile and limited what the model could do, so the worst case is a wrong score, not a data leak.
- **[ADR-0021](0021-terminal-and-config-files-in-v1.md)** — I shipped the engine first and designed the config so a friendlier interface could sit on top later without a rewrite.
- **[ADR-0022](0022-hard-limits-on-outbound-requests-enforced.md)** — I put the politeness rules in one place that every request has to go through, so no config mistake or bug can turn the tool into a nuisance.
- **[ADR-0023](0023-check-sources-twice-a-day-retrying.md)** — I matched how often we check to how often the data actually changes, and only retried what failed.
- **[ADR-0024](0024-email-only-notifications-in-v1-push.md)** — When every push option came with a security, cost or platform catch, I shipped V1 on email, which works everywhere, and kept the interface ready for push in V2.
- **[ADR-0025](0025-score-fit-independently-of-posting-age.md)** — A real-world miss showed I'd tied relevance to recency, so I separated 'is it a good fit?' from 'is it new to me?', and turned the miss into a test case.
- **[ADR-0026](0026-cross-source-duplicate-matching.md)** — The same job shows up in several places with identical wording, so I gate on the title, confirm with the body, prefer the company's own board as the source of truth, and when in doubt I'd rather show a duplicate than hide a role.
