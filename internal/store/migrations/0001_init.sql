-- Migration 0001: initial schema for phase 2 (postings + runs).
-- Later phases add scores, notifications, feedback and company_offices tables.

-- Every job-radar has ever seen. The presence of a row is what makes a posting
-- "not new to the user" (ADR-0025): first_seen_at is set once, the first time
-- we see the id, and never changes; last_seen_at is refreshed every run.
CREATE TABLE postings (
    id               TEXT PRIMARY KEY,      -- stable "source:external_id"
    source           TEXT NOT NULL,         -- greenhouse | lever | ashby | ...
    company          TEXT NOT NULL,         -- display name from config
    title            TEXT NOT NULL,
    location         TEXT NOT NULL DEFAULT '',
    remote           TEXT NOT NULL DEFAULT 'unknown',
    url              TEXT NOT NULL DEFAULT '',
    description_text TEXT NOT NULL DEFAULT '',
    posted_at        TEXT,                  -- RFC3339, nullable (board may omit)
    first_seen_at    TEXT NOT NULL,         -- RFC3339, when we first saw it
    last_seen_at     TEXT NOT NULL,         -- RFC3339, refreshed each run
    raw_json         TEXT NOT NULL DEFAULT ''
);

CREATE INDEX idx_postings_company ON postings (company);
CREATE INDEX idx_postings_source ON postings (source);

-- One row per run, for the runs log and `report`.
CREATE TABLE runs (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    started_at    TEXT NOT NULL,
    finished_at   TEXT,
    new_postings  INTEGER NOT NULL DEFAULT 0,
    scored        INTEGER NOT NULL DEFAULT 0,
    cost_estimate REAL NOT NULL DEFAULT 0,
    errors        TEXT
);
