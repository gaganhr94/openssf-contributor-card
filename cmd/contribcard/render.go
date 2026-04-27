package main

import (
	"github.com/spf13/cobra"

	"github.com/ghr/openssf-contributor-card/internal/avatar"
	htmlrender "github.com/ghr/openssf-contributor-card/internal/render/html"
	ogrender "github.com/ghr/openssf-contributor-card/internal/render/og"
	"github.com/ghr/openssf-contributor-card/internal/store"
	"github.com/ghr/openssf-contributor-card/web"
)

func newRenderCmd() *cobra.Command {
	var (
		siteURL string
		skipOG  bool
	)
	cmd := &cobra.Command{
		Use:   "render",
		Short: "Render the static site from the SQLite store into --dist",
		RunE: func(cmd *cobra.Command, args []string) error {
			ctx := cmd.Context()
			st, err := store.Open(ctx, flagDB)
			if err != nil {
				return err
			}
			defer st.Close()

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

			if skipOG {
				return nil
			}
			contributors, err := st.AllContributorAggregates(ctx)
			if err != nil {
				return err
			}
			projects, err := st.AllProjects(ctx)
			if err != nil {
				return err
			}
			og := ogrender.New(flagDistDir, avatar.NewCache(flagAvatarCache))
			return og.RenderAll(ctx, contributors, len(projects))
		},
	}
	cmd.Flags().StringVar(&siteURL, "site-url", "https://ghr.github.io/openssf-contributor-card", "absolute site URL used in OG tags and share links")
	cmd.Flags().BoolVar(&skipOG, "skip-og", false, "skip OG image generation (faster local iteration)")
	return cmd
}
