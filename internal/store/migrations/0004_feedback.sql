-- Migration 0004: feedback and source health (phase 4).
--
-- feedback: the user's 👍/👎 on a posting, from an email reply or from the
-- terminal. Rows are only ever added; the latest vote for a posting is the one
-- that counts. reply_id (the reply's Message-ID) makes reading the same reply
-- twice harmless.
CREATE TABLE feedback (
    id              INTEGER PRIMARY KEY AUTOINCREMENT,
    posting_id      TEXT NOT NULL REFERENCES postings (id),
    vote            TEXT NOT NULL CHECK (vote IN ('up', 'down')),
    source          TEXT NOT NULL,              -- email | terminal
    notification_id INTEGER REFERENCES notifications (id),
    reply_id        TEXT,                       -- Message-ID of the reply, if by email
    created_at      TEXT NOT NULL               -- RFC3339
);

CREATE UNIQUE INDEX idx_feedback_reply ON feedback (reply_id, posting_id) WHERE reply_id IS NOT NULL;
CREATE INDEX idx_feedback_posting ON feedback (posting_id, created_at);

-- source_status: how many runs in a row each company's job board has failed, so a
-- source that keeps failing can be reported once instead of on every run.
CREATE TABLE source_status (
    company              TEXT PRIMARY KEY,
    consecutive_failures INTEGER NOT NULL DEFAULT 0,
    last_error           TEXT,
    updated_at           TEXT NOT NULL
);
