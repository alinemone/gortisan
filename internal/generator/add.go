package generator

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// componentOutputs are the files a --with component brings to a project.
// The first one marks the component as present.
func componentOutputs(component string) []output {
	switch component {
	case "postgres":
		outputs := []output{{"internal/platform/postgres.go", "create/platform_postgres.go.tmpl"}}
		for _, f := range []string{"orm", "db", "errors", "query", "relation", "repository", "pagination", "tx", "orm_test", "integration_test"} {
			outputs = append(outputs, output{ormDir + "/" + f + ".go", "create/orm/" + f + ".go.tmpl"})
		}
		return outputs
	case "redis":
		return []output{{"internal/platform/redis.go", "create/platform_redis.go.tmpl"}}
	case "migrations":
		return []output{
			{migrationsDir + "/migrations.go", "create/migrations_embed.go.tmpl"},
			{"internal/app/migrate.go", "create/app_migrate.go.tmpl"},
			{"cmd/migrate.go", "create/cmd_migrate.go.tmpl"},
		}
	case "swagger":
		return []output{{"docs/docs.go", "create/docs.go.tmpl"}}
	}
	return nil
}

// componentModules are the Go modules a component needs, at the versions
// create pins.
var componentModules = map[string][]string{
	"postgres":   {"github.com/jackc/pgx/v5@v5.13.0", "github.com/uptrace/bun@v1.3.0", "github.com/uptrace/bun/dialect/pgdialect@v1.3.0"},
	"redis":      {"github.com/redis/go-redis/v9@v9.23.0"},
	"migrations": {"github.com/pressly/goose/v3@v3.30.1"},
	"swagger":    {"github.com/swaggo/echo-swagger/v2@v2.0.1", "github.com/swaggo/swag@" + SwagVersion},
}

// componentEnv are the .env.example lines of a component.
var componentEnv = map[string]string{
	"postgres":   "POSTGRES_HOST=127.0.0.1\nPOSTGRES_PORT=5432\nPOSTGRES_DATABASE=%s\nPOSTGRES_USER=postgres\nPOSTGRES_PASSWORD=postgres\nPOSTGRES_SSLMODE=prefer\n",
	"redis":      "REDIS_ADDR=127.0.0.1:6379\nREDIS_PASSWORD=\nREDIS_DB=0\n",
	"migrations": "# deploy/entrypoint.sh runs `migrate up` before the command when true.\nRUN_MIGRATIONS=false\n",
}

// componentHints are the lines to add by hand after `add`, in code that is
// yours and so is not rewritten.
var componentHints = map[string][]string{
	"postgres": {
		"internal/config/config.go: add `Postgres Postgres` to Config and `Postgres: r.postgres(),` to Load",
		"internal/app/serve.go: db, err := platform.OpenPostgres(ctx, cfg.Postgres); if err != nil { return err }; defer db.Close() — and pass db to features",
	},
	"redis": {
		"internal/config/config.go: add `Redis Redis` to Config and `Redis: r.redis(),` to Load",
		"internal/app/serve.go: rdb, err := platform.OpenRedis(ctx, cfg.Redis); if err != nil { return err }; defer rdb.Close() — and pass rdb to features",
	},
	"migrations": {
		"cmd/root.go: add migrateCommand(), to root.AddCommand",
		"then: go run . migrate up; new ones with ./gortisan make:migration <name>",
	},
	"swagger": {
		`internal/app/serve.go: import echoSwagger "github.com/swaggo/echo-swagger/v2" and _ "<module>/docs", then e.GET("/swagger/*", echoSwagger.WrapHandler)`,
		"main.go: add the general API comments (@title, @version, @BasePath) above func main",
		"then: go mod tidy && ./gortisan swag, and open /swagger/index.html",
	},
}

// Add brings a component to a project created without it: it writes the
// component's files, its config in a file of its own, its settings to
// .env.example, its modules to go.mod and its name to .gortisan.json. Code
// you wrote is not changed; the report lists the few lines to add by hand.
func Add(p *Project, component string) (*Report, error) {
	report := &Report{}
	if err := p.Check(); err != nil {
		return report, err
	}
	if !slices.Contains(Components, component) {
		return report, fmt.Errorf("unknown component %q; choose from %s", component, strings.Join(Components, ", "))
	}
	if p.Has(component) {
		return report, fmt.Errorf("%s is already in this project (see %s)", component, SettingsFile)
	}
	if component == "migrations" && !p.Has("postgres") {
		return report, fmt.Errorf("migrations need postgres; run `./gortisan add postgres` first")
	}

	name := filepath.Base(p.Root)
	with := map[string]bool{component: true}
	for _, c := range p.Settings.With {
		with[c] = true
	}
	data := newProjectData(name, p.Module, with, p.Settings.Version)
	data.BasePath = p.basePath()
	outputs := componentOutputs(component)
	switch component {
	case "postgres", "redis":
		outputs = append(outputs, output{"internal/config/" + component + ".go", "add/config_" + component + ".go.tmpl"})
	case "migrations":
		outputs = append(outputs, output{migrationsDir + "/" + migrationStamp(time.Time{}) + "_init.sql", "create/migration_init.sql.tmpl"})
	}
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

	p.Settings.With = nil
	for _, c := range Components {
		if with[c] {
			p.Settings.With = append(p.Settings.With, c)
		}
	}
	if err := writeSettings(p.Root, p.Settings); err != nil {
		return report, err
	}
	report.Updated = append(report.Updated, SettingsFile)

	// go get pins the version create uses; go mod tidy then records the
	// checksums of every package the new files import.
	tidied := true
	for _, args := range [][]string{append([]string{"get"}, componentModules[component]...), {"mod", "tidy"}} {
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
	for _, hint := range componentHints[component] {
		report.next("%s", strings.ReplaceAll(hint, "<module>", p.Module))
	}
	return report, nil
}
