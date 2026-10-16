# Email setup: alerts, the daily digest and 👍/👎 replies

job-radar can email you when it finds good roles. It sends from a **separate
job-radar email account** (so the key it holds can't touch your real inbox) to
**your main inbox**, where you already look. Setting this up takes about 15
minutes and costs nothing.

**What you'll get**

| Email | When | What's in it |
|---|---|---|
| **Match alert** | At the end of a run, when there are strong matches you haven't been told about (score at or above `thresholds.alert`, default 4). One email per run, not one per job. | Each job: number, score, company, title, track, work model, location, a one-line **why** and **gap**, and a link. |
| **Daily digest** | With the first run after `email.digest_time` (default 08:00 your time), once a day. | New roles since the last digest scoring `thresholds.digest` or higher (default 3), grouped by your tracks. On Mondays, roles still open from earlier weeks too. Nothing new means no email. |
| **Problem notice** | When something needs your attention: a run failed, a job board has failed three runs in a row, or the AI spend cap is reached. | What happened. At most one a day. |

Nothing is ever sent twice for the same job.

## 1. Make the job-radar email account

1. In a private/incognito window, create a new free Gmail account just for job-radar
   (for example `yourname.jobradar@gmail.com`). Use a strong, unique password.
2. Turn on **2-Step Verification** for it (Google requires this before it allows an app password):
   <https://myaccount.google.com/security>.
3. Create an **app password**: <https://myaccount.google.com/apppasswords>. Name it
   `job-radar`, copy the 16-character password Google shows (once only).

> TODO(screenshot): the Google app passwords page.

Read the [email access section of security-and-privacy.md](security-and-privacy.md#email-access)
first: an app password is a key to that whole account, which is why it should be
a separate account that holds nothing else.

## 2. Put the login in `.env`

In the file `.env` (in the folder you run job-radar from, or in your config
folder), add:

```sh
RADAR_EMAIL_ADDRESS=yourname.jobradar@gmail.com
RADAR_EMAIL_APP_PASSWORD=abcdefghijklmnop
```

`.env` is git-ignored. Never paste the app password into a chat, an issue or a
screenshot. Spaces in the password are fine; job-radar ignores them.

On a Mac you can put it in the file straight from the clipboard without it
appearing on screen:

```sh
printf 'RADAR_EMAIL_APP_PASSWORD=%s\n' "$(pbpaste | tr -d ' ')" >> .env
```

## 3. Say who gets the emails

In `config.yaml`:

```yaml
email:
  to: ["you@your-main-inbox.com"]   # more than one address is allowed
```

Email stays off until `to` has an address. The other settings (`match_alerts`,
`digest`, `digest_time`, and the mail servers) are in
[configuration.md](configuration.md#email-fields). If you want your own heading
for a track in the digest, give the track a `label` (for example
`label: "Lead / hands-on roles"`).

## 4. Keep the emails out of spam

1. Add the job-radar address to your **contacts** in your main inbox.
2. Send yourself a test: `job-radar email-test`. It sends one short email.
3. If it landed in spam, open it and click **Not spam** once.

## 5. Vote by replying

Each job in an email has a number. **Reply** to the email with which ones were
good or bad calls:

```
1 👍, 3 👎
```

or `1 up 3 down`. job-radar reads your replies at the start of its next run (or
straight away with `job-radar feedback sync`) and records the votes. See them
with `job-radar report`. You can also vote from the terminal:
`job-radar feedback <posting-id> up`.

**Did it work?** `job-radar run` and `job-radar feedback sync` print a line for
each vote they record, for example `Recorded 👎 Acme — Staff Engineer`, and
add "(changed from 👍)" if you vote again the other way. The next daily digest
lists the votes recorded since the last one. `job-radar report` shows your
current vote on each job. If you vote twice on one job, the latest vote counts
and the earlier one is kept in the history. If a reply has no vote job-radar can
read, `feedback sync` says so.

Only replies from your own address(es) in `email.to`, to a job-radar email, are
read, and only the votes are used. The votes are recorded for you to see and,
later, to help tune the scoring; they don't change any score by themselves.

See [giving-feedback.md](giving-feedback.md) for jobs that were scored too low,
and for turning mistakes into test cases.

## Useful commands

| Command | What it does |
|---|---|
| `job-radar email-test` | Sends one test email. |
| `job-radar run --dry-run` | Shows what it would email, and sends nothing. |
| `job-radar digest` / `digest --dry-run` | Sends (or shows) the digest now. |
| `job-radar feedback sync` | Reads your email replies now. |
| `job-radar list --low` | Recently seen jobs that scored below the digest line (to spot scores that are too low). |
| `job-radar eval draft <id>` | Make a draft test case from a stored job. |
| `job-radar report` | Recent runs, emails sent and your votes. |

## If something goes wrong

- **"rejected the login"**: the app password is wrong, has been revoked, or 2-Step
  Verification is off on the job-radar account. Create a new app password.
- **"can't reach smtp.gmail.com"**: you may be offline or on a network that blocks
  email ports. Try another network.
- **No email arrived**: check spam, then run `job-radar email-test`. More in
  [troubleshooting.md](troubleshooting.md).
