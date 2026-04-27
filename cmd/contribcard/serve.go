package main

import (
	"fmt"
	"log/slog"
	"net/http"

	"github.com/spf13/cobra"
)

func newServeCmd() *cobra.Command {
	var addr string
	cmd := &cobra.Command{
		Use:   "serve",
		Short: "Serve --dist over HTTP for local preview",
		RunE: func(cmd *cobra.Command, args []string) error {
			mux := http.NewServeMux()
			mux.Handle("/", http.FileServer(http.Dir(flagDistDir)))
			srv := &http.Server{Addr: addr, Handler: mux}
			slog.Info("serving", "addr", addr, "dist", flagDistDir)
			if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
				return fmt.Errorf("serve: %w", err)
			}
			return nil
		},
	}
	cmd.Flags().StringVar(&addr, "addr", ":8080", "listen address")
	return cmd
}
