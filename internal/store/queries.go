package store

import (
	"context"
	"database/sql"
	"fmt"

	"github.com/ghr/openssf-contributor-card/internal/config"
)

// SyncProjectsAndRepos upserts the project + repo rows from projects.yaml,
// and removes any repo no longer present in the YAML (cascade-deleting its
// contributions). Projects no longer in the YAML are left intact unless they
// have no repos remaining; that's a safer default than dropping data.
func (s *Store) SyncProjectsAndRepos(ctx context.Context, projects []config.Project) error {
	tx, err := s.DB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()

	wantRepos := map[string]bool{}
	for _, p := range projects {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO projects(slug, name, maturity, url) VALUES(?, ?, ?, ?)
			ON CONFLICT(slug) DO UPDATE SET
				name=excluded.name,
				maturity=excluded.maturity,
				url=excluded.url`,
			p.Slug, p.Name, string(p.Maturity), p.URL); err != nil {
			return fmt.Errorf("upsert project %s: %w", p.Slug, err)
		}
		for _, repo := range p.Repos {
			if _, err := tx.ExecContext(ctx, `
				INSERT INTO repos(full_name, project_slug) VALUES(?, ?)
				ON CONFLICT(full_name) DO UPDATE SET project_slug=excluded.project_slug`,
				repo, p.Slug); err != nil {
				return fmt.Errorf("upsert repo %s: %w", repo, err)
			}
			wantRepos[repo] = true
		}
	}

	rows, err := tx.QueryContext(ctx, `SELECT full_name FROM repos`)
	if err != nil {
		return err
	}
	var stale []string
	for rows.Next() {
		var name string
		if err := rows.Scan(&name); err != nil {
			rows.Close()
			return err
		}
		if !wantRepos[name] {
			stale = append(stale, name)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	rows.Close()
	for _, name := range stale {
		if _, err := tx.ExecContext(ctx, `DELETE FROM repos WHERE full_name = ?`, name); err != nil {
			return fmt.Errorf("delete stale repo %s: %w", name, err)
		}
	}

	return tx.Commit()
}

type RepoRow struct {
	FullName       string
	ProjectSlug    string
	DefaultBranch  sql.NullString
	LastCommitOID  sql.NullString
	LastFetchedAt  sql.NullTime
}

func (s *Store) ListRepos(ctx context.Context) ([]RepoRow, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT full_name, project_slug, default_branch, last_commit_oid, last_fetched_at
		 FROM repos ORDER BY full_name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []RepoRow
	for rows.Next() {
		var r RepoRow
		if err := rows.Scan(&r.FullName, &r.ProjectSlug, &r.DefaultBranch, &r.LastCommitOID, &r.LastFetchedAt); err != nil {
			return nil, err
		}
		out = append(out, r)
	}
	return out, rows.Err()
}
