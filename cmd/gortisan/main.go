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
	"errors"
	"fmt"
	"io"
	"os"
	"os/exec"
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
	// migrate, migrate:<sub> and db:seed run the project's own cmd/migrate
	// and cmd/seed: gortisan holds no database code.
	switch {
	case args[0] == "migrate":
		return runProject("migrate", args[1:], stdout, stderr)
	case strings.HasPrefix(args[0], "migrate:"):
		return runProject("migrate", append([]string{strings.TrimPrefix(args[0], "migrate:")}, args[1:]...), stdout, stderr)
	case args[0] == "db:seed":
		return runProject("seed", args[1:], stdout, stderr)
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
	case "add":
		return runAdd(args[1:], stdout, stderr)
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
	f, err := parseFlags(args, []string{"table", "fields", "type", "members"}, []string{"force", "soft-delete"})
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
	_, err := os.Stat(filepath.Join(p.Root, "internal", "platform", "database", "orm", "orm.go"))
	return err == nil
}

// runProject is `go run ./cmd/<command> args...` in the project root.
func runProject(command string, args []string, stdout, stderr io.Writer) int {
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
	if _, err := os.Stat(filepath.Join(project.Root, "cmd", command, "main.go")); err != nil {
		hint := "it is created with postgres"
		if command == "seed" {
			hint = "run gortisan make:seeder <name> first"
		}
		fmt.Fprintf(stderr, "gortisan: cmd/%s is missing; %s\n", command, hint)
		return 1
	}
	cmd := exec.Command("go", append([]string{"run", "./cmd/" + command}, args...)...)
	cmd.Dir = project.Root
	cmd.Stdin, cmd.Stdout, cmd.Stderr = os.Stdin, stdout, stderr
	if err := cmd.Run(); err != nil {
		var exit *exec.ExitError
		if errors.As(err, &exit) {
			return exit.ExitCode()
		}
		fmt.Fprintln(stderr, "gortisan:", err)
		return 1
	}
	return 0
}

// runAdd is `gortisan add <postgres|redis>`.
func runAdd(args []string, stdout, stderr io.Writer) int {
	if len(args) != 1 {
		fmt.Fprintln(stderr, "usage: gortisan add <postgres|redis>")
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
	report, err := generator.Add(project, args[0])
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
      --with <list>                     postgres, redis, all or none (default: both)
      --no-tidy                         skip "go mod tidy"

  gortisan run [command] [-- args]      build and run cmd/<command>, restarting on changes
      --no-watch                        run once, without watching
  gortisan add <postgres|redis>         add a piece the project was created without

  gortisan migrate                      apply pending migrations (runs go run ./cmd/migrate)
  gortisan migrate:rollback [batches]   undo the last batch (or several)
  gortisan migrate:status               list migrations and whether they ran
  gortisan migrate:fresh --force        drop every table and migrate again (development only)
  gortisan db:seed [name]               run the seeders of internal/seed (go run ./cmd/seed)

  gortisan generate [--check]           write the table mapping of every //gortisan:table
                                        struct into its repository.go
                                        (gortisan run and make:model do it for you)

  gortisan make:<kind> <name> [flags]   add code. A project has four layers:
        internal/domain/<subject>     the business (subjects can be grouped: sales/order)
        internal/api/<version>/       HTTP, one package per resource, named like its URL: users, orders
        internal/platform/            infrastructure

    model <subject>                     <subject>.go, errors.go, store.go, repository.go,
                                        and a migration (postgres)
        --fields "mobile:string,name:string"   optional; or add db-tagged fields later
        --table <name>                  (default: plural of <subject>)
        --soft-delete                   add deleted_at
    service <subject>                   service.go, built on the Store
    handler <version>/<subject>/<action>  e.g. v1/user/list: a method in handler.go,
                                        its input in request.go, its output in response.go
    request <version>/<subject>/<name>  a type in request.go
    response <version>/<subject>/<name> a type in response.go
    validator <subject>/<name>          a validate.Rule in the domain's validator.go
    enum <subject>/<name> --members <list>   <name>.go in the domain, e.g. user/status
        --members "pending,paid=5,sent" integers count up from 1 or the previous value
        --type <type>                   string or an integer type (default: int64)
    cache <subject>/<name> [--type T]   a typed Redis cache in the domain's cache.go (redis)
    middleware <name>                   internal/api/middleware/<name>.go
    client <name>                       internal/platform/<name>/client.go, an HTTP client
    migration <name>                    empty migrations/<time>_<name>.{up,down}.sql
    seeder <name>                       internal/seed/<name>.go, and cmd/seed
    test <subject>                      service_test.go with an in-memory Store
    test <version>/<subject>/<action>   <action>_test.go for that handler

  Files that collect several declarations (handler.go, request.go, ...) are
  appended to; a name that already exists is left alone. --force overwrites
  whole files that make writes. "make handler" (with a space) works too.
  Inside a project, ./gortisan runs the version that created it.

  gortisan list | help | version
`)
}
