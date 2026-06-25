-- VirtBBS file directory schema

CREATE TABLE IF NOT EXISTS file_dirs (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    name        TEXT    NOT NULL,           -- display name
    description TEXT    NOT NULL DEFAULT '',
    path        TEXT    NOT NULL,           -- filesystem path relative to files root
    sort_type   INTEGER NOT NULL DEFAULT 0, -- 0=none,1=name asc,2=date asc,3=name desc,4=date desc
    read_sec    INTEGER NOT NULL DEFAULT 10,
    upload_sec  INTEGER NOT NULL DEFAULT 20,
    conference_id INTEGER,                  -- NULL = all conferences
    active      INTEGER NOT NULL DEFAULT 1
);

INSERT OR IGNORE INTO file_dirs (id, name, description, path)
VALUES (1, 'General', 'General file uploads', 'general');

CREATE TABLE IF NOT EXISTS files (
    id          INTEGER PRIMARY KEY AUTOINCREMENT,
    dir_id      INTEGER NOT NULL REFERENCES file_dirs(id),
    filename    TEXT    NOT NULL,
    size        INTEGER NOT NULL DEFAULT 0,
    description TEXT    NOT NULL DEFAULT '',
    uploader    TEXT    NOT NULL DEFAULT '',
    upload_date TEXT    NOT NULL DEFAULT (date('now')),
    downloads   INTEGER NOT NULL DEFAULT 0,
    flagged     INTEGER NOT NULL DEFAULT 0,
    UNIQUE (dir_id, filename)
);

CREATE INDEX IF NOT EXISTS idx_files_dir  ON files(dir_id);
CREATE INDEX IF NOT EXISTS idx_files_name ON files(filename);
