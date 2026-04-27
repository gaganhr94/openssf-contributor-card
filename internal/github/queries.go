package github

import (
	"context"
	"fmt"
	"log/slog"
	"time"
)

// CommitAuthor is the subset of a commit author we care about for cards.
//
// Many commits have no associated GitHub user (committed by an email that
// isn't linked to an account). Those are dropped during ingestion — anonymous
// emails can't be displayed as cards.
type CommitAuthor struct {
	Login     string
	Name      string
	AvatarURL string
	URL       string
	Bio       string
}

type Commit struct {
	OID           string
	CommittedDate time.Time
	Author        *CommitAuthor // nil for un-linked commits
}

// FetchCommitsOpts controls a single repo fetch.
type FetchCommitsOpts struct {
	Owner string
	Name  string
	Since time.Time // zero means full history
}

// FetchCommitsResult holds the aggregated output of a repo fetch.
type FetchCommitsResult struct {
	DefaultBranch string
	HeadOID       string // OID of the most recent commit on the default branch
	Commits       []Commit
}

const repoCommitsQuery = `
query RepoCommits($owner: String!, $name: String!, $cursor: String, $since: GitTimestamp) {
  repository(owner: $owner, name: $name) {
    defaultBranchRef {
      name
      target {
        ... on Commit {
          oid
          committedDate
          history(first: 100, after: $cursor, since: $since) {
            pageInfo { endCursor hasNextPage }
            nodes {
              oid
              committedDate
              author {
                user {
                  login
                  name
                  avatarUrl
                  url
                  bio
                }
              }
            }
          }
        }
      }
    }
  }
  rateLimit { cost remaining resetAt }
}`

type repoCommitsResp struct {
	Repository *struct {
		DefaultBranchRef *struct {
			Name   string `json:"name"`
			Target struct {
				OID           string    `json:"oid"`
				CommittedDate time.Time `json:"committedDate"`
				History       struct {
					PageInfo struct {
						EndCursor   string `json:"endCursor"`
						HasNextPage bool   `json:"hasNextPage"`
					} `json:"pageInfo"`
					Nodes []struct {
						OID           string    `json:"oid"`
						CommittedDate time.Time `json:"committedDate"`
						Author        *struct {
							User *struct {
								Login     string `json:"login"`
								Name      string `json:"name"`
								AvatarURL string `json:"avatarUrl"`
								URL       string `json:"url"`
								Bio       string `json:"bio"`
							} `json:"user"`
						} `json:"author"`
					} `json:"nodes"`
				} `json:"history"`
			} `json:"target"`
		} `json:"defaultBranchRef"`
	} `json:"repository"`
}

// FetchCommits paginates commit history on a repo's default branch and
// returns a flat list of commits with their (linked) authors.
//
// If opts.Since is non-zero, only commits with committedDate > since are
// returned — used for incremental refreshes.
func (c *Client) FetchCommits(ctx context.Context, opts FetchCommitsOpts) (*FetchCommitsResult, error) {
	out := &FetchCommitsResult{}

	var cursor *string
	pageNum := 0
	for {
		pageNum++
		vars := map[string]any{
			"owner":  opts.Owner,
			"name":   opts.Name,
			"cursor": cursor,
		}
		if !opts.Since.IsZero() {
			// GraphQL GitTimestamp is RFC3339.
			vars["since"] = opts.Since.UTC().Format(time.RFC3339)
		}

		var resp repoCommitsResp
		if err := c.Do(ctx, repoCommitsQuery, vars, &resp); err != nil {
			return nil, fmt.Errorf("%s/%s page %d: %w", opts.Owner, opts.Name, pageNum, err)
		}
		if resp.Repository == nil || resp.Repository.DefaultBranchRef == nil {
			// Empty repo, or no default branch. Skip.
			slog.Debug("repo has no default branch (empty?)", "repo", opts.Owner+"/"+opts.Name)
			return out, nil
		}
		ref := resp.Repository.DefaultBranchRef
		if out.DefaultBranch == "" {
			out.DefaultBranch = ref.Name
			out.HeadOID = ref.Target.OID
		}

		for _, n := range ref.Target.History.Nodes {
			commit := Commit{OID: n.OID, CommittedDate: n.CommittedDate}
			if n.Author != nil && n.Author.User != nil {
				commit.Author = &CommitAuthor{
					Login:     n.Author.User.Login,
					Name:      n.Author.User.Name,
					AvatarURL: n.Author.User.AvatarURL,
					URL:       n.Author.User.URL,
					Bio:       n.Author.User.Bio,
				}
			}
			out.Commits = append(out.Commits, commit)
		}

		cost, remaining, reset := c.LastRateLimit()
		slog.Debug("fetched commit page",
			"repo", opts.Owner+"/"+opts.Name,
			"page", pageNum,
			"commits", len(ref.Target.History.Nodes),
			"cost", cost,
			"remaining", remaining,
			"resetIn", time.Until(reset).Round(time.Second))

		if !ref.Target.History.PageInfo.HasNextPage {
			break
		}
		next := ref.Target.History.PageInfo.EndCursor
		cursor = &next
	}

	return out, nil
}
