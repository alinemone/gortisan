package generator

import (
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// componentOutputs are the files a --with component brings to a project.
func componentOutputs(component string) []output {
	switch component {
	case "postgres":
		return []output{
			{"internal/platform/database/database.go", "create/database.go.tmpl"},
			{"internal/platform/database/orm/orm.go", "create/orm.go.tmpl"},
			{"internal/platform/database/orm/query.go", "create/query.go.tmpl"},
			{"internal/platform/database/orm/query_test.go", "create/query_test.go.tmpl"},
			{"migrations/migrations.go", "create/migrations_embed.go.tmpl"},
			{"internal/platform/database/migrate/migrate.go", "create/migrate.go.tmpl"},
			{"internal/platform/database/migrate/migrate_test.go", "create/migrate_test.go.tmpl"},
			{"cmd/migrate/main.go", "create/migrate_main.go.tmpl"},
		}
	case "redis":
		return []output{{"internal/platform/cache/redis.go", "create/redis.go.tmpl"}}
	}
	return nil
}

// componentModules are the Go modules a component needs, at the versions
// create pins.
var componentModules = map[string]string{
	"postgres": "github.com/jackc/pgx/v5@v5.13.0",
	"redis":    "github.com/redis/go-redis/v9@v9.23.0",
}

// componentEnv are the .env.example lines of a component.
var componentEnv = map[string]string{
	"postgres": "POSTGRES_HOST=127.0.0.1\nPOSTGRES_PORT=5432\nPOSTGRES_DATABASE=%s\nPOSTGRES_USER=postgres\nPOSTGRES_PASSWORD=postgres\nPOSTGRES_SSLMODE=prefer\n",
	"redis":    "REDIS_ADDR=127.0.0.1:6379\nREDIS_PASSWORD=\nREDIS_DB=0\n",
}

// Add brings a component (postgres or redis) to a project created without it:
// it writes the component's files, its config in a file of its own
// (internal/platform/config/<component>.go), adds its settings to
// .env.example and its module to go.mod. Code you wrote is not changed; the
// report lists the few lines to add to config.go and main.go.
func Add(p *Project, component string) (*Report, error) {
	report := &Report{}
	if !slices.Contains(Components, component) {
		return report, fmt.Errorf("unknown component %q; choose from %s", component, strings.Join(Components, ", "))
	}
	marker := componentOutputs(component)[0].path
	if p.exists(marker) {
		return report, fmt.Errorf("%s is already in this project (%s exists)", component, marker)
	}
	name := filepath.Base(p.Root)
	data := projectData{Name: name, Module: p.Module, Title: title(name), Postgres: component == "postgres", Redis: component == "redis"}
	outputs := append(componentOutputs(component), output{"internal/platform/config/" + component + ".go", "add/config_" + component + ".go.tmpl"})
	if err := renderAll(p.Root, outputs, data, false, report); err != nil {
		return report, err
	}

	if line := componentEnv[component]; line != "" {
		if strings.Contains(line, "%s") {
			line = fmt.Sprintf(line, name)
		}
		envFile := filepath.Join(p.Root, ".env.example")
		if f, err := os.OpenFile(envFile, os.O_APPEND|os.O_WRONLY|os.O_CREATE, 0o644); err == nil {
			_, _ = f.WriteString("\n" + line)
			_ = f.Close()
			report.Updated = append(report.Updated, ".env.example")
		}
	}

	// go get pins the version create uses; go mod tidy then records the
	// checksums of every package the new files import (pgxpool needs more than
	// the pgx module root).
	tidied := true
	for _, args := range [][]string{{"get", componentModules[component]}, {"mod", "tidy"}} {
		cmd := exec.Command("go", args...)
		cmd.Dir = p.Root
		if out, err := cmd.CombinedOutput(); err != nil {
			report.next("go %s failed; run it yourself:\n%s", strings.Join(args, " "), strings.TrimSpace(string(out)))
			tidied = false
			break
		}
	}
	if tidied {
		report.Updated = append(report.Updated, "go.mod", "go.sum")
	}

	switch component {
	case "postgres":
		report.next("internal/platform/config/config.go: add `Postgres Postgres` to Config and `Postgres: r.postgres(),` to Load")
		report.next("cmd/api/main.go: db, err := database.Open(ctx, cfg.Postgres); if err != nil { return err }; defer db.Close()")
		report.next("then: ./gortisan make:model <name> and ./gortisan migrate")
	case "redis":
		report.next("internal/platform/config/config.go: add `Redis Redis` to Config and `Redis: r.redis(),` to Load")
		report.next("cmd/api/main.go: rdb, err := cache.Open(ctx, cfg.Redis); if err != nil { return err }; defer rdb.Close()")
	}
	return report, nil
}

// makeTest writes a test: for a subject (user), a service test with an
// in-memory Store; for an action (v1/user/list), a handler test.
func makeTest(p *Project, name string, opts MakeOptions, report *Report) error {
	if version, f, action, err := splitAPI("test", name); err == nil {
		dir, _ := apiPackage(version, f)
		if err := p.require(dir+"/handler.go", "run make:handler "+name+" first"); err != nil {
			return err
		}
		d := apiData(p, version, f, action)
		src, err := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(dir), "handler.go"))
		if err != nil {
			return err
		}
		file, err := parser.ParseFile(token.NewFileSet(), "handler.go", src, parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		if !slices.Contains(declNames(file), "Handler."+d.Action) {
			return fmt.Errorf("%s/handler.go has no %s; run make:handler %s first", dir, d.Action, name)
		}
		d.TestURL = strings.ReplaceAll(d.Path, ":id", "1")
		return renderAll(p.Root, []output{{dir + "/" + action + "_test.go", "make/handler_test.go.tmpl"}}, d, opts.Force, report)
	} else if versionRE.MatchString(strings.SplitN(name, "/", 2)[0]) {
		return err
	}

	f, err := parseFeature(name)
	if err != nil {
		return err
	}
	if err := p.require(f.Dir()+"/service.go", "run make:model "+f.Path+" and make:service "+f.Path+" first"); err != nil {
		return err
	}
	if err := p.require(f.Dir()+"/store.go", "the service test fakes the Store of make:model "+f.Path); err != nil {
		return err
	}
	return renderAll(p.Root, []output{{f.Dir() + "/service_test.go", "make/service_test.go.tmpl"}}, f.data(p), opts.Force, report)
}
