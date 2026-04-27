// Package store wraps the SQLite database used by contribcard.
package store

import (
	"context"
	_ "embed"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

//go:embed schema.sql
var schemaSQL string

const SchemaVersion = "1"

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
	if _, err := s.DB.ExecContext(ctx,
		`INSERT INTO build_meta(key, value) VALUES('schema_version', ?)
		 ON CONFLICT(key) DO UPDATE SET value=excluded.value`, SchemaVersion); err != nil {
		return fmt.Errorf("write schema_version: %w", err)
	}
	return nil
}
