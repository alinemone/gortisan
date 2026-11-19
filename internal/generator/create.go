package generator

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
)

// Components are the optional pieces `create --with` picks.
var Components = []string{"postgres", "redis", "kafka", "metrics"}

// DefaultComponents are what create includes without --with.
var DefaultComponents = []string{"postgres", "redis", "metrics"}

// CreateOptions configure Create.
type CreateOptions struct {
	// Module is the Go module path; the project name when empty.
	Module string
	// With is a comma-separated list of Components, "all" or "none";
	// DefaultComponents when empty.
	With string
	// NoTidy skips `go mod tidy` after writing the files.
	NoTidy bool
	// Version is the gortisan version the project's ./gortisan scripts run;
	// "latest" when empty.
	Version string
}

// ToolPackage is what the project's ./gortisan scripts `go run`.
const ToolPackage = "github.com/alinemone/gortisan/cmd/gortisan"

type projectData struct {
	Name     string
	Module   string
	Title    string
	Postgres bool
	Redis    bool
	Kafka    bool
	Metrics  bool
	Tool     string // ToolPackage@version
}

// Create writes a new project into dir/name.
func Create(dir, name string, opts CreateOptions) (*Report, error) {
	report := &Report{}
	if name == "" || strings.ContainsAny(name, `/\:*?"<>|`) || strings.HasPrefix(name, ".") {
		return report, fmt.Errorf("invalid project name %q", name)
	}
	with, err := parseWith(opts.With)
	if err != nil {
		return report, err
	}
	module := opts.Module
	if module == "" {
		module = name
	}

	root := filepath.Join(dir, name)
	if entries, err := os.ReadDir(root); err == nil && len(entries) > 0 {
		return report, fmt.Errorf("%s already exists and is not empty", root)
	} else if err != nil && !errors.Is(err, os.ErrNotExist) {
		return report, err
	}

	data := projectData{
		Name:     name,
		Module:   module,
		Title:    title(name),
		Postgres: with["postgres"],
		Redis:    with["redis"],
		Kafka:    with["kafka"],
		Metrics:  with["metrics"],
		Tool:     ToolPackage + "@" + toolVersion(opts.Version),
	}

	outputs := []output{
		{"go.mod", "create/go.mod.tmpl"},
		{"cmd/" + name + "/main.go", "create/main.go.tmpl"},
		{"internal/config/config.go", "create/config.go.tmpl"},
		{"internal/server/server.go", "create/server.go.tmpl"},
		{"internal/server/server_test.go", "create/server_test.go.tmpl"},
		{"internal/router/router.go", "create/router.go.tmpl"},
		{"internal/handler/health.go", "create/health.go.tmpl"},
		{"internal/validate/validate.go", "create/validate.go.tmpl"},
		{"internal/validate/validate_test.go", "create/validate_test.go.tmpl"},
		{".env.example", "create/env.example.tmpl"},
		{".gitignore", "create/gitignore.tmpl"},
		{"Dockerfile", "create/Dockerfile.tmpl"},
		{"Makefile", "create/Makefile.tmpl"},
		{"README.md", "create/README.md.tmpl"},
		{".gitattributes", "create/gitattributes.tmpl"},
		{"gortisan", "create/gortisan.sh.tmpl"},
		{"gortisan.bat", "create/gortisan.bat.tmpl"},
		{"gortisan.ps1", "create/gortisan.ps1.tmpl"},
	}
	if data.Postgres {
		outputs = append(outputs,
			output{"internal/database/database.go", "create/database.go.tmpl"},
			output{"internal/database/orm/orm.go", "create/orm.go.tmpl"},
			output{"internal/database/orm/query.go", "create/query.go.tmpl"},
			output{"internal/database/orm/query_test.go", "create/query_test.go.tmpl"},
			output{"migrations/.gitkeep", "create/gitkeep.tmpl"},
		)
	}
	if data.Redis {
		outputs = append(outputs, output{"internal/cache/redis.go", "create/redis.go.tmpl"})
	}
	if data.Kafka {
		outputs = append(outputs, output{"internal/kafka/kafka.go", "create/kafka.go.tmpl"})
	}
	if data.Metrics {
		outputs = append(outputs, output{"internal/metrics/metrics.go", "create/metrics.go.tmpl"})
	}

	if err := renderAll(root, outputs, data, false, report); err != nil {
		return report, err
	}
	if err := os.Chmod(filepath.Join(root, "gortisan"), 0o755); err != nil {
		return report, err
	}

	if opts.NoTidy {
		report.next("cd %s && go mod tidy", name)
	} else if out, err := goModTidy(root); err != nil {
		report.next("go mod tidy failed, run it yourself in %s:\n%s", name, strings.TrimSpace(out))
	}
	report.next("cd %s && cp .env.example .env", name)
	report.next("go run ./cmd/%s   # serves on PORT (8080): /ping, /health", name)
	report.next("./gortisan make:handler order/get_order   # gortisan.bat / gortisan.ps1 on Windows")
	return report, nil
}

// toolVersion is the version to pin: a release or pseudo-version, never a
// local "(devel)" build, which `go run` could not fetch.
func toolVersion(v string) string {
	if v == "" || v == "(devel)" || strings.HasSuffix(v, "+dirty") {
		return "latest"
	}
	return v
}

func parseWith(value string) (map[string]bool, error) {
	with := map[string]bool{}
	switch strings.TrimSpace(value) {
	case "":
		for _, c := range DefaultComponents {
			with[c] = true
		}
		return with, nil
	case "all":
		for _, c := range Components {
			with[c] = true
		}
		return with, nil
	case "none":
		return with, nil
	}
	for _, part := range strings.Split(value, ",") {
		part = strings.TrimSpace(part)
		if !slices.Contains(Components, part) {
			return nil, fmt.Errorf("unknown component %q in --with; choose from %s, all or none", part, strings.Join(Components, ", "))
		}
		with[part] = true
	}
	return with, nil
}

func goModTidy(root string) (string, error) {
	cmd := exec.Command("go", "mod", "tidy")
	cmd.Dir = root
	out, err := cmd.CombinedOutput()
	return string(out), err
}

func title(name string) string {
	words := strings.FieldsFunc(name, func(r rune) bool { return r == '-' || r == '_' || r == '.' || r == ' ' })
	for i, word := range words {
		words[i] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}
