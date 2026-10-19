-- Finding jobs by web search (ADR-0037).
--
-- search_runs: one row per run that searched, so the monthly search cap can be
-- enforced and the queries can rotate (the number of queries run so far says
-- where the next run starts).
CREATE TABLE search_runs (
    id        INTEGER PRIMARY KEY AUTOINCREMENT,
    ran_at    TEXT    NOT NULL,           -- RFC3339 UTC
    queries   INTEGER NOT NULL,
    searches  INTEGER NOT NULL,
    cost_usd  REAL    NOT NULL DEFAULT 0
);

-- leads: links from search results that are NOT a posting on a known job board.
-- They are never fetched or scored; the digest shows them as text with a warning.
-- The link is the key, so the same lead is stored (and shown) once.
CREATE TABLE leads (
    url           TEXT PRIMARY KEY,
    host          TEXT    NOT NULL,
    title         TEXT    NOT NULL,
    known         INTEGER NOT NULL DEFAULT 0,  -- 1 = a well-known host shown as a normal link
    first_seen_at TEXT    NOT NULL           -- RFC3339 UTC
);

CREATE INDEX idx_leads_first_seen ON leads (first_seen_at);
