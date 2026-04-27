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
		res, err := gh.FetchCommits(ctx, github.FetchCommitsOpts{
			Owner: owner, Name: name, Since: since,
		})
		if err != nil {
			return fmt.Errorf("fetch %s: %w", r.FullName, err)
		}
		// Aggregate commits by author login, dropping anonymous and excluded.
		type agg struct {
			contrib  store.ContributorUpsert
			commits  int
			firstAt  time.Time
			lastAt   time.Time
		}
		byLogin := map[string]*agg{}
		for _, c := range res.Commits {
			if c.Author == nil || c.Author.Login == "" {
				continue
			}
			if exclusions.IsExcluded(c.Author.Login) {
				continue
			}
			a := byLogin[c.Author.Login]
			if a == nil {
				a = &agg{
					contrib: store.ContributorUpsert{
						Login:       c.Author.Login,
						DisplayName: c.Author.Name,
						AvatarURL:   c.Author.AvatarURL,
						ProfileURL:  c.Author.URL,
						Bio:         c.Author.Bio,
					},
					firstAt: c.CommittedDate,
					lastAt:  c.CommittedDate,
				}
				byLogin[c.Author.Login] = a
			}
			a.commits++
			if c.CommittedDate.Before(a.firstAt) {
				a.firstAt = c.CommittedDate
			}
			if c.CommittedDate.After(a.lastAt) {
				a.lastAt = c.CommittedDate
			}
		}

		// Persist this repo's batch in a single transaction.
		tx, err := st.DB.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		contribs := make([]store.ContributorUpsert, 0, len(byLogin))
		rows := make([]store.RepoContribution, 0, len(byLogin))
		for login, a := range byLogin {
			contribs = append(contribs, a.contrib)
			rows = append(rows, store.RepoContribution{
				RepoFullName:     r.FullName,
				ContributorLogin: login,
				Commits:          a.commits,
				FirstCommitAt:    a.firstAt,
				LastCommitAt:     a.lastAt,
			})
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
		if err := st.UpdateRepoMetadata(ctx, tx, r.FullName, res.DefaultBranch, res.HeadOID, time.Now().UTC()); err != nil {
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
			"commits", len(res.Commits),
			"contributors", len(byLogin),
			"incremental", !since.IsZero(),
			"elapsed", time.Since(started).Round(time.Millisecond),
			"rateRemaining", remaining)
	}

	return nil
}
