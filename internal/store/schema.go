package store

const SchemaSQL = `
PRAGMA journal_mode=WAL;
PRAGMA synchronous=NORMAL;
PRAGMA foreign_keys=ON;

CREATE TABLE IF NOT EXISTS sessions (
    id                TEXT PRIMARY KEY,
    user_id           TEXT NOT NULL DEFAULT 'default',
    source            TEXT DEFAULT 'cli',
    model             TEXT,
    system_prompt     TEXT,
    parent_session_id TEXT,
    title             TEXT,
    summary           TEXT,
    tokens            INTEGER DEFAULT 0,
    cost              REAL DEFAULT 0.0,
    created_at        DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at        DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS messages (
    id            INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id    TEXT NOT NULL REFERENCES sessions(id),
    role          TEXT NOT NULL,
    content       TEXT NOT NULL,
    tokens        INTEGER DEFAULT 0,
    tool_calls    TEXT,
    tool_name     TEXT,
    tool_input    TEXT,
    tool_result   TEXT,
    finish_reason TEXT,
    timestamp     DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE VIRTUAL TABLE IF NOT EXISTS messages_fts USING fts5(
    content,
    content='messages',
    content_rowid='id',
    tokenize='unicode61'
);

CREATE TRIGGER IF NOT EXISTS messages_fts_insert AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_delete AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.id, old.content);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_update AFTER UPDATE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content) VALUES ('delete', old.id, old.content);
    INSERT INTO messages_fts(rowid, content) VALUES (new.id, new.content);
END;

CREATE TABLE IF NOT EXISTS skills (
    name        TEXT PRIMARY KEY,
    description TEXT,
    content     TEXT,
    source      TEXT DEFAULT 'learned',
    use_count   INTEGER DEFAULT 0,
    last_used_at DATETIME,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP,
    updated_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS user_observations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    category    TEXT NOT NULL,
    key         TEXT NOT NULL,
    value       TEXT,
    confidence  REAL DEFAULT 0.5,
    observed_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    session_id  TEXT
);

CREATE INDEX IF NOT EXISTS idx_messages_session ON messages(session_id, timestamp);
CREATE INDEX IF NOT EXISTS idx_messages_role ON messages(session_id, role);
CREATE INDEX IF NOT EXISTS idx_sessions_user ON sessions(user_id, updated_at);
CREATE INDEX IF NOT EXISTS idx_sessions_source ON sessions(source);
CREATE INDEX IF NOT EXISTS idx_skills_source ON skills(source);
CREATE INDEX IF NOT EXISTS idx_observations_category ON user_observations(category, key);

CREATE TABLE IF NOT EXISTS skill_invocations (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    skill_id    TEXT NOT NULL,
    session_id  TEXT,
    invoked_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS skill_outcomes (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    skill_id    TEXT NOT NULL,
    session_id  TEXT,
    outcome     TEXT NOT NULL,
    recorded_at DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_skill_invocations_skill ON skill_invocations(skill_id);
CREATE INDEX IF NOT EXISTS idx_skill_outcomes_skill ON skill_outcomes(skill_id, outcome);

CREATE TABLE IF NOT EXISTS message_edges (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    session_id  TEXT NOT NULL REFERENCES sessions(id),
    node_id     TEXT NOT NULL,
    parent_id   TEXT,
    branch_name TEXT,
    role        TEXT NOT NULL,
    created_at  DATETIME DEFAULT CURRENT_TIMESTAMP
);

CREATE INDEX IF NOT EXISTS idx_message_edges_session ON message_edges(session_id, node_id);
CREATE INDEX IF NOT EXISTS idx_message_edges_parent ON message_edges(session_id, parent_id);
`
