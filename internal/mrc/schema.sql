CREATE TABLE IF NOT EXISTS mrc_user_prefs (
    user_id      INTEGER PRIMARY KEY,
    handle       TEXT NOT NULL DEFAULT '',
    handle_color INTEGER NOT NULL DEFAULT 11,
    prefix       TEXT NOT NULL DEFAULT '',
    suffix       TEXT NOT NULL DEFAULT '',
    text_color   INTEGER NOT NULL DEFAULT 7,
    theme        INTEGER NOT NULL DEFAULT 1,
    twit         TEXT NOT NULL DEFAULT '',
    updated_at   TEXT NOT NULL DEFAULT (datetime('now'))
);
