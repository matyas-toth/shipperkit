-- Multi-user accounts: remove the single-owner constraint, add roles, account
-- state and provider identities. Child tables are rebuilt so the administrator
-- foreign keys point at the replacement table and remain valid.

CREATE TABLE administrators_new (
    id TEXT PRIMARY KEY,
    email TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL DEFAULT '',
    role TEXT NOT NULL DEFAULT 'admin' CHECK(role IN ('admin','developer','viewer')),
    disabled INTEGER NOT NULL DEFAULT 0,
    password_change_required INTEGER NOT NULL DEFAULT 0,
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL DEFAULT 0,
    last_login_at INTEGER NOT NULL DEFAULT 0
);
INSERT INTO administrators_new(id,email,password_hash,role,disabled,password_change_required,created_at,updated_at,last_login_at)
    SELECT id,email,password_hash,'admin',0,0,created_at,created_at,0 FROM administrators;

CREATE TEMP TABLE sessions_keep AS SELECT * FROM sessions;
CREATE TEMP TABLE factor_enrollments_keep AS SELECT * FROM factor_enrollments;
CREATE TEMP TABLE factors_keep AS SELECT * FROM account_factors;
CREATE TEMP TABLE recovery_keep AS SELECT * FROM factor_recovery_codes;
CREATE TEMP TABLE terminal_keep AS SELECT * FROM terminal_sessions;
CREATE TEMP TABLE tokens_keep AS SELECT * FROM operation_tokens;

DROP TABLE factor_enrollments;
DROP TABLE sessions;
DROP TABLE account_factors;
DROP TABLE factor_recovery_codes;
DROP TABLE terminal_sessions;
DROP TABLE operation_tokens;
DROP TABLE administrators;
ALTER TABLE administrators_new RENAME TO administrators;

CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES administrators(id) ON DELETE CASCADE,
    csrf_token TEXT NOT NULL,
    expires_at INTEGER NOT NULL,
    factor_verified INTEGER NOT NULL DEFAULT 0,
    first_factor TEXT NOT NULL DEFAULT 'password',
    pending_provider TEXT NOT NULL DEFAULT '',
    pending_subject TEXT NOT NULL DEFAULT '',
    pending_email TEXT NOT NULL DEFAULT ''
);
CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE INDEX sessions_user ON sessions(user_id);
CREATE TABLE factor_enrollments (
    session_hash TEXT PRIMARY KEY REFERENCES sessions(token_hash) ON DELETE CASCADE,
    secret TEXT NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE TABLE account_factors (
    user_id TEXT PRIMARY KEY REFERENCES administrators(id) ON DELETE CASCADE,
    secret TEXT NOT NULL,
    last_step INTEGER NOT NULL
);
CREATE TABLE factor_recovery_codes (
    user_id TEXT NOT NULL REFERENCES administrators(id) ON DELETE CASCADE,
    code_hash TEXT NOT NULL,
    PRIMARY KEY(user_id, code_hash)
);
CREATE TABLE terminal_sessions (
	 id TEXT PRIMARY KEY, user_id TEXT NOT NULL REFERENCES administrators(id),
	 auth_hash TEXT NOT NULL, application_id TEXT NOT NULL, container_id TEXT NOT NULL,
	 expires_at INTEGER NOT NULL, ended_at INTEGER NOT NULL DEFAULT 0
);
CREATE INDEX terminal_sessions_owner ON terminal_sessions(user_id, ended_at, expires_at);
CREATE TABLE operation_tokens (
	 id TEXT PRIMARY KEY, name TEXT NOT NULL, token_hash TEXT UNIQUE NOT NULL,
	 user_id TEXT NOT NULL REFERENCES administrators(id), scopes TEXT NOT NULL,
	 applications TEXT NOT NULL, expires_at INTEGER NOT NULL, created_at INTEGER NOT NULL
);

INSERT INTO sessions(token_hash,user_id,csrf_token,expires_at,factor_verified)
    SELECT token_hash,user_id,csrf_token,expires_at,factor_verified FROM sessions_keep;
INSERT INTO factor_enrollments SELECT * FROM factor_enrollments_keep;
INSERT INTO account_factors SELECT * FROM factors_keep;
INSERT INTO factor_recovery_codes SELECT * FROM recovery_keep;
INSERT INTO terminal_sessions SELECT * FROM terminal_keep;
INSERT INTO operation_tokens SELECT * FROM tokens_keep;

DROP TABLE sessions_keep;
DROP TABLE factor_enrollments_keep;
DROP TABLE factors_keep;
DROP TABLE recovery_keep;
DROP TABLE terminal_keep;
DROP TABLE tokens_keep;

CREATE TABLE user_identities (
    provider TEXT NOT NULL,
    subject TEXT NOT NULL,
    user_id TEXT NOT NULL REFERENCES administrators(id) ON DELETE CASCADE,
    email TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    last_login_at INTEGER NOT NULL DEFAULT 0,
    PRIMARY KEY(provider, subject),
    UNIQUE(provider, user_id)
);
CREATE INDEX user_identities_owner ON user_identities(user_id);
CREATE TABLE oauth_states (
    hash TEXT PRIMARY KEY,
    provider TEXT NOT NULL,
    nonce TEXT NOT NULL,
    verifier TEXT NOT NULL,
    redirect TEXT NOT NULL,
    mode TEXT NOT NULL DEFAULT 'login',
    user_id TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE INDEX oauth_states_expiry ON oauth_states(expires_at);

ALTER TABLE account_audit ADD COLUMN actor_id TEXT NOT NULL DEFAULT '';
ALTER TABLE account_audit ADD COLUMN detail TEXT NOT NULL DEFAULT '';

INSERT INTO settings(key,value) VALUES ('auth_password_enabled','true') ON CONFLICT(key) DO NOTHING;
