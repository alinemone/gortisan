package main

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"syscall"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/alinemone/gortisan/internal/generator"
	"github.com/alinemone/gortisan/internal/runner"
)

func runCommand() *cobra.Command {
	var noWatch bool
	cmd := &cobra.Command{
		Use:     "run [command] [-- args...]",
		Aliases: []string{"serve"},
		Short:   "Build and run the service, restarting on every change",
		Long: `Build the project and run one of its commands — serve when none is given —
restarting it whenever a .go file, go.mod or .env changes.

  gortisan run                  go run . serve
  gortisan run migrate -- up    go run . migrate up`,
		GroupID: "run",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			before, programArgs := args, []string(nil)
			if dash := cmd.ArgsLenAtDash(); dash >= 0 {
				before, programArgs = args[:dash], args[dash:]
			}
			if len(before) > 1 {
				return usagef("run takes one command; put its arguments after --")
			}
			name := ""
			if len(before) == 1 {
				name = before[0]
			}
			p, err := project()
			if err != nil {
				return err
			}
			opts := runner.Options{
				Root:   p.Root,
				Watch:  !noWatch,
				Stdout: cmd.OutOrStdout(),
				Stderr: cmd.ErrOrStderr(),
			}
			if runner.HasRootCommand(p.Root) {
				if name == "" {
					name = "serve"
				}
				opts.Target, opts.Args = ".", append([]string{name}, programArgs...)
			} else {
				if opts.Target, err = runner.ResolveTarget(p.Root, p.Module, name); err != nil {
					return err
				}
				opts.Args = programArgs
			}
			ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
			defer stop()
			return runner.Run(ctx, opts)
		},
	}
	cmd.Flags().BoolVar(&noWatch, "no-watch", false, "run once, without watching")
	return cmd
}

func routeListCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "route:list",
		Aliases: []string{"routes"},
		Short:   "List every route: method, path, handler and file",
		GroupID: "run",
		Args:    cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := project()
			if err != nil {
				return err
			}
			routes, err := generator.Routes(p)
			if err != nil {
				return err
			}
			out := cmd.OutOrStdout()
			w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
			fmt.Fprintln(w, "METHOD\tPATH\tHANDLER\tFILE")
			for _, r := range routes {
				fmt.Fprintf(w, "%s\t%s\t%s\t%s\n", r.Method, r.Path, r.Handler, r.File)
			}
			_ = w.Flush()
			fmt.Fprintf(out, "\n%d routes; feature paths are under BASE_PATH (default %s)\n", len(routes), p.Settings.BasePath)
			return nil
		},
	}
}

func swagCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "swag [-- swag init flags]",
		Short: "Regenerate docs/ (Swagger) from the swag comments",
		Long: `Run swag init in the project root, with the swag version the project's
go.mod requires — nothing to install. It reads the general API comments of
main.go and the comments of every handler, and writes docs/.`,
		GroupID: "run",
		Args:    cobra.ArbitraryArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := project()
			if err != nil {
				return err
			}
			if !p.Has("swagger") {
				return fmt.Errorf("this project has no swagger; add it with ./gortisan add swagger")
			}
			swag := exec.Command("go", append([]string{"run", "github.com/swaggo/swag/cmd/swag@" + generator.SwagVersion, "init"}, args...)...)
			swag.Dir = p.Root
			swag.Stdout, swag.Stderr = cmd.OutOrStdout(), cmd.ErrOrStderr()
			return swag.Run()
		},
	}
}
