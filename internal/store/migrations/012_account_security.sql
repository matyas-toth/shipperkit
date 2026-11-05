ALTER TABLE sessions ADD COLUMN factor_verified INTEGER NOT NULL DEFAULT 0;
INSERT OR IGNORE INTO settings(key,value) SELECT 'acme_contact',email FROM administrators;

CREATE TABLE account_factors (
    user_id TEXT PRIMARY KEY REFERENCES administrators(id) ON DELETE CASCADE,
    secret TEXT NOT NULL,
    last_step INTEGER NOT NULL
);
CREATE TABLE factor_enrollments (
    session_hash TEXT PRIMARY KEY REFERENCES sessions(token_hash) ON DELETE CASCADE,
    secret TEXT NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE TABLE factor_recovery_codes (
    user_id TEXT NOT NULL REFERENCES administrators(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    PRIMARY KEY(user_id, code_hash)
);
CREATE TABLE account_audit (
    sequence INTEGER PRIMARY KEY AUTOINCREMENT,
    user_id TEXT NOT NULL,
    action TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
