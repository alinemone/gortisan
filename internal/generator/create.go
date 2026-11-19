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
var Components = []string{"postgres", "redis"}

// DefaultComponents are what create includes without --with.
var DefaultComponents = []string{"postgres", "redis"}

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
	Tool     string // ToolPackage@version
	Version  string // the API version a routes file is for, e.g. v1
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
		Tool:     ToolPackage + "@" + toolVersion(opts.Version),
		Version:  "v1",
	}

	outputs := []output{
		{"go.mod", "create/go.mod.tmpl"},
		{"cmd/api/main.go", "create/main.go.tmpl"},
		{"internal/arch_test.go", "create/arch_test.go.tmpl"},
		{"internal/api/router.go", "create/api_router.go.tmpl"},
		{"internal/api/v1/routes.go", "create/version_routes.go.tmpl"},
		{"internal/platform/config/config.go", "create/config.go.tmpl"},
		{"internal/platform/server/server.go", "create/server.go.tmpl"},
		{"internal/platform/server/server_test.go", "create/server_test.go.tmpl"},
		{"internal/platform/health/health.go", "create/health.go.tmpl"},
		{"internal/platform/validate/validate.go", "create/validate.go.tmpl"},
		{"internal/platform/validate/validate_test.go", "create/validate_test.go.tmpl"},
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
		outputs = append(outputs, componentOutputs("postgres")...)
	}
	if data.Redis {
		outputs = append(outputs, componentOutputs("redis")...)
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
	report.next("./gortisan run   # or go run ./cmd/api; serves on PORT (8080): /ping, /health")
	report.next("./gortisan make:model user, make:service user, make:handler v1/user/list   # gortisan.bat / gortisan.ps1 on Windows")
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
