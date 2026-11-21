package generator

import (
	"errors"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"time"
)

// Components are the optional pieces `create --with` picks.
var Components = []string{"postgres", "redis", "migrations", "swagger"}

// DefaultComponents are what create includes without --with.
var DefaultComponents = Components

// DefaultBasePath is where features are mounted unless BASE_PATH says otherwise.
const DefaultBasePath = "/api"

// SwagVersion is the swag command `gortisan swag` runs; it matches the swag
// module generated projects require.
const SwagVersion = "v1.16.6"

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
	// Now stamps the first migration; time.Now when zero.
	Now time.Time
}

// ToolPackage is what the project's ./gortisan scripts `go run`. Moving
// gortisan to another repository changes this constant and go.mod only.
const ToolPackage = "github.com/alinemone/gortisan/cmd/gortisan"

type projectData struct {
	Name        string
	Module      string
	Title       string
	Postgres    bool
	Redis       bool
	Migrations  bool
	Swagger     bool
	BasePath    string
	Tool        string // ToolPackage@version
	SwagVersion string
}

func newProjectData(name, module string, with map[string]bool, version string) projectData {
	return projectData{
		Name:        name,
		Module:      module,
		Title:       title(name),
		Postgres:    with["postgres"],
		Redis:       with["redis"],
		Migrations:  with["migrations"],
		Swagger:     with["swagger"],
		BasePath:    DefaultBasePath,
		Tool:        ToolPackage + "@" + version,
		SwagVersion: SwagVersion,
	}
}

// baseOutputs are the files of every project.
var baseOutputs = []output{
	{"go.mod", "create/go.mod.tmpl"},
	{"main.go", "create/main.go.tmpl"},
	{"cmd/root.go", "create/cmd_root.go.tmpl"},
	{"cmd/serve.go", "create/cmd_serve.go.tmpl"},
	{"internal/app/serve.go", "create/app_serve.go.tmpl"},
	{"internal/config/config.go", "create/config.go.tmpl"},
	{"internal/server/server.go", "create/server.go.tmpl"},
	{"internal/server/server_test.go", "create/server_test.go.tmpl"},
	{"internal/server/health.go", "create/server_health.go.tmpl"},
	{"internal/server/response/problem.go", "create/response_problem.go.tmpl"},
	{validatePath, "create/validate.go.tmpl"},
	{"internal/server/validate/validate_test.go", "create/validate_test.go.tmpl"},
	{"internal/platform/trans/trans.go", "create/trans.go.tmpl"},
	{"internal/platform/trans/trans_test.go", "create/trans_test.go.tmpl"},
	{langPath, "create/lang_fa.json.tmpl"},
	{"tests/arch_test.go", "create/arch_test.go.tmpl"},
	{"deploy/Dockerfile", "create/deploy_Dockerfile.tmpl"},
	{"deploy/entrypoint.sh", "create/entrypoint.sh.tmpl"},
	{"AGENTS.md", "create/AGENTS.md.tmpl"},
	{"CLAUDE.md", "create/CLAUDE.md.tmpl"},
	{".agents/skills/gortisan/SKILL.md", "create/skill.md.tmpl"},
	{"README.md", "create/README.md.tmpl"},
	{"Makefile", "create/Makefile.tmpl"},
	{".env.example", "create/env.example.tmpl"},
	{".gitignore", "create/gitignore.tmpl"},
	{".gitattributes", "create/gitattributes.tmpl"},
	{".dockerignore", "create/dockerignore.tmpl"},
	{".aiignore", "create/aiignore.tmpl"},
	{"gortisan", "create/gortisan.sh.tmpl"},
	{"gortisan.bat", "create/gortisan.bat.tmpl"},
	{"gortisan.ps1", "create/gortisan.ps1.tmpl"},
}

// executables are written with the executable bit.
var executables = []string{"gortisan", "deploy/entrypoint.sh"}

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

	version := toolVersion(opts.Version)
	data := newProjectData(name, module, with, version)
	outputs := slices.Clone(baseOutputs)
	for _, c := range Components {
		if with[c] {
			outputs = append(outputs, componentOutputs(c)...)
		}
	}
	if err := renderAll(root, outputs, data, false, report); err != nil {
		return report, err
	}
	if with["migrations"] {
		// The embedded directory must not be empty, or the project does not build.
		first := migrationsDir + "/" + migrationStamp(opts.Now) + "_init.sql"
		if err := renderAll(root, []output{{first, "create/migration_init.sql.tmpl"}}, data, false, report); err != nil {
			return report, err
		}
	}
	for _, rel := range executables {
		if err := os.Chmod(filepath.Join(root, filepath.FromSlash(rel)), 0o755); err != nil {
			return report, err
		}
	}
	settings := &Settings{Layout: Layout, Version: version, BasePath: data.BasePath}
	for _, c := range Components {
		if with[c] {
			settings.With = append(settings.With, c)
		}
	}
	if err := writeSettings(root, settings); err != nil {
		return report, err
	}
	report.Created = append(report.Created, SettingsFile)

	if opts.NoTidy {
		report.next("cd %s && go mod tidy", name)
	} else if out, err := goModTidy(root); err != nil {
		report.next("go mod tidy failed, run it yourself in %s:\n%s", name, strings.TrimSpace(out))
	}
	report.next("cd %s && cp .env.example .env", name)
	report.next("./gortisan run   # or: go run . serve — serves on PORT (8000), features under %s", data.BasePath)
	report.next("./gortisan make:feature user   # gortisan.bat / gortisan.ps1 on Windows")
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
	case "all":
		for _, c := range Components {
			with[c] = true
		}
	case "none":
	default:
		for _, part := range strings.Split(value, ",") {
			part = strings.TrimSpace(part)
			if !slices.Contains(Components, part) {
				return nil, fmt.Errorf("unknown component %q in --with; choose from %s, all or none", part, strings.Join(Components, ", "))
			}
			with[part] = true
		}
	}
	if with["migrations"] && !with["postgres"] {
		return nil, errors.New("--with migrations needs postgres too")
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
