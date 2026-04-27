package store

import (
	"context"
	"database/sql"
	"time"
)

// ProjectSummary identifies a project by slug + display name + maturity.
type ProjectSummary struct {
	Slug     string
	Name     string
	Maturity string
	URL      string
}

// ContributorAggregate is the per-contributor view used to render a card.
type ContributorAggregate struct {
	Login            string
	DisplayName      string
	AvatarURL        string
	ProfileURL       string
	Bio              string
	TotalCommits     int
	RepoCount        int
	Projects         []ProjectSummary // distinct projects, ordered by slug
	FirstContrib     time.Time
	LastContrib      time.Time
	Rank             int // 1-indexed by total_commits desc; ties get the same rank
	TotalContributors int // for "X / N" display
}

// AllContributorAggregates returns one ContributorAggregate per contributor,
// sorted by total_commits desc then login asc. Includes only contributors
// with at least one recorded commit.
func (s *Store) AllContributorAggregates(ctx context.Context) ([]ContributorAggregate, error) {
	const q = `
		SELECT
			c.login,
			COALESCE(c.display_name, '') AS display_name,
			COALESCE(c.avatar_url, '')   AS avatar_url,
			COALESCE(c.profile_url, '')  AS profile_url,
			COALESCE(c.bio, '')          AS bio,
			SUM(ct.commits)              AS total_commits,
			COUNT(DISTINCT ct.repo_full_name) AS repo_count,
			MIN(ct.first_commit_at)      AS first_at,
			MAX(ct.last_commit_at)       AS last_at
		FROM contributors c
		JOIN contributions ct ON ct.contributor_login = c.login
		GROUP BY c.login
		HAVING SUM(ct.commits) > 0
		ORDER BY total_commits DESC, c.login ASC`
	rows, err := s.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []ContributorAggregate
	for rows.Next() {
		var a ContributorAggregate
		var first, last sql.NullString
		if err := rows.Scan(&a.Login, &a.DisplayName, &a.AvatarURL, &a.ProfileURL, &a.Bio,
			&a.TotalCommits, &a.RepoCount, &first, &last); err != nil {
			return nil, err
		}
		// MIN/MAX over DATETIME columns come back as text via the modernc
		// driver, not auto-converted to time.Time. Parse explicitly.
		if first.Valid {
			a.FirstContrib = parseSQLiteTime(first.String)
		}
		if last.Valid {
			a.LastContrib = parseSQLiteTime(last.String)
		}
		out = append(out, a)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}

	// Attach projects for each contributor, in a single query. Building a map
	// avoids N+1 queries when there are thousands of contributors.
	projects, err := s.contributorProjects(ctx)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Projects = projects[out[i].Login]
	}

	// Compute ranks (dense: ties share a rank, next rank skips appropriately).
	total := len(out)
	for i := range out {
		out[i].TotalContributors = total
	}
	if total > 0 {
		rank := 1
		out[0].Rank = rank
		for i := 1; i < total; i++ {
			if out[i].TotalCommits < out[i-1].TotalCommits {
				rank = i + 1
			}
			out[i].Rank = rank
		}
	}
	return out, nil
}

// contributorProjects returns a map from login -> distinct project list,
// each project sorted by slug.
func (s *Store) contributorProjects(ctx context.Context) (map[string][]ProjectSummary, error) {
	const q = `
		SELECT DISTINCT
			ct.contributor_login,
			p.slug, p.name, p.maturity, COALESCE(p.url, '')
		FROM contributions ct
		JOIN repos r ON r.full_name = ct.repo_full_name
		JOIN projects p ON p.slug = r.project_slug
		ORDER BY ct.contributor_login, p.slug`
	rows, err := s.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	out := map[string][]ProjectSummary{}
	for rows.Next() {
		var login string
		var p ProjectSummary
		if err := rows.Scan(&login, &p.Slug, &p.Name, &p.Maturity, &p.URL); err != nil {
			return nil, err
		}
		out[login] = append(out[login], p)
	}
	return out, rows.Err()
}

// parseSQLiteTime parses the text formats the modernc/sqlite driver writes
// for time.Time values (RFC3339Nano with fractional seconds, or the SQLite
// "YYYY-MM-DD HH:MM:SS" form). Returns the zero time on parse failure.
func parseSQLiteTime(s string) time.Time {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999-07:00",
		"2006-01-02 15:04:05-07:00",
		"2006-01-02 15:04:05",
	} {
		if t, err := time.Parse(layout, s); err == nil {
			return t
		}
	}
	return time.Time{}
}

// AllProjects returns the project list sorted by name.
func (s *Store) AllProjects(ctx context.Context) ([]ProjectSummary, error) {
	rows, err := s.DB.QueryContext(ctx,
		`SELECT slug, name, maturity, COALESCE(url, '') FROM projects ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	var out []ProjectSummary
	for rows.Next() {
		var p ProjectSummary
		if err := rows.Scan(&p.Slug, &p.Name, &p.Maturity, &p.URL); err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	return out, rows.Err()
}
