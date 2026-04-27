package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ghr/openssf-contributor-card/internal/config"
	"github.com/ghr/openssf-contributor-card/internal/etl"
	"github.com/ghr/openssf-contributor-card/internal/store"
)

func newFetchCmd() *cobra.Command {
	var (
		onlyProject string
		onlyRepo    string
		skipFetch   bool
	)
	cmd := &cobra.Command{
		Use:   "fetch",
		Short: "Fetch contributor data from GitHub into the SQLite store",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()

			projects, err := config.LoadProjects(flagProjectsYAML)
			if err != nil {
				return err
			}
			exclusions, err := config.LoadExclusions(flagExclusions)
			if err != nil {
				return err
			}

			st, err := store.Open(ctx, flagDB)
			if err != nil {
				return err
			}
			defer st.Close()

			token := os.Getenv("GH_TOKEN")
			if token == "" {
				token = os.Getenv("GITHUB_TOKEN")
			}
			if token == "" && !skipFetch {
				return fmt.Errorf("GH_TOKEN or GITHUB_TOKEN must be set (or pass --skip-fetch to only sync YAML)")
			}

			return etl.Run(ctx, st, projects, exclusions, etl.Options{
				GitHubToken: token,
				OnlyProject: onlyProject,
				OnlyRepo:    onlyRepo,
				SkipFetch:   skipFetch,
			})
		},
	}
	cmd.Flags().StringVar(&onlyProject, "project", "", "limit to a single project slug (debug)")
	cmd.Flags().StringVar(&onlyRepo, "repo", "", "limit to a single owner/name repo (debug)")
	cmd.Flags().BoolVar(&skipFetch, "skip-fetch", false, "only sync projects.yaml; do not hit GitHub")
	return cmd
}
