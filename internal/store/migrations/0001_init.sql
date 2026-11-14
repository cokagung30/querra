-- Connection groups, saved connections, and settings (PLAN §3.6). Tables for
-- later features (query history, snippets, known SSH hosts) arrive in the
-- migrations of the milestones that need them.
--
-- No secrets in this database: passwords and passphrases live in the OS
-- keychain under conn/<connection id>/<kind>.

CREATE TABLE connection_groups (
    id         TEXT PRIMARY KEY,
    name       TEXT NOT NULL,
    sort_order INTEGER NOT NULL DEFAULT 0
) STRICT;

CREATE TABLE connections (
    id            TEXT PRIMARY KEY,
    group_id      TEXT REFERENCES connection_groups (id) ON DELETE SET NULL,
    name          TEXT NOT NULL,
    engine        TEXT NOT NULL,
    env           TEXT NOT NULL,
    color         TEXT NOT NULL DEFAULT '',
    host          TEXT NOT NULL DEFAULT '',
    port          INTEGER NOT NULL DEFAULT 0,
    username      TEXT NOT NULL DEFAULT '',
    database_name TEXT NOT NULL DEFAULT '',
    ssl_json      TEXT NOT NULL DEFAULT '{}',
    ssh_json      TEXT NOT NULL DEFAULT '{}',
    options_json  TEXT NOT NULL DEFAULT '{}',
    read_only     INTEGER NOT NULL DEFAULT 0,
    sort_order    INTEGER NOT NULL DEFAULT 0,
    created_at    TEXT NOT NULL,
    updated_at    TEXT NOT NULL
) STRICT;

CREATE INDEX connections_by_group ON connections (group_id, sort_order);

CREATE TABLE settings (
    key        TEXT PRIMARY KEY,
    value_json TEXT NOT NULL
) STRICT;
