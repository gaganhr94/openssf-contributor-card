-- contribcard SQLite schema.
-- Single migration for now; bump schema_version in build_meta when changing.

CREATE TABLE IF NOT EXISTS projects (
    slug     TEXT PRIMARY KEY,
    name     TEXT NOT NULL,
    maturity TEXT NOT NULL CHECK (maturity IN ('graduated','incubating','sandbox')),
    url      TEXT
);

CREATE TABLE IF NOT EXISTS repos (
    full_name        TEXT PRIMARY KEY,           -- e.g. sigstore/cosign
    project_slug     TEXT NOT NULL REFERENCES projects(slug) ON DELETE CASCADE,
    default_branch   TEXT,
    last_commit_oid  TEXT,
    last_fetched_at  DATETIME
);
CREATE INDEX IF NOT EXISTS idx_repos_project ON repos(project_slug);

CREATE TABLE IF NOT EXISTS contributors (
    login          TEXT PRIMARY KEY COLLATE NOCASE,
    display_name   TEXT,
    avatar_url     TEXT,
    profile_url    TEXT,
    bio            TEXT,
    first_seen_at  DATETIME,
    last_seen_at   DATETIME
);

CREATE TABLE IF NOT EXISTS contributions (
    repo_full_name     TEXT NOT NULL REFERENCES repos(full_name) ON DELETE CASCADE,
    contributor_login  TEXT NOT NULL REFERENCES contributors(login) ON DELETE CASCADE,
    commits            INTEGER NOT NULL DEFAULT 0,
    prs_opened         INTEGER NOT NULL DEFAULT 0,
    prs_merged         INTEGER NOT NULL DEFAULT 0,
    issues_opened      INTEGER NOT NULL DEFAULT 0,
    first_commit_at    DATETIME,
    last_commit_at     DATETIME,
    PRIMARY KEY (repo_full_name, contributor_login)
);
CREATE INDEX IF NOT EXISTS idx_contributions_login ON contributions(contributor_login);
CREATE INDEX IF NOT EXISTS idx_contributions_repo  ON contributions(repo_full_name);

CREATE TABLE IF NOT EXISTS build_meta (
    key   TEXT PRIMARY KEY,
    value TEXT NOT NULL
);
