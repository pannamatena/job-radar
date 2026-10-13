-- Migration 0002: scores (phase 3).
--
-- One row per scoring of a posting, by either scorer. Rows are only ever
-- added, so a posting's history (rules → AI, prompt v1 → v2) is kept for the
-- eval and the report. Token counts and the cost estimate are stored per row:
-- the monthly AI spend cap is the sum of cost_usd this calendar month.
CREATE TABLE scores (
    id                 INTEGER PRIMARY KEY AUTOINCREMENT,
    posting_id         TEXT NOT NULL REFERENCES postings (id),
    scored_at          TEXT NOT NULL,          -- RFC3339
    scorer             TEXT NOT NULL,          -- rules | llm (whose score this is)
    ai_status          TEXT,                   -- scored | cached | not_sent | skipped_cap | unscored; NULL in rules-only mode
    ai_note            TEXT,
    model              TEXT,
    prompt_version     TEXT,                   -- hash of system prompt + profile + rubric
    content_hash       TEXT NOT NULL,          -- hash of the ad as scored; an edited ad is re-scored
    score              INTEGER NOT NULL,
    track              TEXT NOT NULL,
    work_model         TEXT NOT NULL,
    office             TEXT,
    why                TEXT NOT NULL,
    gap                TEXT NOT NULL,
    flags              TEXT NOT NULL DEFAULT '[]', -- JSON array
    turns              INTEGER NOT NULL DEFAULT 0,
    input_tokens       INTEGER NOT NULL DEFAULT 0,
    output_tokens      INTEGER NOT NULL DEFAULT 0,
    cache_write_tokens INTEGER NOT NULL DEFAULT 0,
    cache_read_tokens  INTEGER NOT NULL DEFAULT 0,
    cost_usd           REAL NOT NULL DEFAULT 0,
    schema_failures    INTEGER NOT NULL DEFAULT 0,
    duplicate_of       TEXT                    -- set when this posting was matched to another source's copy (ADR-0026)
);

CREATE INDEX idx_scores_posting ON scores (posting_id, scored_at);
CREATE INDEX idx_scores_scored_at ON scores (scored_at);

-- Runs: record AI usage alongside the existing cost_estimate.
ALTER TABLE runs ADD COLUMN ai_sent INTEGER NOT NULL DEFAULT 0;
ALTER TABLE runs ADD COLUMN ai_skipped INTEGER NOT NULL DEFAULT 0;
