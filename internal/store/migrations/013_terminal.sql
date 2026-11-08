CREATE TABLE terminal_sessions (
 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES administrators(id),
 auth_hash TEXT NOT NULL, application_id TEXT NOT NULL, container_id TEXT NOT NULL,
 expires_at INTEGER NOT NULL, ended_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX terminal_sessions_owner ON terminal_sessions(user_id, ended_at, expires_at);
CREATE TABLE terminal_audit (
 sequence INTEGER PRIMARY KEY AUTOINCREMENT, session_id TEXT NOT NULL,
 user_id TEXT NOT NULL, application_id TEXT NOT NULL, container_id TEXT NOT NULL,
 event TEXT NOT NULL, reason TEXT NOT NULL DEFAULT '', created_at INTEGER NOT NULL
);
