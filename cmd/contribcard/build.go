package main

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/ghr/openssf-contributor-card/internal/avatar"
	"github.com/ghr/openssf-contributor-card/internal/config"
	"github.com/ghr/openssf-contributor-card/internal/etl"
	htmlrender "github.com/ghr/openssf-contributor-card/internal/render/html"
	ogrender "github.com/ghr/openssf-contributor-card/internal/render/og"
	"github.com/ghr/openssf-contributor-card/internal/store"
	"github.com/ghr/openssf-contributor-card/web"
)

func newBuildCmd() *cobra.Command {
	var (
		siteURL string
		dryRun  bool
	)
	cmd := &cobra.Command{
		Use:   "build",
		Short: "Run fetch then render (full pipeline)",
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
			if token == "" {
				return fmt.Errorf("GH_TOKEN or GITHUB_TOKEN must be set")
			}

			if err := etl.Run(ctx, st, projects, exclusions, etl.Options{GitHubToken: token}); err != nil {
				return err
			}

			r, err := htmlrender.New(
				htmlrender.DefaultOptions(siteURL, flagDistDir),
				web.Templates(),
				web.Static(),
			)
			if err != nil {
				return err
			}
			if err := r.Render(ctx, st); err != nil {
				return err
			}

			contributors, err := st.AllContributorAggregates(ctx)
			if err != nil {
				return err
			}
			ogProjects, err := st.AllProjects(ctx)
			if err != nil {
				return err
			}
			og := ogrender.New(flagDistDir, avatar.NewCache(flagAvatarCache))
			if err := og.RenderAll(ctx, contributors, len(ogProjects)); err != nil {
				return err
			}

			if dryRun {
				fmt.Fprintln(os.Stderr, "build: --dry-run set, skipping deploy")
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&siteURL, "site-url", "https://ghr.github.io/openssf-contributor-card", "absolute site URL used in OG tags and share links")
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "skip deploy step (CI sets this on PR builds)")
	return cmd
}
