package generator

import (
	"bytes"
	"encoding/json"
	"errors"
	"fmt"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
	"time"
	"unicode"
)

// Paths of the shared pieces make writes into or depends on.
const (
	appServe      = "internal/app/serve.go"
	ormDir        = "internal/platform/orm"
	validatePath  = "internal/server/validate/validate.go"
	validatorDir  = "internal/server/validator"
	middlewareDir = "internal/server/middleware"
	langPath      = "internal/platform/trans/lang/fa.json"
	migrationsDir = "internal/migrations"
)

// sharedDirs are the directories of internal/ that are not features; a
// feature cannot take one of their names.
var sharedDirs = []string{"app", "config", "server", "platform", "clients", "external", "migrations"}

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
	"feature", "handler", "migration", "enum", "cache", "validator",
	"middleware", "client", "external", "test",
}

// Make writes one building block of kind named name into p.
//
// A feature is one package under internal/ — internal/user, or grouped,
// internal/sales/order — with everything of it: entity and errors, service,
// storage, HTTP handlers, DTOs and routes. Shared code lives in
// internal/server, internal/platform, internal/clients and internal/external.
func Make(p *Project, kind, name string, opts MakeOptions) (*Report, error) {
	report := &Report{}
	if err := p.Check(); err != nil {
		return report, err
	}
	var err error
	switch kind {
	case "feature":
		err = makeFeature(p, name, opts, report)
	case "handler":
		err = makeHandler(p, name, report)
	case "migration":
		err = makeMigration(p, name, opts, report)
	case "enum":
		err = makeEnum(p, name, opts, report)
	case "cache":
		err = makeCache(p, name, opts, report)
	case "validator":
		err = makeValidator(p, name, report)
	case "middleware":
		err = makeMiddleware(p, name, opts, report)
	case "client":
		err = makeClient(p, name, "clients", "client", "make/client.go.tmpl", opts, report)
	case "external":
		err = makeClient(p, name, "external", "", "make/external.go.tmpl", opts, report)
	case "test":
		err = makeTest(p, name, opts, report)
	default:
		err = fmt.Errorf("unknown kind %q; choose from %s", kind, strings.Join(MakeKinds, ", "))
	}
	return report, err
}

// feature is one feature package, e.g. internal/sales/order.
type feature struct {
	Path   string // sales/order (directories, as written)
	Name   string // order (snake, last segment)
	Pkg    string // order
	Type   string // Order
	Parent string // sales (empty when not grouped)
}

func (f feature) Dir() string { return "internal/" + f.Path }

// Label is "sales.order", used to prefix cache keys.
func (f feature) Label() string { return strings.ReplaceAll(f.Path, "/", ".") }

// Route is the default path of the feature under BASE_PATH: /users, or
// /sales/orders for a grouped feature.
func (f feature) Route() string {
	route := "/" + plural(strings.ReplaceAll(f.Name, "_", "-"))
	if f.Parent != "" {
		route = "/" + f.Parent + route
	}
	return route
}

// parseFeature reads "user" or "sales/order_item".
func parseFeature(path string) (feature, error) {
	path = strings.Trim(filepath.ToSlash(path), "/")
	if path == "" {
		return feature{}, errors.New("a feature name is required, e.g. user or sales/order")
	}
	var dirs []string
	var last string
	for _, segment := range strings.Split(path, "/") {
		n, err := identifier("feature", segment)
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
	if slices.Contains(sharedDirs, dirs[0]) {
		return feature{}, fmt.Errorf("%q is a shared directory of internal/ (%s), not a feature; pick another name", dirs[0], strings.Join(sharedDirs, ", "))
	}
	return feature{
		Path:   strings.Join(dirs, "/"),
		Name:   last,
		Pkg:    dirs[len(dirs)-1],
		Type:   pascal(last),
		Parent: strings.Join(dirs[:len(dirs)-1], "/"),
	}, nil
}

// splitItem reads "<feature>/<item>", e.g. user/status or sales/order/status.
func splitItem(kind, name string) (feature, string, error) {
	name = strings.Trim(filepath.ToSlash(name), "/")
	i := strings.LastIndex(name, "/")
	if i < 0 {
		return feature{}, "", fmt.Errorf("%s name must be <feature>/<name>, e.g. user/list; got %q", kind, name)
	}
	f, err := parseFeature(name[:i])
	if err != nil {
		return feature{}, "", err
	}
	item, err := identifier(kind, name[i+1:])
	return f, item, err
}

// data is what the make templates are rendered with.
type data struct {
	Module  string
	Pkg     string // the package being written
	Name    string
	Words   string // "order item"
	Type    string
	Var     string
	Feature string // sales/order

	Table      string
	Fields     []Field
	SoftDelete bool
	JSON       bool
	Postgres   bool
	Swagger    bool

	Route       string // the feature's group path, /users
	Action      string
	Method      string
	MethodLower string
	Path        string // the action's path inside the group, /:id
	SwagPath    string // /users/{id}
	Tag         string
	HasID       bool
	TestURL     string

	Base    string
	Members []EnumMember
	Plural  string
	Value   string
	Prefix  string
	Func    string

	Package   string
	Title     string
	EnvPrefix string
}

func (f feature) data(p *Project) data {
	return data{
		Module:   p.Module,
		Pkg:      f.Pkg,
		Name:     f.Name,
		Words:    strings.ReplaceAll(f.Name, "_", " "),
		Type:     f.Type,
		Var:      camel(f.Type),
		Feature:  f.Path,
		Route:    f.Route(),
		Postgres: p.Has("postgres"),
		Swagger:  p.Has("swagger"),
	}
}

func makeFeature(p *Project, name string, opts MakeOptions, report *Report) error {
	f, err := parseFeature(name)
	if err != nil {
		return err
	}
	dir := f.Dir()
	if p.exists(dir+"/service.go") && !opts.Force {
		return fmt.Errorf("%s already exists; add to it with make:handler %s/<action>, or pass --force to overwrite its files", dir, f.Path)
	}
	d := f.data(p)
	d.Table = opts.Table
	if d.Table == "" {
		d.Table = strings.ReplaceAll(f.Path, "/", "_")
		d.Table = d.Table[:len(d.Table)-len(f.Name)] + plural(f.Name)
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

	outputs := []output{
		{dir + "/" + f.Pkg + ".go", "make/feature_entity.go.tmpl"},
		{dir + "/service.go", "make/feature_service.go.tmpl"},
		{dir + "/handler.go", "make/feature_handler.go.tmpl"},
		{dir + "/dto.go", "make/feature_dto.go.tmpl"},
		{dir + "/routes.go", "make/feature_routes.go.tmpl"},
		{dir + "/service_test.go", "make/feature_service_test.go.tmpl"},
	}
	if d.Postgres {
		outputs = append(outputs, output{dir + "/postgres.go", "make/feature_postgres.go.tmpl"})
	}
	if err := renderAll(p.Root, outputs, d, opts.Force, report); err != nil {
		return err
	}
	if d.Postgres && p.Has("migrations") {
		file := migrationsDir + "/" + freshStamp(p, opts.Now) + "_create_" + d.Table + ".sql"
		if err := renderAll(p.Root, []output{{file, "make/create_table.sql.tmpl"}}, d, false, report); err != nil {
			return err
		}
		report.next("create the table: go run . migrate up   (%s)", file)
	} else if d.Postgres {
		report.next("the %s table is yours to create; postgres.go reads and writes it", d.Table)
	}

	// Wire it into internal/app: the service, its storage, its routes.
	service := d.Var + "Service"
	repo := "nil /* a Repository */"
	if d.Postgres {
		repo = f.Pkg + ".NewPostgres(db)"
	}
	lines := []string{
		fmt.Sprintf("%s := %s.NewService(%s)", service, f.Pkg, repo),
		fmt.Sprintf("%s.RegisterRoutes(api, %s.NewHandler(%s))", f.Pkg, f.Pkg, service),
	}
	imports := []string{strconv.Quote(p.Module + "/" + dir)}
	if _, err := addToFunc(p, appServe, "features", lines, imports, nil, true, report); err != nil {
		if !errors.Is(err, errNoFunc) && !errors.Is(err, os.ErrNotExist) {
			return err
		}
		report.next("build it in %s: %s", appServe, strings.Join(lines, "; "))
	}
	if !d.Postgres {
		report.next("give %s a Repository in %s (it is nil until then)", service, appServe)
	}
	report.next("add an endpoint: ./gortisan make:handler %s/list", f.Path)
	return nil
}

func makeHandler(p *Project, name string, report *Report) error {
	f, action, err := splitItem("handler", name)
	if err != nil {
		return err
	}
	dir := f.Dir()
	if err := p.require(dir+"/handler.go", "run make:feature "+f.Path+" first"); err != nil {
		return err
	}
	d := handlerData(p, f, action)
	for _, step := range []struct{ file, tmpl string }{
		{dir + "/handler.go", "make/handler_action.go.tmpl"},
		{dir + "/dto.go", "make/dto_request.go.tmpl"},
		{dir + "/dto.go", "make/dto_response.go.tmpl"},
	} {
		if err := writeOrAppend(p, step.file, step.tmpl, d, report); err != nil {
			return err
		}
	}

	routes := dir + "/routes.go"
	group, _, ok := routeGroup(p, routes)
	line := fmt.Sprintf("g.%s(%q, h.%s)", d.Method, d.Path, d.Action)
	if ok {
		line = fmt.Sprintf("%s.%s(%q, h.%s)", group, d.Method, d.Path, d.Action)
		if _, err := addToFunc(p, routes, "RegisterRoutes", []string{line}, nil, []string{"_ = " + group}, false, report); err != nil {
			return err
		}
	} else {
		report.next("register it in %s: %s", routes, line)
	}
	if d.Swagger {
		report.next("document it in the swag comments of %s, then ./gortisan swag", d.Action)
	}
	return nil
}

// handlerData describes an action of f: its method and path come from its
// name (routeFor), its group path from routes.go.
func handlerData(p *Project, f feature, action string) data {
	d := f.data(p)
	d.Action = pascal(action)
	if _, route, ok := routeGroup(p, f.Dir()+"/routes.go"); ok {
		d.Route = route
	}
	d.Method, d.Path = routeFor(d.Action)
	d.MethodLower = strings.ToLower(d.Method)
	d.HasID = strings.Contains(d.Path, ":id")
	d.SwagPath = strings.ReplaceAll(d.Route+d.Path, ":id", "{id}")
	d.Tag = strings.TrimPrefix(d.Route, "/")
	d.TestURL = strings.ReplaceAll(d.Route+d.Path, ":id", "1")
	d.Var = camel(f.Type) + d.Action
	return d
}

// routeFor guesses the REST method and path of an action from its name:
// Create is POST "", Get is GET /:id, and so on. Anything else is a GET on
// /<action>; the route is only a starting point.
func routeFor(action string) (string, string) {
	switch action {
	case "List", "Index", "Search":
		return "GET", ""
	case "Get", "Show", "Find":
		return "GET", "/:id"
	case "Create", "Store":
		return "POST", ""
	case "Update", "Edit":
		return "PUT", "/:id"
	case "Patch":
		return "PATCH", "/:id"
	case "Delete", "Destroy", "Remove":
		return "DELETE", "/:id"
	}
	return "GET", "/" + strings.ReplaceAll(snake(action), "_", "-")
}

func makeMigration(p *Project, name string, opts MakeOptions, report *Report) error {
	if err := p.need("migrations", "make:migration"); err != nil {
		return err
	}
	n, err := identifier("migration", name)
	if err != nil {
		return err
	}
	file := migrationsDir + "/" + freshStamp(p, opts.Now) + "_" + n + ".sql"
	if err := renderAll(p.Root, []output{{file, "make/migration.sql.tmpl"}}, data{Name: n}, opts.Force, report); err != nil {
		return err
	}
	report.next("write the change in %s, then: go run . migrate up", file)
	return nil
}

// migrationStamp is the version of a migration: the UTC time to the second.
func migrationStamp(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return now.UTC().Format("20060102150405")
}

// freshStamp is migrationStamp moved forward a second at a time until no
// migration uses it: two make commands in the same second must not share a
// version.
func freshStamp(p *Project, now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	for {
		stamp := migrationStamp(now)
		if matches, _ := filepath.Glob(filepath.Join(p.Root, filepath.FromSlash(migrationsDir), stamp+"_*")); len(matches) == 0 {
			return stamp
		}
		now = now.Add(time.Second)
	}
}

// makeValidator writes internal/server/validator/<group>/<name>.go, or
// internal/server/validator/<name>.go without a group, and adds its message
// key to internal/platform/trans/lang/fa.json.
func makeValidator(p *Project, name string, report *Report) error {
	dir, pkg, prefix := validatorDir, "validator", "validation"
	var item string
	var err error
	if strings.Contains(strings.Trim(filepath.ToSlash(name), "/"), "/") {
		var group feature
		if group, item, err = splitItem("validator", name); err != nil {
			return err
		}
		dir, pkg, prefix = dir+"/"+group.Path, group.Pkg, group.Label()
	} else if item, err = identifier("validator", name); err != nil {
		return err
	}
	if err := p.require(validatePath, "make:validator builds on validate.Base"); err != nil {
		return err
	}
	if err := p.require(langPath, "make:validator adds the validator's message there"); err != nil {
		return err
	}

	key := prefix + ":" + strings.ReplaceAll(item, "_", "-")
	d := data{Module: p.Module, Pkg: pkg, Type: pascal(item), Value: key}
	file := dir + "/" + item + ".go"
	if err := renderAll(p.Root, []output{{file, "make/validator.go.tmpl"}}, d, false, report); err != nil {
		return err
	}
	if err := addPhrase(p, key, "مقدار وارد شده معتبر نیست.", report); err != nil {
		return err
	}
	report.next("write Passes in %s and the message of %q in %s", file, key, langPath)
	report.next("use it: if err := %s.New%s(\"body.field\", value).Validate(ctx); err != nil { return err }", pkg, d.Type)
	return nil
}

// addPhrase adds key to the language file, after the last entry, unless it
// is there already. The rest of the file is kept as it is.
func addPhrase(p *Project, key, text string, report *Report) error {
	path := filepath.Join(p.Root, filepath.FromSlash(langPath))
	src, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var phrases map[string]string
	if err := json.Unmarshal(src, &phrases); err != nil {
		return fmt.Errorf("%s: %w", langPath, err)
	}
	if _, ok := phrases[key]; ok {
		return nil
	}
	quote := func(s string) string {
		var buf bytes.Buffer
		enc := json.NewEncoder(&buf)
		enc.SetEscapeHTML(false)
		_ = enc.Encode(s) // a string always encodes
		return strings.TrimSpace(buf.String())
	}
	end := bytes.LastIndexByte(src, '}')
	head := bytes.TrimRight(src[:end], " \t\r\n")
	sep := ",\n"
	if len(phrases) == 0 {
		sep = "\n"
	}
	var out bytes.Buffer
	out.Write(head)
	out.WriteString(sep + "  " + quote(key) + ": " + quote(text) + "\n")
	out.Write(src[end:])
	if !json.Valid(out.Bytes()) {
		return fmt.Errorf("%s: could not add %q; add it by hand", langPath, key)
	}
	if err := os.WriteFile(path, out.Bytes(), 0o644); err != nil {
		return err
	}
	report.Updated = append(report.Updated, langPath)
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
	if err := p.need("redis", "make:cache"); err != nil {
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
	report.next("build it in %s: %s.New%s(rdb, time.Hour, load)", appServe, f.Pkg, d.Type)
	return nil
}

func makeMiddleware(p *Project, name string, opts MakeOptions, report *Report) error {
	n, err := identifier("middleware", name)
	if err != nil {
		return err
	}
	d := data{Module: p.Module, Func: pascal(n)}
	if err := renderAll(p.Root, []output{{middlewareDir + "/" + n + ".go", "make/middleware.go.tmpl"}}, d, opts.Force, report); err != nil {
		return err
	}
	report.next("use it in %s (every route) or a feature's routes.go, e.g. g.Use(middleware.%s())", appServe, d.Func)
	return nil
}

// makeClient writes an HTTP client: internal/clients/<name>/client.go for one
// of our own services, internal/external/<name>/<name>.go for a third-party
// API. file is the file name; empty means <name>.go.
func makeClient(p *Project, name, parent, file, tmpl string, opts MakeOptions, report *Report) error {
	n, err := identifier(strings.TrimSuffix(parent, "s"), name)
	if err != nil {
		return err
	}
	pkg := strings.Map(func(r rune) rune {
		if unicode.IsLetter(r) || unicode.IsDigit(r) {
			return r
		}
		return -1
	}, n)
	dir := "internal/" + parent + "/" + pkg
	if p.exists(dir) {
		return fmt.Errorf("%s already exists; pick another name", dir)
	}
	if file == "" {
		file = pkg
	}
	d := data{Package: pkg, Title: title(n), EnvPrefix: strings.ToUpper(n)}
	if err := renderAll(p.Root, []output{{dir + "/" + file + ".go", tmpl}}, d, opts.Force, report); err != nil {
		return err
	}
	report.next("add %s_BASE_URL to internal/config and build it in %s: %s.New(%s.Config{BaseURL: ...})", d.EnvPrefix, appServe, pkg, pkg)
	report.next("a feature that calls it declares the small interface it needs and receives the client in its constructor")
	return nil
}

// makeTest writes a test: for a feature (user), a service test with a fake
// Repository; for an action (user/list), an HTTP test of that handler.
func makeTest(p *Project, name string, opts MakeOptions, report *Report) error {
	if f, err := parseFeature(name); err == nil && p.exists(f.Dir()+"/service.go") {
		return renderAll(p.Root, []output{{f.Dir() + "/service_test.go", "make/feature_service_test.go.tmpl"}}, f.data(p), opts.Force, report)
	}
	f, action, err := splitItem("test", name)
	if err != nil {
		return err
	}
	if err := p.require(f.Dir()+"/handler.go", "run make:feature "+f.Path+" first"); err != nil {
		return err
	}
	d := handlerData(p, f, action)
	src, err := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(f.Dir()), "handler.go"))
	if err != nil {
		return err
	}
	parsed, err := parser.ParseFile(token.NewFileSet(), "handler.go", src, parser.SkipObjectResolution)
	if err != nil {
		return err
	}
	if !slices.Contains(declNames(parsed), "Handler."+d.Action) {
		return fmt.Errorf("%s/handler.go has no %s; run make:handler %s first", f.Dir(), d.Action, name)
	}
	return renderAll(p.Root, []output{{f.Dir() + "/" + action + "_test.go", "make/handler_test.go.tmpl"}}, d, opts.Force, report)
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
