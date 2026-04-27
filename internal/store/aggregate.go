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

// RepoStat is one repo a contributor has touched, with their commit count
// in that repo. Used to render the per-contributor "Repositories" badge row.
type RepoStat struct {
	FullName    string // owner/name
	Commits     int
	PRsOpened   int
	IssuesOpened int
	Total       int
}

// FirstContribution identifies the contributor's earliest known contribution.
// We approximate "earliest" as the minimum first_commit_at recorded across
// all (repo, contributor) rows for that contributor — i.e., the first repo
// in which they showed up.
type FirstContribution struct {
	RepoFullName string
	At           time.Time
}

// ContributorAggregate is the per-contributor view used to render a card.
type ContributorAggregate struct {
	Login              string
	DisplayName        string
	AvatarURL          string
	ProfileURL         string
	Bio                string
	TotalCommits       int
	TotalPRs           int // PRs opened across all repos
	TotalIssues        int // issues opened across all repos
	TotalContributions int // commits + PRs + issues; what we rank by
	RepoCount          int
	Projects           []ProjectSummary  // distinct projects, ordered by slug
	Repos              []RepoStat        // every repo the contributor touched, sorted by count desc
	FirstContribution  FirstContribution // earliest observed contribution
	FirstContrib       time.Time         // alias of FirstContribution.At, kept for templates
	LastContrib        time.Time
	Rank               int // 1-indexed by total_contributions desc; ties share a rank
	TotalContributors  int // for "X / N" display
}

// SinceYear returns the year of the first observed contribution, or 0 if
// there is none recorded (template uses 0 as the "unknown" sentinel).
func (a ContributorAggregate) SinceYear() int {
	if a.FirstContrib.IsZero() {
		return 0
	}
	return a.FirstContrib.Year()
}

// YearsActive returns whole years from first to last contribution. 0 means
// the contributor's whole activity sits inside a single calendar year — we
// render that as "this year" rather than "0 years" in templates.
func (a ContributorAggregate) YearsActive() int {
	if a.FirstContrib.IsZero() || a.LastContrib.IsZero() {
		return 0
	}
	years := a.LastContrib.Year() - a.FirstContrib.Year()
	if years < 0 {
		return 0
	}
	return years
}

// YearsList returns the inclusive range of years between first and last
// contribution. Used for the "Years contributing" badge row matching
// CNCF's contribcard. Approximation — we don't track per-year activity
// separately, so a contributor first seen in 2020 and last in 2024 is
// shown as 2020/2021/2022/2023/2024 even if they had a gap year.
func (a ContributorAggregate) YearsList() []int {
	if a.FirstContrib.IsZero() {
		return nil
	}
	first := a.FirstContrib.Year()
	last := first
	if !a.LastContrib.IsZero() {
		last = a.LastContrib.Year()
	}
	if last < first {
		last = first
	}
	years := make([]int, 0, last-first+1)
	for y := first; y <= last; y++ {
		years = append(years, y)
	}
	return years
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
			SUM(ct.prs_opened)           AS total_prs,
			SUM(ct.issues_opened)        AS total_issues,
			SUM(ct.commits + ct.prs_opened + ct.issues_opened) AS total_contributions,
			COUNT(DISTINCT ct.repo_full_name) AS repo_count,
			MIN(ct.first_commit_at)      AS first_at,
			MAX(ct.last_commit_at)       AS last_at
		FROM contributors c
		JOIN contributions ct ON ct.contributor_login = c.login
		GROUP BY c.login
		HAVING total_contributions > 0
		ORDER BY total_contributions DESC, c.login ASC`
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
			&a.TotalCommits, &a.TotalPRs, &a.TotalIssues, &a.TotalContributions,
			&a.RepoCount, &first, &last); err != nil {
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
	repos, err := s.contributorRepos(ctx)
	if err != nil {
		return nil, err
	}
	firsts, err := s.contributorFirstContributions(ctx)
	if err != nil {
		return nil, err
	}
	for i := range out {
		out[i].Projects = projects[out[i].Login]
		out[i].Repos = repos[out[i].Login]
		if fc, ok := firsts[out[i].Login]; ok {
			out[i].FirstContribution = fc
		}
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
			if out[i].TotalContributions < out[i-1].TotalContributions {
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

// contributorRepos returns a map from login -> per-repo stats, sorted by
// total descending then by repo name.
func (s *Store) contributorRepos(ctx context.Context) (map[string][]RepoStat, error) {
	const q = `
		SELECT
			contributor_login,
			repo_full_name,
			commits,
			prs_opened,
			issues_opened,
			(commits + prs_opened + issues_opened) AS total
		FROM contributions
		ORDER BY contributor_login, total DESC, repo_full_name`
	rows, err := s.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string][]RepoStat{}
	for rows.Next() {
		var login string
		var r RepoStat
		if err := rows.Scan(&login, &r.FullName, &r.Commits, &r.PRsOpened, &r.IssuesOpened, &r.Total); err != nil {
			return nil, err
		}
		out[login] = append(out[login], r)
	}
	return out, rows.Err()
}

// contributorFirstContributions returns a map from login -> the (repo, ts)
// of that contributor's earliest observed activity.
func (s *Store) contributorFirstContributions(ctx context.Context) (map[string]FirstContribution, error) {
	const q = `
		SELECT contributor_login, repo_full_name, first_commit_at
		FROM contributions
		WHERE first_commit_at IS NOT NULL AND first_commit_at <> ''
		ORDER BY contributor_login, first_commit_at ASC`
	rows, err := s.DB.QueryContext(ctx, q)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]FirstContribution{}
	for rows.Next() {
		var login, repo, first string
		if err := rows.Scan(&login, &repo, &first); err != nil {
			return nil, err
		}
		// First row per login (ordered ASC) wins; subsequent rows skipped.
		if _, exists := out[login]; exists {
			continue
		}
		out[login] = FirstContribution{
			RepoFullName: repo,
			At:           parseSQLiteTime(first),
		}
	}
	return out, rows.Err()
}

// parseSQLiteTime parses the text formats the modernc/sqlite driver returns
// for time.Time values. Direct column reads use RFC3339Nano ("2024-01-18T17:44:39Z"),
// but MIN/MAX aggregates return Go's time.Time.String() output
// ("2024-01-18 17:44:39 +0000 UTC"). We try both. Returns the zero time
// on failure (and an obviously-zero result like 0001-01-01 if the row was
// stored as Go's zero time, which the IsZero() check downstream catches).
func parseSQLiteTime(s string) time.Time {
	for _, layout := range []string{
		time.RFC3339Nano,
		time.RFC3339,
		"2006-01-02 15:04:05.999999999 -0700 MST",
		"2006-01-02 15:04:05 -0700 MST",
		"2006-01-02 15:04:05 +0000 UTC",
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
