package main

import (
	"errors"
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alinemone/gortisan/internal/generator"
)

// execute runs the command line args and returns the exit code: 0, 1 when a
// command failed, 2 when the command line was wrong.
func execute(args []string) int {
	cobra.EnableCommandSorting = false // help lists commands in the order of rootCommand
	root := rootCommand()
	root.SetArgs(args)
	cmd, err := root.ExecuteC()
	if err == nil {
		return 0
	}
	fmt.Fprintln(root.ErrOrStderr(), "gortisan:", err)
	var usage usageError
	if errors.As(err, &usage) || strings.HasPrefix(err.Error(), "unknown command") {
		fmt.Fprintf(root.ErrOrStderr(), "run '%s --help' for usage\n", cmd.CommandPath())
		return 2
	}
	return 1
}

// rootCommand is gortisan: every command it has is listed here.
func rootCommand() *cobra.Command {
	root := &cobra.Command{
		Use:   "gortisan",
		Short: "gortisan writes Go service code. The code it writes never imports gortisan.",
		Long: `gortisan writes Go service code. The code it writes never imports gortisan.

A project is feature-first: one package per feature under internal/ (entity,
service, storage, handler, DTOs, routes, tests), wiring in internal/app, and
Cobra commands in cmd/. Inside a project, ./gortisan runs the gortisan
version the project pins.`,
		Version:       buildVersion(),
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	root.SetVersionTemplate("gortisan {{.Version}}\n")
	root.SetFlagErrorFunc(func(cmd *cobra.Command, err error) error {
		return usageError{err.Error()}
	})
	root.AddGroup(
		&cobra.Group{ID: "project", Title: "Projects:"},
		&cobra.Group{ID: "make", Title: "Generate code (also: gortisan make <kind> ...):"},
		&cobra.Group{ID: "run", Title: "Inside a project:"},
	)
	root.AddCommand(
		createCommand(),
		addCommand(),
		upgradeCommand(),
		runCommand(),
		routeListCommand(),
		swagCommand(),
		versionCommand(),
	)
	root.AddCommand(makeCommands()...)
	root.AddCommand(makeParentCommand())
	return root
}

// usageError is a command line that does not fit the command.
type usageError struct{ msg string }

func (e usageError) Error() string { return e.msg }

func usagef(format string, args ...any) error {
	return usageError{fmt.Sprintf(format, args...)}
}

func versionCommand() *cobra.Command {
	return &cobra.Command{
		Use:     "version",
		Short:   "Print the gortisan version",
		GroupID: "project",
		Args:    cobra.NoArgs,
		Run: func(cmd *cobra.Command, args []string) {
			fmt.Fprintln(cmd.OutOrStdout(), "gortisan", buildVersion())
		},
	}
}

// buildVersion is the module version this binary was built from: a tag or
// pseudo-version after `go install ...@version`, "(devel)" for a local build.
func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

// project finds the project around the working directory.
func project() (*generator.Project, error) {
	cwd, err := os.Getwd()
	if err != nil {
		return nil, err
	}
	return generator.FindProject(cwd)
}

func printReport(w io.Writer, report *generator.Report) {
	if report == nil {
		return
	}
	for _, path := range report.Created {
		fmt.Fprintln(w, "created", path)
	}
	for _, path := range report.Updated {
		fmt.Fprintln(w, "updated", path)
	}
	for _, path := range report.Removed {
		fmt.Fprintln(w, "removed", path)
	}
	for _, path := range report.Skipped {
		if strings.Contains(path, "already exists") {
			fmt.Fprintln(w, "exists ", path)
		} else {
			fmt.Fprintln(w, "exists ", path, "(kept; --force overwrites)")
		}
	}
	for _, next := range report.Next {
		fmt.Fprintln(w, "next:  ", next)
	}
}
