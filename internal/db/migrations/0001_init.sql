CREATE TABLE IF NOT EXISTS events (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    ts TEXT NOT NULL,
    type TEXT NOT NULL,
    category TEXT NOT NULL,
    source TEXT NOT NULL,
    severity TEXT NOT NULL DEFAULT 'info',
    subject TEXT,
    path TEXT,
    pid INTEGER,
    process_name TEXT,
    cmdline TEXT,
    repo_path TEXT,
    session_id TEXT,
    metadata_json TEXT
);

CREATE INDEX IF NOT EXISTS idx_events_ts ON events(ts);
CREATE INDEX IF NOT EXISTS idx_events_type ON events(type);
CREATE INDEX IF NOT EXISTS idx_events_category ON events(category);
CREATE INDEX IF NOT EXISTS idx_events_path ON events(path);
CREATE INDEX IF NOT EXISTS idx_events_session ON events(session_id);
CREATE INDEX IF NOT EXISTS idx_events_repo_path ON events(repo_path);
CREATE INDEX IF NOT EXISTS idx_events_severity ON events(severity);

CREATE TABLE IF NOT EXISTS sessions (
    id TEXT PRIMARY KEY,
    started_at TEXT NOT NULL,
    ended_at TEXT,
    detected_agent TEXT,
    confidence REAL,
    repo_path TEXT,
    status TEXT NOT NULL,
    summary TEXT,
    risk_count INTEGER DEFAULT 0,
    files_changed_count INTEGER DEFAULT 0,
    commands_seen_count INTEGER DEFAULT 0,
    ports_opened_count INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_sessions_started_at ON sessions(started_at);
CREATE INDEX IF NOT EXISTS idx_sessions_repo_path ON sessions(repo_path);
CREATE INDEX IF NOT EXISTS idx_sessions_status ON sessions(status);

CREATE TABLE IF NOT EXISTS watched_paths (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    path TEXT NOT NULL UNIQUE,
    created_at TEXT NOT NULL,
    enabled BOOLEAN NOT NULL DEFAULT 1
);

CREATE TABLE IF NOT EXISTS config (
    key TEXT PRIMARY KEY,
    value TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS daemon_status (
    id INTEGER PRIMARY KEY CHECK (id = 1),
    pid INTEGER,
    started_at TEXT,
    last_heartbeat_at TEXT,
    status TEXT,
    version TEXT,
    metadata_json TEXT
);
