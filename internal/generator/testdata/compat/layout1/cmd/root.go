// Package cmd is the command line of Layout1, one file per command. A
// command only reads its arguments and hands over to internal/app, where the
// service is built.
package cmd

import (
	"context"
	"fmt"
	"os"
	"os/signal"
	"syscall"

	"github.com/spf13/cobra"

	"example.com/compat/internal/config"
)

// Execute runs the command named on the command line, e.g. `layout1 serve`.
// Every command of the service is listed here.
func Execute() {
	root := &cobra.Command{
		Use:          "layout1",
		Short:        "Layout1",
		SilenceUsage: true,
	}
	root.AddCommand(
		serveCommand(),
		migrateCommand(),
	)
	if err := root.Execute(); err != nil {
		os.Exit(1)
	}
}

// run loads the config (.env, then the environment) and calls fn with a
// context that is cancelled on Ctrl+C or SIGTERM.
func run(fn func(ctx context.Context, cfg config.Config) error) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	return fn(ctx, cfg)
}
