package main

import (
	"github.com/spf13/cobra"

	"github.com/alinemone/gortisan/internal/generator"
)

// kind is one `make:<kind>` command.
type kind struct {
	name, args, short string
	flags             func(cmd *cobra.Command, opts *generator.MakeOptions)
}

// kinds are what make writes, in the order help lists them.
var kinds = []kind{
	{"feature", "<name>", "A feature package: entity, service, storage, handler, DTOs, routes, test — wired into internal/app", featureFlags},
	{"handler", "<feature>/<action>", "An endpoint: a handler method, its request and response, and its route", nil},
	{"migration", "<name>", "An empty SQL migration in internal/migrations", nil},
	{"enum", "<feature>/<name>", "An enum type in a feature", enumFlags},
	{"cache", "<feature>/<name>", "A typed Redis cache in a feature's cache.go", cacheFlags},
	{"validator", "[<group>/]<name>", "A reusable validator in internal/server/validator, with its message key", nil},
	{"middleware", "<name>", "An echo middleware in internal/server/middleware", nil},
	{"client", "<name>", "An HTTP client for one of our services, in internal/clients", nil},
	{"external", "<name>", "An HTTP client for a third-party API, in internal/external", nil},
	{"test", "<feature>[/<action>]", "A service test, or an HTTP test of one action", nil},
}

// makeCommands are make:feature, make:handler, ...
func makeCommands() []*cobra.Command {
	var cmds []*cobra.Command
	for _, k := range kinds {
		cmd := makeCommand(k, "make:"+k.name)
		cmd.GroupID = "make"
		cmds = append(cmds, cmd)
	}
	return cmds
}

// makeParentCommand is `make <kind> <name>`, the same commands spelled with
// a space.
func makeParentCommand() *cobra.Command {
	parent := &cobra.Command{
		Use:    "make <kind> <name>",
		Short:  "The make:<kind> commands, spelled with a space",
		Hidden: true,
	}
	for _, k := range kinds {
		parent.AddCommand(makeCommand(k, k.name))
	}
	return parent
}

func makeCommand(k kind, use string) *cobra.Command {
	var opts generator.MakeOptions
	cmd := &cobra.Command{
		Use:   use + " " + k.args,
		Short: k.short,
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := project()
			if err != nil {
				return err
			}
			report, err := generator.Make(p, k.name, args[0], opts)
			printReport(cmd.OutOrStdout(), report)
			return err
		},
	}
	cmd.Flags().BoolVar(&opts.Force, "force", false, "overwrite whole files that make writes")
	if k.flags != nil {
		k.flags(cmd, &opts)
	}
	return cmd
}

func featureFlags(cmd *cobra.Command, opts *generator.MakeOptions) {
	cmd.Flags().StringVar(&opts.Fields, "fields", "", `columns, e.g. "mobile:string,paid_at:time?" — string, int, int32, int16, float, bool, time, json; ? makes it nullable`)
	cmd.Flags().StringVar(&opts.Table, "table", "", "table name (default: plural of the feature)")
	cmd.Flags().BoolVar(&opts.SoftDelete, "soft-delete", false, "add deleted_at; Delete then sets it instead of removing the row")
}

func enumFlags(cmd *cobra.Command, opts *generator.MakeOptions) {
	cmd.Flags().StringVar(&opts.Members, "members", "", `the values, e.g. "pending,paid=5,sent": integers count up from 1 or the previous value`)
	cmd.Flags().StringVar(&opts.Type, "type", "", "string or an integer type (default: int64)")
}

func cacheFlags(cmd *cobra.Command, opts *generator.MakeOptions) {
	cmd.Flags().StringVar(&opts.Type, "type", "", `the cached value type, e.g. "*User" (default: string)`)
}
