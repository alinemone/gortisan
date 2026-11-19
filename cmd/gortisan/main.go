// Command gortisan creates Go service projects and adds files to them.
//
// It only writes code. The generated project does not import gortisan and
// does not list it in go.mod; after generation every file is yours.
//
//	go install github.com/alinemone/gortisan/cmd/gortisan@latest
//	gortisan create shop --module github.com/you/shop
//	gortisan make:handler order/get_order   (or: make handler ...)
//
// Inside a project, the generated ./gortisan script (gortisan.bat,
// gortisan.ps1 on Windows) runs the same version that created it.
package main

import (
	"context"
	"fmt"
	"io"
	"os"
	"os/signal"
	"path/filepath"
	"runtime/debug"
	"strings"
	"syscall"

	"github.com/alinemone/gortisan/internal/generator"
	"github.com/alinemone/gortisan/internal/runner"
)

func main() {
	os.Exit(run(os.Args[1:], os.Stdout, os.Stderr))
}

func run(args []string, stdout, stderr io.Writer) int {
	if len(args) == 0 {
		usage(stderr)
		return 2
	}
	// make:handler is the artisan-style spelling of "make handler".
	if kind, ok := strings.CutPrefix(args[0], "make:"); ok {
		return runMake(append([]string{kind}, args[1:]...), stdout, stderr)
	}
	switch args[0] {
	case "create", "new":
		return runCreate(args[1:], stdout, stderr)
	case "make":
		return runMake(args[1:], stdout, stderr)
	case "run", "serve":
		return runRun(args[1:], stdout, stderr)
	case "generate", "gen":
		return runGenerate(args[1:], stdout, stderr)
	case "version", "--version", "-v":
		fmt.Fprintln(stdout, version())
		return 0
	case "help", "--help", "-h", "list":
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
		Module:  f.values["module"],
		With:    f.values["with"],
		NoTidy:  f.bools["no-tidy"],
		Version: buildVersion(),
	})
	printReport(stdout, report)
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}
	return 0
}

func runMake(args []string, stdout, stderr io.Writer) int {
	f, err := parseFlags(args, []string{"table", "fields", "type", "members", "topic", "group"}, []string{"force", "soft-delete"})
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
		Type:       f.values["type"],
		Members:    f.values["members"],
		Topic:      f.values["topic"],
		Group:      f.values["group"],
	})
	printReport(stdout, report)
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}
	return 0
}

// runRun is `gortisan run [command] [--no-watch] [-- args...]`.
func runRun(args []string, stdout, stderr io.Writer) int {
	var programArgs []string
	for i, arg := range args {
		if arg == "--" {
			args, programArgs = args[:i], args[i+1:]
			break
		}
	}
	f, err := parseFlags(args, nil, []string{"no-watch"})
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 2
	}
	if len(f.positional) > 1 {
		fmt.Fprintln(stderr, "usage: gortisan run [command] [--no-watch] [-- args...]")
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
	name := ""
	if len(f.positional) == 1 {
		name = f.positional[0]
	}
	target, err := runner.ResolveTarget(project.Root, project.Module, name)
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	err = runner.Run(ctx, runner.Options{
		Root:   project.Root,
		Target: target,
		Args:   programArgs,
		Watch:  !f.bools["no-watch"],
		Stdout: stdout,
		Stderr: stderr,
		BeforeBuild: func() error {
			if !hasORM(project) {
				return nil
			}
			report, err := generator.Generate(project, generator.GenerateOptions{})
			printReport(stderr, report)
			return err
		},
	})
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}
	return 0
}

// runGenerate is `gortisan generate [--check]`.
func runGenerate(args []string, stdout, stderr io.Writer) int {
	f, err := parseFlags(args, nil, []string{"check"})
	if err != nil || len(f.positional) > 0 {
		fmt.Fprintln(stderr, "usage: gortisan generate [--check]")
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
	report, err := generator.Generate(project, generator.GenerateOptions{Check: f.bools["check"]})
	printReport(stdout, report)
	if err != nil {
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}
	if f.bools["check"] {
		fmt.Fprintln(stdout, "generated files are up to date")
	} else if len(report.Created)+len(report.Updated)+len(report.Removed)+len(report.Next) == 0 {
		fmt.Fprintln(stdout, "nothing to generate; every mapping is up to date")
	}
	return 0
}

func hasORM(p *generator.Project) bool {
	_, err := os.Stat(filepath.Join(p.Root, "internal", "database", "orm", "orm.go"))
	return err == nil
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
		fmt.Fprintln(w, "exists ", path, "(kept; --force overwrites)")
	}
	for _, next := range report.Next {
		fmt.Fprintln(w, "next:  ", next)
	}
}

func version() string {
	return "gortisan " + buildVersion()
}

// buildVersion is the module version this binary was built from: a tag or
// pseudo-version after `go install ...@version`, "(devel)" for a local build.
func buildVersion() string {
	if info, ok := debug.ReadBuildInfo(); ok && info.Main.Version != "" {
		return info.Main.Version
	}
	return "(devel)"
}

func usage(w io.Writer) {
	fmt.Fprint(w, `gortisan writes Go service code. The code it writes never imports gortisan.

Usage:
  gortisan create <name> [flags]        create a new project in ./<name>
      --module <path>                   Go module path (default: <name>)
      --with <list>                     postgres, redis, kafka, metrics, all or none
                                        (default: postgres,redis,metrics)
      --no-tidy                         skip "go mod tidy"

  gortisan run [command] [-- args]      build and run cmd/<command>, restarting on changes
      --no-watch                        run once, without watching
  gortisan generate [--check]           write the table mappings of internal/model
                                        (gortisan run and make:model do it for you)

  gortisan make:<kind> <name> [flags]   add code to the project:
    handler <group>/<action>            internal/handler/<group>_<action>.go
    service <name>                      internal/service/<name>_service.go
    model <name>                        model, repository and migration (postgres)
        --fields "user_id:int64,status:string"   optional; columns to write in
        --table <name>                  (default: plural of <name>)
        --soft-delete                   add deleted_at
    migration <name>                    empty migrations/<time>_<name>.{up,down}.sql
    middleware <name>                   internal/middleware/<name>.go
    request [<group>/]<name>            internal/request/<name>.go
    response [<group>/]<name>           internal/response/<name>.go
    validator [<group>/]<name>          internal/validator/<name>.go (a validate.Rule)
    enum <name> --members <list>        internal/enum/<name>.go
        --members "pending,paid=5,sent" integers count up from 1 or the previous value
        --type <type>                   string or an integer type (default: int64)
    cache <name> [--type <type>]        internal/cache/<name>.go, a Redis cache (redis)
    client <name>                       internal/client/<name>/client.go, an HTTP client
    worker <name>                       internal/worker/<name>.go and cmd/worker (kafka)
        --topic <topic>  --group <group>  (default: <name>)
    cdc <model> [--topic <topic>]       internal/cdc/<model>.go, change events (kafka)

  make flags: --force overwrites existing files. "make handler" (with a space)
  works too. Inside a project, ./gortisan runs the version that created it.

  gortisan list | help | version
`)
}
