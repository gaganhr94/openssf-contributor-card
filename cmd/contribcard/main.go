// contribcard is the OpenSSF Contributor Card site generator.
package main

import (
	"context"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"
)

var (
	flagDB           string
	flagProjectsYAML string
	flagExclusions   string
	flagDistDir      string
	flagAvatarCache  string
	flagVerbose      bool
)

func main() {
	rootCmd := &cobra.Command{
		Use:           "contribcard",
		Short:         "OpenSSF Contributor Card site generator",
		SilenceUsage:  true,
		SilenceErrors: true,
		PersistentPreRun: func(cmd *cobra.Command, args []string) {
			level := slog.LevelInfo
			if flagVerbose {
				level = slog.LevelDebug
			}
			slog.SetDefault(slog.New(slog.NewTextHandler(os.Stderr, &slog.HandlerOptions{Level: level})))
		},
	}

	rootCmd.PersistentFlags().StringVar(&flagDB, "db", "build.db", "SQLite database path")
	rootCmd.PersistentFlags().StringVar(&flagProjectsYAML, "projects", "data/projects.yaml", "projects YAML path")
	rootCmd.PersistentFlags().StringVar(&flagExclusions, "exclusions", "data/excluded_logins.yaml", "excluded logins YAML path")
	rootCmd.PersistentFlags().StringVar(&flagDistDir, "dist", "dist", "static output directory")
	rootCmd.PersistentFlags().StringVar(&flagAvatarCache, "avatar-cache", ".cache/avatars", "avatar cache directory")
	rootCmd.PersistentFlags().BoolVarP(&flagVerbose, "verbose", "v", false, "verbose logging")

	rootCmd.AddCommand(
		newFetchCmd(),
		newRenderCmd(),
		newBuildCmd(),
		newServeCmd(),
	)

	ctx, cancel := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer cancel()

	if err := rootCmd.ExecuteContext(ctx); err != nil {
		slog.Error("command failed", "err", err)
		os.Exit(1)
	}
}
