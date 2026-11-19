// Command gortisan creates Go service projects and adds files to them.
//
// It only writes code. The generated project does not import gortisan and
// does not list it in go.mod; after generation every file is yours.
//
//	go install github.com/alinemone/gortisan/cmd/gortisan@latest
//	gortisan create shop --module github.com/you/shop
//	gortisan make handler order/get_order
package main

import (
	"fmt"
	"io"
	"os"
	"runtime/debug"
	"strings"

	"github.com/alinemone/gortisan/internal/generator"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	switch args[0] {
	case "create", "new":
		return runCreate(args[1:], stdout, stderr)
	case "make":
		return runMake(args[1:], stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version())
		return 0
	case "help", "--help", "-h":
		usage(stdout)
		return 0
	default:
		fmt.Fprintf(stderr, "gortisan: unknown command %q\n\n", args[0])
		usage(stderr)
		return 2
	}
}

// flags is the parsed command line: positional arguments and --name[=value]
// switches, which may appear anywhere.
type flags struct {
	positional []string
	values     map[string]string
	bools      map[string]bool
}

// parseFlags splits args. valueFlags take a value (--table=x or --table x);
// boolFlags take none.
func parseFlags(args []string, valueFlags, boolFlags []string) (*flags, error) {
	f := &flags{values: map[string]string{}, bools: map[string]bool{}}
	isValue := map[string]bool{}
	for _, name := range valueFlags {
		isValue[name] = true
	}
	isBool := map[string]bool{}
	for _, name := range boolFlags {
		isBool[name] = true
	}
	for i := 0; i < len(args); i++ {
		arg := args[i]
		if !strings.HasPrefix(arg, "-") || arg == "-" {
			f.positional = append(f.positional, arg)
			continue
		}
		name, value, hasValue := strings.Cut(strings.TrimLeft(arg, "-"), "=")
		switch {
		case isBool[name]:
			if hasValue {
				return nil, fmt.Errorf("--%s takes no value", name)
			}
			f.bools[name] = true
		case isValue[name]:
			if !hasValue {
				if i+1 >= len(args) {
					return nil, fmt.Errorf("--%s needs a value", name)
				}
				i++
				value = args[i]
			}
			f.values[name] = value
		default:
			return nil, fmt.Errorf("unknown flag %q", arg)
		}
	}
	return f, nil
}

func runCreate(args []string, stdout, stderr io.Writer) int {
	f, err := parseFlags(args, []string{"module", "with"}, []string{"no-tidy"})
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 2
	}
	if len(f.positional) != 1 {
		fmt.Fprintln(stderr, "usage: gortisan create <name> [--module <path>] [--with postgres,redis|all|none] [--no-tidy]")
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}
	report, err := generator.Create(cwd, f.positional[0], generator.CreateOptions{
		Module: f.values["module"],
		With:   f.values["with"],
		NoTidy: f.bools["no-tidy"],
	})
	printReport(stdout, report)
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}
	return 0
}

func runMake(args []string, stdout, stderr io.Writer) int {
	f, err := parseFlags(args, []string{"table", "fields"}, []string{"force", "soft-delete"})
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 2
	}
	if len(f.positional) != 2 {
		fmt.Fprintf(stderr, "usage: gortisan make <%s> <name> [flags]\n", strings.Join(generator.MakeKinds, "|"))
		return 2
	}
	cwd, err := os.Getwd()
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}
	project, err := generator.FindProject(cwd)
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}
	report, err := generator.Make(project, f.positional[0], f.positional[1], generator.MakeOptions{
		Force:      f.bools["force"],
		Table:      f.values["table"],
		Fields:     f.values["fields"],
		SoftDelete: f.bools["soft-delete"],
	})
	printReport(stdout, report)
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}
	return 0
}

func printReport(w io.Writer, report *generator.Report) {
	if report == nil {
		return
	}
	for _, path := range report.Created {
		fmt.Fprintln(w, "created", path)
	}
	for _, path := range report.Skipped {
		fmt.Fprintln(w, "exists ", path, "(kept; --force overwrites)")
	}
	for _, next := range report.Next {
		fmt.Fprintln(w, "next:  ", next)
	}
}

func version() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return "gortisan " + info.Main.Version
	}
	return "gortisan (devel)"
}

func usage(w io.Writer) {
	fmt.Fprint(w, `gortisan writes Go service code. The code it writes never imports gortisan.

Usage:
  gortisan create <name> [flags]        create a new project in ./<name>
      --module <path>                   Go module path (default: <name>)
      --with postgres,redis|all|none    pieces to include (default: all)
      --no-tidy                         skip "go mod tidy"

  gortisan make handler <group>/<action>   internal/handler/<group>_<action>.go
  gortisan make service <name>             internal/service/<name>_service.go
  gortisan make model <name> [flags]       model, repository and migration
      --fields "user_id:int64,status:string,paid_at:time?"
      --table <name>                    (default: plural of <name>)
      --soft-delete                     add deleted_at
  gortisan make migration <name>           empty migrations/<time>_<name>.{up,down}.sql

  make flags: --force overwrites existing files.

  gortisan version
`)
}
