CREATE TABLE administrators (
    id TEXT PRIMARY KEY,
    singleton INTEGER NOT NULL UNIQUE DEFAULT 1 CHECK(singleton = 1),
    email TEXT NOT NULL UNIQUE COLLATE NOCASE,
    password_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL
);
CREATE TABLE sessions (
    token_hash TEXT PRIMARY KEY,
    user_id TEXT NOT NULL REFERENCES administrators(id) ON DELETE CASCADE,
    csrf_token TEXT NOT NULL,
    expires_at INTEGER NOT NULL
);
CREATE INDEX sessions_expiry ON sessions(expires_at);
CREATE TABLE applications (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    source_kind TEXT NOT NULL DEFAULT '',
    source_ref TEXT NOT NULL DEFAULT '',
    method TEXT NOT NULL DEFAULT '',
    state TEXT NOT NULL DEFAULT 'missing',
    created_at INTEGER NOT NULL,
    updated_at INTEGER NOT NULL
);
CREATE TABLE containers (
    id TEXT PRIMARY KEY,
    name TEXT NOT NULL,
    image TEXT NOT NULL,
    state TEXT NOT NULL,
    status TEXT NOT NULL,
    health TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    ports_json TEXT NOT NULL,
    managed INTEGER NOT NULL DEFAULT 0,
    application_id TEXT NOT NULL DEFAULT '',
    project TEXT NOT NULL DEFAULT '',
    service TEXT NOT NULL DEFAULT ''
);
CREATE INDEX containers_application ON containers(application_id);

