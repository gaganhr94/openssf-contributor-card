// Package store wraps the SQLite database used by contribcard.
package store

import (
	"context"
	_ "embed"
	"database/sql"
	"fmt"
	"strings"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

const SchemaVersion = "2"

type Store struct {
	DB *sql.DB
}

func Open(ctx context.Context, path string) (*Store, error) {
	dsn := fmt.Sprintf("file:%s?_pragma=foreign_keys(1)&_pragma=journal_mode(WAL)&_pragma=synchronous(NORMAL)", path)
	db, err := sql.Open("sqlite", dsn)
	if err != nil {
		return nil, fmt.Errorf("open sqlite %s: %w", path, err)
	}
	if err := db.PingContext(ctx); err != nil {
		db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	s := &Store{DB: db}
	if err := s.migrate(ctx); err != nil {
		db.Close()
		return nil, err
	}
	return s, nil
}

func (s *Store) Close() error {
	return s.DB.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	if _, err := s.DB.ExecContext(ctx, schemaSQL); err != nil {
		return fmt.Errorf("apply schema: %w", err)
	}
	// Idempotent column-add for DBs cached at the v1 schema. SQLite has no
	// "ADD COLUMN IF NOT EXISTS"; ignore the duplicate-column error instead.
	if _, err := s.DB.ExecContext(ctx,
		`ALTER TABLE contributions ADD COLUMN issues_opened INTEGER NOT NULL DEFAULT 0`); err != nil {
		if !strings.Contains(err.Error(), "duplicate column") {
			return fmt.Errorf("add issues_opened column: %w", err)
		}
	}
	// Older builds wrote Go's zero time as the literal text
	// "0001-01-01T00:00:00Z" into first_commit_at/last_commit_at when a
	// contributor only had PRs/issues. Replace those with NULL so MIN/MAX
	// upserts can later backfill them with real timestamps.
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE contributions SET first_commit_at = NULL
		 WHERE first_commit_at IS NOT NULL
		   AND first_commit_at LIKE '0001-01-01%'`); err != nil {
		return fmt.Errorf("clear zero first_commit_at: %w", err)
	}
	if _, err := s.DB.ExecContext(ctx,
		`UPDATE contributions SET last_commit_at = NULL
		 WHERE last_commit_at IS NOT NULL
		   AND last_commit_at LIKE '0001-01-01%'`); err != nil {
		return fmt.Errorf("clear zero last_commit_at: %w", err)
	}
	if _, err := s.DB.ExecContext(ctx,
		`INSERT INTO build_meta(key, value) VALUES('schema_version', ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, SchemaVersion); err != nil {
		return fmt.Errorf("write schema_version: %w", err)
	}
	return nil
}
