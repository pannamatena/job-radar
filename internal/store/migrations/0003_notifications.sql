-- Migration 0003: notifications (phase 4).
--
-- Every email job-radar sends is recorded, so a posting is never alerted on
-- twice on the same channel, the daily digest goes out once a day, and a reply
-- like "1 👎" can be matched back to the posting that was numbered 1 in that
-- email (through the Message-ID header).
CREATE TABLE notifications (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    channel     TEXT NOT NULL,              -- email
    kind        TEXT NOT NULL,              -- alert | digest | problem
    sent_at     TEXT NOT NULL,              -- RFC3339
    subject     TEXT NOT NULL,
    message_id  TEXT,                       -- the Message-ID header we sent
    recipients  TEXT NOT NULL               -- comma-separated
);

CREATE INDEX idx_notifications_kind ON notifications (kind, sent_at);
CREATE INDEX idx_notifications_message_id ON notifications (message_id);

-- The numbered jobs listed in one email.
CREATE TABLE notification_items (
    notification_id INTEGER NOT NULL REFERENCES notifications (id),
    number          INTEGER NOT NULL,       -- the number shown next to the job
    posting_id      TEXT NOT NULL REFERENCES postings (id),
    PRIMARY KEY (notification_id, number)
);

CREATE INDEX idx_notification_items_posting ON notification_items (posting_id);
