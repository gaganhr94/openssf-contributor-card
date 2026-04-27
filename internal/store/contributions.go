package store

import (
	"context"
	"database/sql"
	"fmt"
	"time"
)

type ContributorUpsert struct {
	Login       string
	DisplayName string
	AvatarURL   string
	ProfileURL  string
	Bio         string
}

// UpsertContributors writes contributor profile fields, only overwriting
// non-empty values. first_seen_at is set on insert; last_seen_at is bumped
// on every observation by ApplyRepoContributions.
func (s *Store) UpsertContributors(ctx context.Context, tx *sql.Tx, cs []ContributorUpsert) error {
	const q = `
		INSERT INTO contributors (login, display_name, avatar_url, profile_url, bio, first_seen_at, last_seen_at)
		VALUES (?, ?, ?, ?, ?, ?, ?)
		ON CONFLICT(login) DO UPDATE SET
			display_name = COALESCE(NULLIF(excluded.display_name, ''), contributors.display_name),
			avatar_url   = COALESCE(NULLIF(excluded.avatar_url, ''), contributors.avatar_url),
			profile_url  = COALESCE(NULLIF(excluded.profile_url, ''), contributors.profile_url),
			bio          = COALESCE(NULLIF(excluded.bio, ''), contributors.bio),
			last_seen_at = excluded.last_seen_at`
	now := time.Now().UTC()
	for _, c := range cs {
		if _, err := tx.ExecContext(ctx, q,
			c.Login, c.DisplayName, c.AvatarURL, c.ProfileURL, c.Bio, now, now); err != nil {
			return fmt.Errorf("upsert contributor %s: %w", c.Login, err)
		}
	}
	return nil
}

// RepoContribution is one (repo, contributor) row of aggregate stats coming
// out of a single fetch. ApplyRepoContributions adds these counts onto any
// existing row (incremental fetches just contribute deltas).
type RepoContribution struct {
	RepoFullName     string
	ContributorLogin string
	Commits          int
	FirstCommitAt    time.Time
	LastCommitAt     time.Time
}

// ApplyRepoContributions adds (or initializes) per-repo per-contributor stats.
// Pass `replace=true` for a full re-fetch (overwrites existing counts);
// `replace=false` for incremental (adds counts on top of stored values).
func (s *Store) ApplyRepoContributions(ctx context.Context, tx *sql.Tx, rs []RepoContribution, replace bool) error {
	if replace {
		// Group deletes per-repo so we don't wipe rows for repos not in this batch.
		seen := map[string]bool{}
		for _, r := range rs {
			seen[r.RepoFullName] = true
		}
		for repo := range seen {
			if _, err := tx.ExecContext(ctx,
				`DELETE FROM contributions WHERE repo_full_name = ?`, repo); err != nil {
				return fmt.Errorf("clear contributions for %s: %w", repo, err)
			}
		}
	}

	var q string
	if replace {
		q = `
		INSERT INTO contributions (repo_full_name, contributor_login, commits, first_commit_at, last_commit_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(repo_full_name, contributor_login) DO UPDATE SET
			commits         = excluded.commits,
			first_commit_at = excluded.first_commit_at,
			last_commit_at  = excluded.last_commit_at`
	} else {
		q = `
		INSERT INTO contributions (repo_full_name, contributor_login, commits, first_commit_at, last_commit_at)
		VALUES (?, ?, ?, ?, ?)
		ON CONFLICT(repo_full_name, contributor_login) DO UPDATE SET
			commits         = contributions.commits + excluded.commits,
			first_commit_at = MIN(contributions.first_commit_at, excluded.first_commit_at),
			last_commit_at  = MAX(contributions.last_commit_at, excluded.last_commit_at)`
	}
	for _, r := range rs {
		if _, err := tx.ExecContext(ctx, q,
			r.RepoFullName, r.ContributorLogin, r.Commits, r.FirstCommitAt, r.LastCommitAt); err != nil {
			return fmt.Errorf("apply contribution %s/%s: %w", r.RepoFullName, r.ContributorLogin, err)
		}
	}
	return nil
}

// UpdateRepoMetadata records what we just fetched so the next run can do an
// incremental --since query.
func (s *Store) UpdateRepoMetadata(ctx context.Context, tx *sql.Tx, fullName, defaultBranch, headOID string, fetchedAt time.Time) error {
	_, err := tx.ExecContext(ctx, `
		UPDATE repos
		SET default_branch = ?, last_commit_oid = ?, last_fetched_at = ?
		WHERE full_name = ?`,
		defaultBranch, headOID, fetchedAt, fullName)
	return err
}
