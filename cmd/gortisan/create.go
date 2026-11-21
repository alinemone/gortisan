package main

import (
	"fmt"
	"os"
	"path"
	"strings"

	"github.com/spf13/cobra"

	"github.com/alinemone/gortisan/internal/generator"
)

func createCommand() *cobra.Command {
	var opts generator.CreateOptions
	cmd := &cobra.Command{
		Use:     "create <name> [<module>]",
		Aliases: []string{"new"},
		Short:   "Create a new project in ./<name>",
		Long: `Create a new project in ./<name>.

  gortisan create blog                       ./blog, module blog
  gortisan create blog github.com/you/blog   ./blog, module github.com/you/blog
  gortisan create github.com/you/blog        the same

A service whose database already exists can leave out migrations:

  gortisan create blog --with postgres,redis,swagger`,
		GroupID: "project",
		RunE: func(cmd *cobra.Command, args []string) error {
			name, module, err := createTarget(args, opts.Module)
			if err != nil {
				return err
			}
			cwd, err := os.Getwd()
			if err != nil {
				return err
			}
			opts.Module = module
			opts.Version = buildVersion()
			report, err := generator.Create(cwd, name, opts)
			printReport(cmd.OutOrStdout(), report)
			return err
		},
	}
	cmd.Flags().StringVar(&opts.Module, "module", "", "Go module path (default: <name>)")
	cmd.Flags().StringVar(&opts.With, "with", "", "components: "+strings.Join(generator.Components, ", ")+", all or none (default: all)")
	cmd.Flags().BoolVar(&opts.NoTidy, "no-tidy", false, `skip "go mod tidy"`)
	return cmd
}

// createTarget reads the project name and module path of create from
//
//	create blog                              name blog, module blog
//	create blog github.com/you/blog          name blog, module github.com/you/blog
//	create github.com/you/blog               name blog, module github.com/you/blog
//
// --module may be given instead of the second argument, not with a different one.
func createTarget(positional []string, flagModule string) (name, module string, err error) {
	switch len(positional) {
	case 1:
		name = positional[0]
		if strings.Contains(name, "/") {
			module, name = name, path.Base(name)
		}
	case 2:
		name, module = positional[0], positional[1]
	default:
		return "", "", usagef("create takes a name, and optionally a module path")
	}
	if flagModule != "" {
		if module != "" && module != flagModule {
			return "", "", fmt.Errorf("module given twice: %s and --module %s", module, flagModule)
		}
		module = flagModule
	}
	return name, module, nil
}

func addCommand() *cobra.Command {
	return &cobra.Command{
		Use:       "add <" + strings.Join(generator.Components, "|") + ">",
		Short:     "Add a component the project was created without",
		GroupID:   "project",
		ValidArgs: generator.Components,
		Args:      cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := project()
			if err != nil {
				return err
			}
			report, err := generator.Add(p, args[0])
			printReport(cmd.OutOrStdout(), report)
			return err
		},
	}
}

func upgradeCommand() *cobra.Command {
	return &cobra.Command{
		Use:   "upgrade [version]",
		Short: "Pin another gortisan version in ./gortisan (default: the latest)",
		Long: `Pin another gortisan version in the project's ./gortisan scripts and
.gortisan.json. The project's code is not changed: every version of the same
major generates code that fits the project (see COMPATIBILITY.md).`,
		GroupID: "project",
		Args:    cobra.MaximumNArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := project()
			if err != nil {
				return err
			}
			version := "latest"
			if len(args) == 1 {
				version = args[0]
			}
			report, err := generator.Upgrade(p, version)
			printReport(cmd.OutOrStdout(), report)
			return err
		},
	}
}
