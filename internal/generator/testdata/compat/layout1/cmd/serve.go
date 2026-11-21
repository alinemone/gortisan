package cmd

import (
	"github.com/spf13/cobra"

	"example.com/compat/internal/app"
)

func serveCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "serve",
		Short: "Serve HTTP on PORT, every feature under BASE_PATH",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return run(app.Serve)
		},
	}
}
