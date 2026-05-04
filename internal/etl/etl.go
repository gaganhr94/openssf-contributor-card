// Package etl orchestrates the GitHub-fetch → SQLite pipeline.
package etl

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"github.com/ghr/openssf-contributor-card/internal/config"
	"github.com/ghr/openssf-contributor-card/internal/github"
	"github.com/ghr/openssf-contributor-card/internal/store"
)

type Options struct {
	GitHubToken string
	OnlyProject string  // optional slug filter; empty means all
	OnlyRepo    string  // optional owner/name filter; empty means all
	SkipFetch   bool    // if true, only sync the YAML; don't hit GitHub
}

func Run(ctx context.Context, st *store.Store, projects *config.Projects, exclusions *config.ExclusionMatcher, opt Options) error {
	slog.Info("syncing projects and repos from YAML", "projects", len(projects.Projects))
	if err := st.SyncProjectsAndRepos(ctx, projects.Projects); err != nil {
		return err
	}

	repos, err := st.ListRepos(ctx)
	if err != nil {
		return err
	}
	slog.Info("repos in store", "count", len(repos))

	if opt.SkipFetch {
		return nil
	}

	// Filter by --project / --repo if requested. Useful for development.
	if opt.OnlyProject != "" || opt.OnlyRepo != "" {
		filtered := repos[:0]
		for _, r := range repos {
			if opt.OnlyProject != "" && r.ProjectSlug != opt.OnlyProject {
				continue
			}
			if opt.OnlyRepo != "" && !strings.EqualFold(r.FullName, opt.OnlyRepo) {
				continue
			}
			filtered = append(filtered, r)
		}
		repos = filtered
		slog.Info("filtered repos", "count", len(repos))
	}

	gh := github.NewClient(opt.GitHubToken)

	for i, r := range repos {
		owner, name, ok := strings.Cut(r.FullName, "/")
		if !ok {
			slog.Warn("skipping malformed repo full_name", "full_name", r.FullName)
			continue
		}
		var since time.Time
		if r.LastFetchedAt.Valid {
			// Re-fetch from one minute before to forgive clock skew.
			since = r.LastFetchedAt.Time.Add(-1 * time.Minute)
		}

		started := time.Now()

		// Per-(repo, login) aggregation across commits, PRs, and issues.
		type agg struct {
			contrib      store.ContributorUpsert
			commits      int
			prsOpened    int
			prsMerged    int
			issuesOpened int
			firstAt      time.Time
			lastAt       time.Time
			firstKind    string // 'commit' | 'pr' | 'issue'
			firstURL     string
			years        map[int]struct{}
		}
		byLogin := map[string]*agg{}

		// noteFirst updates firstAt/firstKind/firstURL if `at` is the new
		// earliest event. Used by the commit/PR/issue passes below so the
		// stored "first contribution" link points to whichever of the three
		// happened first.
		noteFirst := func(a *agg, at time.Time, kind, url string) {
			if at.IsZero() {
				return
			}
			if a.firstAt.IsZero() || at.Before(a.firstAt) {
				a.firstAt = at
				a.firstKind = kind
				a.firstURL = url
			}
		}

		ensure := func(author *github.CommitAuthor) *agg {
			if author == nil || author.Login == "" {
				return nil
			}
			if exclusions.IsExcluded(author.Login) {
				return nil
			}
			a := byLogin[author.Login]
			if a == nil {
				a = &agg{
					contrib: store.ContributorUpsert{
						Login:       author.Login,
						DisplayName: author.Name,
						AvatarURL:   author.AvatarURL,
						ProfileURL:  author.URL,
						Bio:         author.Bio,
					},
					years: map[int]struct{}{},
				}
				byLogin[author.Login] = a
			}
			return a
		}

		// 1. Commits.
		commitsRes, err := gh.FetchCommits(ctx, github.FetchCommitsOpts{
			Owner: owner, Name: name, Since: since,
		})
		if err != nil {
			return fmt.Errorf("fetch commits %s: %w", r.FullName, err)
		}
		for _, c := range commitsRes.Commits {
			a := ensure(c.Author)
			if a == nil {
				continue
			}
			a.commits++
			noteFirst(a, c.CommittedDate, "commit", c.URL)
			if c.CommittedDate.After(a.lastAt) {
				a.lastAt = c.CommittedDate
			}
			if !c.CommittedDate.IsZero() {
				a.years[c.CommittedDate.UTC().Year()] = struct{}{}
			}
		}

		// 2. PRs.
		prs, err := gh.FetchPRs(ctx, owner, name, since)
		if err != nil {
			return fmt.Errorf("fetch prs %s: %w", r.FullName, err)
		}
		for _, p := range prs {
			a := ensure(p.Author)
			if a == nil {
				continue
			}
			a.prsOpened++
			if p.Merged {
				a.prsMerged++
			}
			// PR/issue authorship counts as a contribution too — without
			// this, contributors who only opened PRs or issues had zero
			// firstAt/lastAt, which hid both the "First contribution" and
			// "Years contributing" sections on their card.
			noteFirst(a, p.CreatedAt, "pr", p.URL)
			if p.CreatedAt.After(a.lastAt) {
				a.lastAt = p.CreatedAt
			}
			if !p.CreatedAt.IsZero() {
				a.years[p.CreatedAt.UTC().Year()] = struct{}{}
			}
		}

		// 3. Issues.
		issues, err := gh.FetchIssues(ctx, owner, name, since)
		if err != nil {
			return fmt.Errorf("fetch issues %s: %w", r.FullName, err)
		}
		for _, is := range issues {
			a := ensure(is.Author)
			if a == nil {
				continue
			}
			a.issuesOpened++
			noteFirst(a, is.CreatedAt, "issue", is.URL)
			if is.CreatedAt.After(a.lastAt) {
				a.lastAt = is.CreatedAt
			}
			if !is.CreatedAt.IsZero() {
				a.years[is.CreatedAt.UTC().Year()] = struct{}{}
			}
		}

		// Persist this repo's batch in a single transaction.
		tx, err := st.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		contribs := make([]store.ContributorUpsert, 0, len(byLogin))
		rows := make([]store.RepoContribution, 0, len(byLogin))
		var yearRows []store.ContributionYear
		for login, a := range byLogin {
			contribs = append(contribs, a.contrib)
			rows = append(rows, store.RepoContribution{
				RepoFullName:          r.FullName,
				ContributorLogin:      login,
				Commits:               a.commits,
				PRsOpened:             a.prsOpened,
				PRsMerged:             a.prsMerged,
				IssuesOpened:          a.issuesOpened,
				FirstCommitAt:         a.firstAt,
				LastCommitAt:          a.lastAt,
				FirstContributionKind: a.firstKind,
				FirstContributionURL:  a.firstURL,
			})
			for y := range a.years {
				yearRows = append(yearRows, store.ContributionYear{
					RepoFullName: r.FullName, ContributorLogin: login, Year: y,
				})
			}
		}
		if err := st.UpsertContributors(ctx, tx, contribs); err != nil {
			tx.Rollback()
			return err
		}
		// Full fetches replace stored counts; incremental fetches add deltas.
		replace := since.IsZero()
		if err := st.ApplyRepoContributions(ctx, tx, rows, replace); err != nil {
			tx.Rollback()
			return err
		}
		if err := st.ApplyContributionYears(ctx, tx, yearRows, replace); err != nil {
			tx.Rollback()
			return err
		}
		if err := st.UpdateRepoMetadata(ctx, tx, r.FullName, commitsRes.DefaultBranch, commitsRes.HeadOID, time.Now().UTC()); err != nil {
			tx.Rollback()
			return err
		}
		if err := tx.Commit(); err != nil {
			return err
		}

		_, remaining, _ := gh.LastRateLimit()
		slog.Info("fetched repo",
			"repo", r.FullName,
			"index", fmt.Sprintf("%d/%d", i+1, len(repos)),
			"commits", len(commitsRes.Commits),
			"prs", len(prs),
			"issues", len(issues),
			"contributors", len(byLogin),
			"incremental", !since.IsZero(),
			"elapsed", time.Since(started).Round(time.Millisecond),
			"rateRemaining", remaining)
	}

	return nil
}
