CREATE TABLE IF NOT EXISTS checkpoints (
    id TEXT PRIMARY KEY,
    created_at TEXT NOT NULL,
    trigger TEXT NOT NULL,
    root TEXT,
    command TEXT,
    reason TEXT,
    snapshot_name TEXT,
    snapshot_date TEXT,
    status TEXT NOT NULL DEFAULT 'saved',
    session_id TEXT,
    restored_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_checkpoints_created_at ON checkpoints(created_at);
CREATE INDEX IF NOT EXISTS idx_checkpoints_root ON checkpoints(root);
