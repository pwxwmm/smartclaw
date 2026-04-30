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
    tool_input,
    tool_result,
    content='messages',
    content_rowid='id',
    tokenize='unicode61'
);

CREATE TRIGGER IF NOT EXISTS messages_fts_insert AFTER INSERT ON messages BEGIN
    INSERT INTO messages_fts(rowid, content, tool_input, tool_result) VALUES (new.id, new.content, new.tool_input, new.tool_result);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_delete AFTER DELETE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content, tool_input, tool_result) VALUES ('delete', old.id, old.content, old.tool_input, old.tool_result);
END;

CREATE TRIGGER IF NOT EXISTS messages_fts_update AFTER UPDATE ON messages BEGIN
    INSERT INTO messages_fts(messages_fts, rowid, content, tool_input, tool_result) VALUES ('delete', old.id, old.content, old.tool_input, old.tool_result);
    INSERT INTO messages_fts(rowid, content, tool_input, tool_result) VALUES (new.id, new.content, new.tool_input, new.tool_result);
END;

CREATE VIRTUAL TABLE IF NOT EXISTS sessions_fts USING fts5(
    title,
    summary,
    content='sessions',
    content_rowid='rowid'
);

CREATE TRIGGER IF NOT EXISTS sessions_fts_insert AFTER INSERT ON sessions BEGIN
    INSERT INTO sessions_fts(rowid, title, summary) VALUES (new.rowid, new.title, new.summary);
END;

CREATE TRIGGER IF NOT EXISTS sessions_fts_delete AFTER DELETE ON sessions BEGIN
    INSERT INTO sessions_fts(sessions_fts, rowid, title, summary) VALUES ('delete', old.rowid, old.title, old.summary);
END;

CREATE TRIGGER IF NOT EXISTS sessions_fts_update AFTER UPDATE ON sessions BEGIN
    INSERT INTO sessions_fts(sessions_fts, rowid, title, summary) VALUES ('delete', old.rowid, old.title, old.summary);
    INSERT INTO sessions_fts(rowid, title, summary) VALUES (new.rowid, new.title, new.summary);
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

CREATE TABLE IF NOT EXISTS shared_sessions (
    share_id   TEXT PRIMARY KEY,
    session_id TEXT NOT NULL REFERENCES sessions(id),
    created_at DATETIME DEFAULT CURRENT_TIMESTAMP,
    expires_at DATETIME,
    view_count INTEGER DEFAULT 0
);

CREATE INDEX IF NOT EXISTS idx_shared_sessions_session ON shared_sessions(session_id);

CREATE TABLE IF NOT EXISTS cost_snapshots (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    date TEXT NOT NULL,
    model TEXT NOT NULL,
    input_tokens INTEGER DEFAULT 0,
    output_tokens INTEGER DEFAULT 0,
    cache_read_tokens INTEGER DEFAULT 0,
    cache_create_tokens INTEGER DEFAULT 0,
    cost_usd REAL DEFAULT 0,
    query_count INTEGER DEFAULT 0,
    UNIQUE(date, model)
);

CREATE INDEX IF NOT EXISTS idx_cost_snapshots_date ON cost_snapshots(date);
CREATE INDEX IF NOT EXISTS idx_cost_snapshots_model ON cost_snapshots(model);

CREATE TABLE IF NOT EXISTS onboarding_states (
    user_id TEXT PRIMARY KEY,
    step INTEGER DEFAULT 0,
    started_at INTEGER,
    done_at INTEGER
);

CREATE TABLE IF NOT EXISTS skill_registry_index (
    name TEXT PRIMARY KEY,
    description TEXT,
    author TEXT,
    version TEXT,
    tags TEXT,
    category TEXT,
    downloads INTEGER DEFAULT 0,
    rating REAL DEFAULT 0,
    source TEXT DEFAULT 'local',
    content TEXT,
    installed_at TEXT,
    updated_at TEXT
);

CREATE INDEX IF NOT EXISTS idx_skill_registry_category ON skill_registry_index(category);
CREATE INDEX IF NOT EXISTS idx_skill_registry_source ON skill_registry_index(source);

CREATE TABLE IF NOT EXISTS embeddings (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    source_type TEXT NOT NULL,
    source_id TEXT NOT NULL,
    content TEXT NOT NULL,
    vector BLOB NOT NULL,
    created_at TEXT NOT NULL,
    UNIQUE(source_type, source_id)
);
CREATE INDEX IF NOT EXISTS idx_embeddings_source ON embeddings(source_type, source_id);

CREATE TABLE IF NOT EXISTS teams (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    description TEXT DEFAULT '',
    settings TEXT DEFAULT '{}',
    created_at TEXT NOT NULL,
    updated_at TEXT NOT NULL
);

CREATE TABLE IF NOT EXISTS team_members (
    team_id TEXT NOT NULL,
    user_id TEXT NOT NULL,
    role TEXT DEFAULT 'member',
    joined_at TEXT NOT NULL,
    PRIMARY KEY (team_id, user_id),
    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
);

CREATE TABLE IF NOT EXISTS team_memories (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    team_id TEXT NOT NULL,
    memory_id TEXT NOT NULL DEFAULT '',
    title TEXT NOT NULL,
    content TEXT NOT NULL,
    type TEXT DEFAULT 'conversation',
    visibility TEXT DEFAULT 'team',
    tags TEXT DEFAULT '[]',
    author_id TEXT DEFAULT '',
    created_at TEXT NOT NULL,
    FOREIGN KEY (team_id) REFERENCES teams(id) ON DELETE CASCADE
);

CREATE INDEX IF NOT EXISTS idx_team_memories_team ON team_memories(team_id);
`
