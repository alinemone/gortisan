package generator

import (
	"bufio"
	"errors"
	"fmt"
	"go/token"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Project is the Go module `make` writes into.
type Project struct {
	Root   string
	Module string
}

// FindProject walks up from dir to the nearest go.mod.
func FindProject(dir string) (*Project, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for {
		gomod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(gomod); err == nil {
			module, err := readModule(gomod)
			if err != nil {
				return nil, err
			}
			return &Project{Root: dir, Module: module}, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, errors.New("no go.mod in this directory or any parent; run gortisan make inside a project")
		}
		dir = parent
	}
}

func readModule(gomod string) (string, error) {
	file, err := os.Open(gomod)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s has no module line", gomod)
}

func (p *Project) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(p.Root, filepath.FromSlash(rel)))
	return err == nil
}

func (p *Project) require(path, why string) error {
	if !p.exists(path) {
		return fmt.Errorf("%s is missing; %s", path, why)
	}
	return nil
}

// Paths of the platform pieces make depends on.
const (
	ormPath      = "internal/platform/database/orm/orm.go"
	redisPath    = "internal/platform/cache/redis.go"
	validatePath = "internal/platform/validate/validate.go"
)

// MakeOptions configure Make.
type MakeOptions struct {
	Force      bool
	Table      string
	Fields     string
	SoftDelete bool
	// Type is the enum base type or the cached value type.
	Type string
	// Members are the enum values, e.g. "pending,paid=5".
	Members string
	// Now stamps migration file names; time.Now when zero.
	Now time.Time
}

// MakeKinds are what `gortisan make` can write.
var MakeKinds = []string{
	"model", "service", "handler", "request", "response", "validator", "enum",
	"cache", "middleware", "client", "migration", "seeder",
}

// Make writes one building block of kind named name into p.
//
// A project has four layers under internal/:
//
//	domain/<subject>/      the business: model, errors, store, repository, service
//	api/<version>/<resources>/  HTTP (named like the URL, e.g. users): handler, request, response
//	platform/              infrastructure
//
// Subjects may be grouped: "sales/order" is internal/domain/sales/order.
func Make(p *Project, kind, name string, opts MakeOptions) (*Report, error) {
	report := &Report{}
	var err error
	switch kind {
	case "model":
		err = makeModel(p, name, opts, report)
	case "service":
		err = makeService(p, name, opts, report)
	case "handler":
		err = makeHandler(p, name, report)
	case "request":
		err = makeRequest(p, name, report)
	case "response":
		err = makeResponse(p, name, report)
	case "validator":
		err = makeValidator(p, name, report)
	case "enum":
		err = makeEnum(p, name, opts, report)
	case "cache":
		err = makeCache(p, name, opts, report)
	case "middleware":
		err = makeMiddleware(p, name, opts, report)
	case "client", "sdk":
		err = makeClient(p, name, opts, report)
	case "migration":
		err = makeMigration(p, name, opts, report)
	case "seeder":
		err = makeSeeder(p, name, opts, report)
	default:
		err = fmt.Errorf("unknown kind %q; choose from %s", kind, strings.Join(MakeKinds, ", "))
	}
	return report, err
}

// feature is one subject of the domain, e.g. internal/domain/sales/order.
type feature struct {
	Path   string // sales/order (directories, as written)
	Name   string // order (snake, last segment)
	Pkg    string // order
	Type   string // Order
	Parent string // sales (empty when not grouped)
}

func (f feature) Dir() string { return "internal/domain/" + f.Path }

// Label is "sales.order", used to name workers.
func (f feature) Label() string { return strings.ReplaceAll(f.Path, "/", ".") }

// parseFeature reads "user" or "sales/order_item".
func parseFeature(path string) (feature, error) {
	path = strings.Trim(filepath.ToSlash(path), "/")
	if path == "" {
		return feature{}, errors.New("a subject name is required, e.g. user or sales/order")
	}
	var dirs []string
	var last string
	for _, segment := range strings.Split(path, "/") {
		n, err := identifier("subject", segment)
		if err != nil {
			return feature{}, err
		}
		last = n
		pkg := strings.ReplaceAll(n, "_", "")
		if token.IsKeyword(pkg) {
			return feature{}, fmt.Errorf("%q is a Go keyword and cannot be a package name", pkg)
		}
		dirs = append(dirs, pkg)
	}
	return feature{
		Path:   strings.Join(dirs, "/"),
		Name:   last,
		Pkg:    dirs[len(dirs)-1],
		Type:   pascal(last),
		Parent: strings.Join(dirs[:len(dirs)-1], "/"),
	}, nil
}

// splitItem reads "<subject>/<item>", e.g. user/status or sales/order/status.
func splitItem(kind, name string) (feature, string, error) {
	name = strings.Trim(filepath.ToSlash(name), "/")
	i := strings.LastIndex(name, "/")
	if i < 0 {
		return feature{}, "", fmt.Errorf("%s name must be <subject>/<name>, e.g. user/status; got %q", kind, name)
	}
	f, err := parseFeature(name[:i])
	if err != nil {
		return feature{}, "", err
	}
	item, err := identifier(kind, name[i+1:])
	return f, item, err
}

var versionRE = regexp.MustCompile(`^v[0-9]+$`)

// splitAPI reads "<version>/<subject>/<item>", e.g. v1/user/list or
// v2/sales/order/create.
func splitAPI(kind, name string) (string, feature, string, error) {
	name = strings.Trim(filepath.ToSlash(name), "/")
	version, rest, _ := strings.Cut(name, "/")
	if !versionRE.MatchString(version) {
		return "", feature{}, "", fmt.Errorf("%s name must start with the API version: <version>/<subject>/<name>, e.g. v1/user/list; got %q", kind, name)
	}
	f, item, err := splitItem(kind, rest)
	return version, f, item, err
}

// data is what the make templates are rendered with.
type data struct {
	Module string
	Pkg    string // the package being written
	Name   string
	Words  string // "order item"
	Type   string
	Var    string

	Table      string
	Fields     []Field
	SoftDelete bool
	JSON       bool

	HasStore   bool
	HasService bool

	DomainPkg    string
	DomainImport string
	Version      string
	Action       string
	Route        string
	Method       string
	Path         string

	Message string
	Base    string
	Members []EnumMember
	Plural  string
	Value   string
	Prefix  string
	Func    string
	Feature string

	Package   string
	Title     string
	EnvPrefix string
}

func (f feature) data(p *Project) data {
	return data{
		Module:       p.Module,
		Pkg:          f.Pkg,
		Name:         f.Name,
		Words:        strings.ReplaceAll(f.Name, "_", " "),
		Type:         f.Type,
		Var:          camel(f.Type),
		DomainPkg:    f.Pkg,
		DomainImport: p.Module + "/" + f.Dir(),
		Feature:      f.Label(),
	}
}

// ensure writes the outputs that do not exist yet, without reporting the ones
// that do: shared files several commands need.
func ensure(p *Project, outputs []output, d any, report *Report) error {
	for _, out := range outputs {
		if p.exists(out.path) {
			continue
		}
		if err := renderAll(p.Root, []output{out}, d, false, report); err != nil {
			return err
		}
	}
	return nil
}

// ensureValidate writes internal/platform/validate when the project has none.
func ensureValidate(p *Project, report *Report) error {
	return ensure(p, []output{
		{validatePath, "create/validate.go.tmpl"},
		{"internal/platform/validate/validate_test.go", "create/validate_test.go.tmpl"},
	}, projectData{Module: p.Module}, report)
}

func makeModel(p *Project, name string, opts MakeOptions, report *Report) error {
	if err := p.require(ormPath, "make:model needs a project created with postgres"); err != nil {
		return err
	}
	f, err := parseFeature(name)
	if err != nil {
		return err
	}
	d := f.data(p)
	d.Table = opts.Table
	if d.Table == "" {
		d.Table = plural(f.Name)
	}
	if d.Table, err = identifier("table", d.Table); err != nil {
		return err
	}
	if d.Fields, err = parseFields(opts.Fields); err != nil {
		return err
	}
	d.SoftDelete = opts.SoftDelete
	for _, field := range d.Fields {
		d.JSON = d.JSON || field.JSON
	}
	dir := f.Dir()
	stamp := freshStamp(p, opts.Now)
	if err := renderAll(p.Root, []output{
		{dir + "/" + f.Pkg + ".go", "make/model.go.tmpl"},
		{dir + "/errors.go", "make/errors.go.tmpl"},
		{dir + "/store.go", "make/store.go.tmpl"},
		{dir + "/repository.go", "make/repository.go.tmpl"},
		{"migrations/" + stamp + "_create_" + d.Table + "_table.up.sql", "make/create_table.up.sql.tmpl"},
		{"migrations/" + stamp + "_create_" + d.Table + "_table.down.sql", "make/create_table.down.sql.tmpl"},
	}, d, opts.Force, report); err != nil {
		return err
	}
	generated, err := Generate(p, GenerateOptions{})
	report.merge(generated)
	if err != nil {
		return err
	}
	report.next("create the table: ./gortisan migrate   (migrations/%s_create_%s_table.up.sql)", stamp, d.Table)
	report.next("then: ./gortisan make:service %s", f.Path)
	return nil
}

func makeService(p *Project, name string, opts MakeOptions, report *Report) error {
	f, err := parseFeature(strings.TrimSuffix(snake(name), "_service"))
	if err != nil {
		return err
	}
	d := f.data(p)
	d.HasStore = p.exists(f.Dir() + "/store.go")
	errorsFile := []output{}
	if !p.exists(f.Dir() + "/errors.go") {
		errorsFile = append(errorsFile, output{f.Dir() + "/errors.go", "make/errors.go.tmpl"})
	}
	if err := renderAll(p.Root, append(errorsFile, output{f.Dir() + "/service.go", "make/service.go.tmpl"}), d, opts.Force, report); err != nil {
		return err
	}
	if d.HasStore {
		report.next("build it in cmd/api/main.go: %s.NewService(%s.NewRepository(db))", f.Pkg, f.Pkg)
	} else {
		report.next("build it in cmd/api/main.go: %s.NewService()", f.Pkg)
	}
	return nil
}

// apiPackage is where version's HTTP code for f lives, named like the URL:
// internal/api/v1/users for /v1/users, internal/api/v1/sales/orders. The plural
// also keeps it apart from the domain package (user), so neither needs an
// import alias.
func apiPackage(version string, f feature) (dir, pkg string) {
	pkg = strings.ReplaceAll(plural(f.Name), "_", "")
	dir = "internal/api/" + version
	if f.Parent != "" {
		dir += "/" + f.Parent
	}
	return dir + "/" + pkg, pkg
}

// ensureVersion writes internal/api/<version>/routes.go for a new version.
func ensureVersion(p *Project, version string, report *Report) error {
	routes := "internal/api/" + version + "/routes.go"
	if p.exists(routes) {
		return nil
	}
	if err := renderAll(p.Root, []output{{routes, "create/version_routes.go.tmpl"}}, projectData{Module: p.Module, Version: version}, false, report); err != nil {
		return err
	}
	report.next("mount the new version in internal/api/router.go: import %q, add `%s %s.Services` to Routes and call %s.Register(e.Group(\"/%s\"), r.%s); then set it in cmd/api/main.go",
		p.Module+"/internal/api/"+version, strings.ToUpper(version), version, version, version, strings.ToUpper(version))
	return nil
}

func apiData(p *Project, version string, f feature, item string) data {
	d := f.data(p)
	d.Version = version
	_, d.Pkg = apiPackage(version, f)
	d.Action = pascal(item)
	d.Route = "/" + plural(strings.ReplaceAll(f.Name, "_", "-"))
	d.Method, d.Path = routeFor(d.Action, d.Route)
	d.Var = camel(f.Name) + "Handler"
	d.HasService = p.exists(f.Dir() + "/service.go")
	return d
}

func makeHandler(p *Project, name string, report *Report) error {
	version, f, action, err := splitAPI("handler", name)
	if err != nil {
		return err
	}
	if err := ensureValidate(p, report); err != nil {
		return err
	}
	if err := ensureVersion(p, version, report); err != nil {
		return err
	}
	dir, pkg := apiPackage(version, f)
	d := apiData(p, version, f, action)

	handlerFile := dir + "/handler.go"
	isNew := !p.exists(handlerFile)
	if isNew {
		if err := renderAll(p.Root, []output{{handlerFile, "make/handler.go.tmpl"}}, d, false, report); err != nil {
			return err
		}
	}
	for _, step := range []struct{ file, tmpl string }{
		{handlerFile, "make/handler_action.go.tmpl"},
		{dir + "/request.go", "make/request.go.tmpl"},
		{dir + "/response.go", "make/response.go.tmpl"},
	} {
		if err := writeOrAppend(p, step.file, step.tmpl, d, report); err != nil {
			return err
		}
	}

	routes := "internal/api/" + version + "/routes.go"
	if isNew {
		if d.HasService {
			report.next("in %s: add `%s *%s.Service` to Services, then `%s := %s.NewHandler(s.%s)` in Register; set %s in cmd/api/main.go",
				routes, f.Type, f.Pkg, d.Var, pkg, f.Type, f.Type)
		} else {
			report.next("in %s: `%s := %s.NewHandler()` in Register (run make:service %s to give it the domain service)",
				routes, d.Var, pkg, f.Path)
		}
	}
	report.next("register the route in %s, e.g. g.%s(%q, %s.%s)", routes, d.Method, d.Path, d.Var, d.Action)
	return nil
}

// routeFor guesses the REST method and path of an action from its name:
// Create is POST /users, Get is GET /users/:id, and so on. Anything else is a
// GET on the collection; the hint is only a suggestion.
func routeFor(action, collection string) (string, string) {
	switch action {
	case "List", "Index", "Search":
		return "GET", collection
	case "Get", "Show", "Find":
		return "GET", collection + "/:id"
	case "Create", "Store":
		return "POST", collection
	case "Update", "Edit":
		return "PUT", collection + "/:id"
	case "Patch":
		return "PATCH", collection + "/:id"
	case "Delete", "Destroy", "Remove":
		return "DELETE", collection + "/:id"
	}
	return "GET", collection + "/" + strings.ReplaceAll(snake(action), "_", "-")
}

func makeRequest(p *Project, name string, report *Report) error {
	version, f, item, err := splitAPI("request", name)
	if err != nil {
		return err
	}
	if err := ensureValidate(p, report); err != nil {
		return err
	}
	dir, _ := apiPackage(version, f)
	d := apiData(p, version, f, strings.TrimSuffix(item, "_request"))
	return writeOrAppend(p, dir+"/request.go", "make/request.go.tmpl", d, report)
}

func makeResponse(p *Project, name string, report *Report) error {
	version, f, item, err := splitAPI("response", name)
	if err != nil {
		return err
	}
	dir, _ := apiPackage(version, f)
	d := apiData(p, version, f, strings.TrimSuffix(item, "_response"))
	return writeOrAppend(p, dir+"/response.go", "make/response.go.tmpl", d, report)
}

func makeValidator(p *Project, name string, report *Report) error {
	f, item, err := splitItem("validator", name)
	if err != nil {
		return err
	}
	if err := ensureValidate(p, report); err != nil {
		return err
	}
	d := f.data(p)
	d.Type = pascal(item)
	d.Message = "is invalid"
	if err := writeOrAppend(p, f.Dir()+"/validator.go", "make/validator.go.tmpl", d, report); err != nil {
		return err
	}
	report.next("use it in a request's Validate: errs.Check(ctx, \"field\", %s.New%s(...))", f.Pkg, d.Type)
	return nil
}

// EnumMember is one value of a generated enum.
type EnumMember struct {
	Name  string // Paid
	Label string // paid
	Value string // 2, or "paid" for string enums
}

var enumBases = map[string]bool{
	"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true,
}

func makeEnum(p *Project, name string, opts MakeOptions, report *Report) error {
	f, item, err := splitItem("enum", name)
	if err != nil {
		return err
	}
	base := opts.Type
	if base == "" {
		base = "int64"
	}
	if !enumBases[base] {
		return fmt.Errorf("enum --type must be string or an integer type such as int64; got %q", base)
	}
	members, err := parseMembers(opts.Members, base == "string")
	if err != nil {
		return err
	}
	d := f.data(p)
	d.Type = pascal(item)
	d.Plural = pascal(plural(item))
	d.Base = base
	d.Members = members
	return renderAll(p.Root, []output{{f.Dir() + "/" + item + ".go", "make/enum.go.tmpl"}}, d, opts.Force, report)
}

// parseMembers reads "pending,paid=5,sent": integers count up from 1 or from
// the previous value; string values default to the member's name.
func parseMembers(spec string, isString bool) ([]EnumMember, error) {
	var members []EnumMember
	seen := map[string]bool{}
	next := int64(1)
	for _, part := range strings.Split(spec, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		rawName, rawValue, hasValue := strings.Cut(part, "=")
		label, err := identifier("enum member", rawName)
		if err != nil {
			return nil, err
		}
		if seen[label] {
			return nil, fmt.Errorf("enum member %q is listed twice", label)
		}
		seen[label] = true
		member := EnumMember{Name: pascal(label), Label: label}
		switch {
		case isString && hasValue:
			member.Value = strconv.Quote(strings.TrimSpace(rawValue))
		case isString:
			member.Value = strconv.Quote(label)
		default:
			if hasValue {
				v, err := strconv.ParseInt(strings.TrimSpace(rawValue), 10, 64)
				if err != nil {
					return nil, fmt.Errorf("enum member %q: %q is not an integer", label, rawValue)
				}
				next = v
			}
			member.Value = strconv.FormatInt(next, 10)
			next++
		}
		members = append(members, member)
	}
	if len(members) == 0 {
		return nil, errors.New(`enum needs --members, e.g. --members "pending,paid,sent" or "user=3021,vendor"`)
	}
	return members, nil
}

func makeCache(p *Project, name string, opts MakeOptions, report *Report) error {
	if err := p.require(redisPath, "make:cache needs a project created with redis"); err != nil {
		return err
	}
	f, item, err := splitItem("cache", name)
	if err != nil {
		return err
	}
	item = strings.TrimSuffix(item, "_cache")
	d := f.data(p)
	d.Type = pascal(item) + "Cache"
	d.Prefix = f.Label() + ":" + item
	d.Value = opts.Type
	if d.Value == "" {
		d.Value = "string"
	}
	if err := writeOrAppend(p, f.Dir()+"/cache.go", "make/cache.go.tmpl", d, report); err != nil {
		return err
	}
	report.next("build it in cmd/api/main.go: %s.New%s(rdb, time.Hour, load)", f.Pkg, d.Type)
	return nil
}

func makeMiddleware(p *Project, name string, opts MakeOptions, report *Report) error {
	n, err := identifier("middleware", name)
	if err != nil {
		return err
	}
	d := data{Module: p.Module, Func: pascal(n)}
	if err := renderAll(p.Root, []output{{"internal/api/middleware/" + n + ".go", "make/middleware.go.tmpl"}}, d, opts.Force, report); err != nil {
		return err
	}
	report.next("use it in internal/api/router.go (every route) or a version's routes.go, e.g. g.Use(middleware.%s())", d.Func)
	return nil
}

func makeClient(p *Project, name string, opts MakeOptions, report *Report) error {
	n, err := identifier("client", name)
	if err != nil {
		return err
	}
	pkg := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, n)
	if p.exists("internal/platform/" + pkg) {
		return fmt.Errorf("internal/platform/%s already exists; pick another name", pkg)
	}
	d := data{Package: pkg, Title: title(n), EnvPrefix: strings.ToUpper(n)}
	if err := renderAll(p.Root, []output{{"internal/platform/" + pkg + "/client.go", "make/client.go.tmpl"}}, d, opts.Force, report); err != nil {
		return err
	}
	report.next("add %s_BASE_URL to internal/platform/config and build it in cmd/api/main.go: %s.New(%s.Config{BaseURL: ...})", d.EnvPrefix, pkg, pkg)
	return nil
}

func makeMigration(p *Project, name string, opts MakeOptions, report *Report) error {
	n, err := identifier("migration", name)
	if err != nil {
		return err
	}
	stamp := freshStamp(p, opts.Now)
	return renderAll(p.Root, []output{
		{"migrations/" + stamp + "_" + n + ".up.sql", "make/migration.up.sql.tmpl"},
		{"migrations/" + stamp + "_" + n + ".down.sql", "make/migration.down.sql.tmpl"},
	}, data{Name: n}, opts.Force, report)
}

func makeSeeder(p *Project, name string, opts MakeOptions, report *Report) error {
	if err := p.require(ormPath, "make:seeder needs a project created with postgres"); err != nil {
		return err
	}
	n, err := identifier("seeder", strings.TrimSuffix(snake(name), "_seeder"))
	if err != nil {
		return err
	}
	d := data{Module: p.Module, Name: n, Type: pascal(n)}
	if err := renderAll(p.Root, []output{{"internal/seed/" + n + ".go", "make/seeder.go.tmpl"}}, d, opts.Force, report); err != nil {
		return err
	}
	if p.exists("cmd/seed/main.go") {
		report.next("list it in cmd/seed/main.go: {%q, seed.%s},", n, d.Type)
	} else if err := renderAll(p.Root, []output{{"cmd/seed/main.go", "make/seed_main.go.tmpl"}}, d, false, report); err != nil {
		return err
	}
	report.next("run it: ./gortisan db:seed   (or ./gortisan db:seed %s)", n)
	return nil
}

func migrationStamp(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return now.UTC().Format("20060102150405")
}

// freshStamp is migrationStamp moved forward a second at a time until no
// migration uses it: two make commands in the same second must not share a
// version, or the migrator refuses to run.
func freshStamp(p *Project, now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	for {
		stamp := migrationStamp(now)
		if matches, _ := filepath.Glob(filepath.Join(p.Root, "migrations", stamp+"_*")); len(matches) == 0 {
			return stamp
		}
		now = now.Add(time.Second)
	}
}

func (r *Report) merge(other *Report) {
	if other == nil {
		return
	}
	r.Created = append(r.Created, other.Created...)
	r.Updated = append(r.Updated, other.Updated...)
	r.Removed = append(r.Removed, other.Removed...)
	r.Skipped = append(r.Skipped, other.Skipped...)
	r.Next = append(r.Next, other.Next...)
}
