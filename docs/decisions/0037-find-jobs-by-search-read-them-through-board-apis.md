## ADR-0037: Find jobs by search, read them through the board APIs

**Status:** Accepted (the decision is made; the feature is being built in this phase)

**Context:** The job-board APIs (Greenhouse, Lever and Ashby) answer for one company at a time: you must already know the company. A watch-list therefore only covers the companies a user already thought of, and in practice that was five companies and about 47 open roles. People also want to ask "every job with this title in my area", the way a search engine does. A web search can answer that, but search results are text and links that anyone on the internet can write. Whatever uses them has to assume some are hostile.

**Options considered:**
1. **Watch-list only.** Safe and free, but only finds companies the user already knows.
2. **Search, then open the pages it finds.** Finds the most, but job-radar would be reading arbitrary websites: more terms-of-use problems, and a hacked or lookalike page becomes a way in.
3. **Search finds candidates, the board APIs read them (chosen).** A search returns links. Only links that pass a strict check as a posting on one of three known job-board hosts are used, and even then job-radar never opens the link itself: it takes the company name and job id from it and asks the official API.
4. **A job-aggregator API.** Possible later; each service has its own coverage, price and terms, which are not researched yet.

**Decision:** Option 3.
- **Strict check, no opening of search links.** A link counts only if it is `https`, has no username or port, its host is exactly one of a short fixed list (Ashby, Greenhouse, Lever), and its path matches a fixed pattern. Anything else, including lookalike hosts such as `jobs.ashbyhq.com.example.net`, is not a posting. For a valid link job-radar builds its own request to the board's API from the company name and job id. Nothing returned by a search is ever fetched.
- **Everything else is a lead, shown as text.** A job found on any other site is listed in the digest without a score, with the site's name written so that email programs cannot turn it into a link (`example[.]com`), and a warning that job-radar does not know the site and the reader should check it is safe before opening it. A small list of well-known hosts (for example LinkedIn) are shown as normal links, matched exactly, and are never fetched by job-radar. A link appearing in a search result is not job-radar reading that site, so ADR-0001 still holds.
- **The search model is separate from the scoring model** (`search.model`, default Haiku). On six queries Haiku and Sonnet found the same postings (22 of 23 in common) and Haiku cost about a third as much.
- **Searches are paid and capped.** A search costs about 1 cent plus tokens. Queries are configured per track, and only the tracks listed in `search.tracks` are searched. Other tracks are matched only against the free watch-list boards. A monthly cap stops searching when it is reached.
- **Site-restricted queries do the work.** In the test, queries restricted to a job-board host found 23 postings at 21 different companies; general queries found none on those hosts and mostly returned aggregators, which become leads.
- **Settings, not code:** which tracks, the queries, how many per run, a list of blocked domains, and a list of companies to ignore.
- **Provider-neutral shape.** The search sits behind a small `Searcher` interface that takes queries and returns candidate links. The Anthropic web search is the first implementation, because the tool belongs to that provider's API. A provider is chosen with `search.provider`, built by a registry in one file, and priced with `search.price_per_search_usd`, so adding another is one new file and one registry line, with no change to the strict check, the board reads, the scoring or the digest. Only Anthropic ships today.
- **Search cannot always be localised** (the tool rejected the country code of one country in testing), so the queries themselves name the place.

**Trade-offs:**
- Leads from unknown sites are not scored, so the best of them may be missed until the user acts on the warning. That is the price of never reading them.
- Reading a posting means fetching its company's whole board once, then keeping only the found job. That is one polite request per board, and it reuses code that already works, at the cost of reading more than strictly needed.
- Search results can be old or already closed. A job that the board no longer lists is dropped, which is also the check that it is still open.
- A search has a cost and adds a dependency on one provider's tool until a second implementation exists.

**Why:** reading only through official APIs, and only after a strict check of where the link points, keeps the tool's rule that it is a polite client of known services, and means a hostile search result can at worst fail the check. Showing other sites as text with a warning keeps the user informed without turning the digest into a way to click a bad link.

**Revisit if:** an aggregator API with clear terms covers the user's market better, or a second search provider is added.

**In one sentence:** "Search finds the links, the job boards' own APIs read the jobs, and a link I don't recognise is only ever shown as text with a warning."
