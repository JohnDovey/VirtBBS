CREATE TABLE IF NOT EXISTS app_usage_daily (
    day                 TEXT NOT NULL PRIMARY KEY,
    unique_users        INTEGER NOT NULL DEFAULT 0,
    messages_downloaded INTEGER NOT NULL DEFAULT 0,
    messages_uploaded   INTEGER NOT NULL DEFAULT 0,
    files_downloaded    INTEGER NOT NULL DEFAULT 0,
    files_uploaded      INTEGER NOT NULL DEFAULT 0
);

CREATE TABLE IF NOT EXISTS app_usage_users (
    day      TEXT NOT NULL,
    user_id  INTEGER NOT NULL,
    PRIMARY KEY (day, user_id)
);

CREATE TABLE IF NOT EXISTS app_usage_totals (
    id                  INTEGER PRIMARY KEY CHECK (id = 1),
    messages_downloaded INTEGER NOT NULL DEFAULT 0,
    messages_uploaded   INTEGER NOT NULL DEFAULT 0,
    files_downloaded    INTEGER NOT NULL DEFAULT 0,
    files_uploaded      INTEGER NOT NULL DEFAULT 0
);
