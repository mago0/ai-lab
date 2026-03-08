CREATE TABLE IF NOT EXISTS messages (
    id             INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id     TEXT,
    role           TEXT NOT NULL CHECK (role IN ('user', 'assistant', 'system')),
    content        TEXT NOT NULL,
    discord_msg_id TEXT,
    cost_usd       REAL,
    model          TEXT,
    created_at     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cron_jobs (
    id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(8)))),
    name            TEXT NOT NULL UNIQUE,
    description     TEXT,
    schedule        TEXT NOT NULL,
    enabled         INTEGER NOT NULL DEFAULT 1,
    prompt          TEXT NOT NULL,
    model           TEXT DEFAULT 'sonnet',
    working_dir     TEXT NOT NULL,
    allowed_tools   TEXT,
    disallowed_tools TEXT,
    max_budget_usd  REAL DEFAULT 1.00,
    timeout_seconds INTEGER DEFAULT 600,
    retry_max       INTEGER DEFAULT 0,
    retry_delay_s   INTEGER DEFAULT 60,
    on_failure      TEXT DEFAULT 'alert',
    tags            TEXT,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS cron_runs (
    id              TEXT PRIMARY KEY DEFAULT (lower(hex(randomblob(8)))),
    job_id          TEXT NOT NULL REFERENCES cron_jobs(id),
    status          TEXT NOT NULL DEFAULT 'pending',
    attempt         INTEGER NOT NULL DEFAULT 1,
    session_id      TEXT,
    pid             INTEGER,
    exit_code       INTEGER,
    output_text     TEXT,
    error_output    TEXT,
    cost_usd        REAL,
    duration_ms     INTEGER,
    stream_log_path TEXT,
    started_at      DATETIME,
    finished_at     DATETIME,
    created_at      DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS sessions (
    id          TEXT PRIMARY KEY,
    source      TEXT NOT NULL,
    cron_job_id TEXT REFERENCES cron_jobs(id),
    status      TEXT DEFAULT 'active',
    model       TEXT,
    total_cost  REAL DEFAULT 0,
    started_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    ended_at    DATETIME
);

CREATE TABLE IF NOT EXISTS activity_log (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    source      TEXT NOT NULL,
    event_type  TEXT NOT NULL,
    summary     TEXT,
    metadata    TEXT,
    session_id  TEXT,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id);
CREATE INDEX IF NOT EXISTS idx_messages_created ON messages(created_at);
CREATE INDEX IF NOT EXISTS idx_cron_runs_job ON cron_runs(job_id);
CREATE INDEX IF NOT EXISTS idx_cron_runs_status ON cron_runs(status);
CREATE INDEX IF NOT EXISTS idx_activity_log_source ON activity_log(source);
CREATE INDEX IF NOT EXISTS idx_activity_log_created ON activity_log(created_at);
