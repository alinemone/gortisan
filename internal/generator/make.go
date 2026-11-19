package generator

import (
	"bufio"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
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

// MakeOptions configure Make.
type MakeOptions struct {
	Force      bool
	Table      string
	Fields     string
	SoftDelete bool
	// Now stamps migration file names; time.Now when zero.
	Now time.Time
}

// MakeKinds are what `gortisan make` can write.
var MakeKinds = []string{"handler", "service", "model", "migration"}

// Make writes one building block of kind named name into p.
func Make(p *Project, kind, name string, opts MakeOptions) (*Report, error) {
	report := &Report{}
	var err error
	switch kind {
	case "handler":
		err = makeHandler(p, name, opts, report)
	case "service":
		err = makeService(p, name, opts, report)
	case "model":
		err = makeModel(p, name, opts, report)
	case "migration":
		err = makeMigration(p, name, opts, report)
	default:
		err = fmt.Errorf("unknown kind %q; choose from %s", kind, strings.Join(MakeKinds, ", "))
	}
	return report, err
}

type handlerData struct {
	Module   string
	Group    string // order
	Handler  string // OrderHandler
	Field    string // Order (field on router.Handlers)
	Method   string // GetOrder
	TypeName string // GetOrder, or UserList for user/list
	Route    string // /orders
}

// makeHandler writes internal/handler/<group>.go (once) and
// internal/handler/<group>_<action>.go for "group/action".
func makeHandler(p *Project, name string, opts MakeOptions, report *Report) error {
	groupName, action, ok := strings.Cut(strings.Trim(filepath.ToSlash(name), "/"), "/")
	if !ok || strings.Contains(action, "/") {
		return fmt.Errorf("handler name must be <group>/<action>, e.g. order/get_order; got %q", name)
	}
	group, err := identifier("handler group", groupName)
	if err != nil {
		return err
	}
	act, err := identifier("handler action", action)
	if err != nil {
		return err
	}
	method := pascal(act)
	typeName := method
	if !strings.Contains(act, group) {
		typeName = pascal(group) + method
	}
	data := handlerData{
		Module:   p.Module,
		Group:    group,
		Handler:  pascal(group) + "Handler",
		Field:    pascal(group),
		Method:   method,
		TypeName: typeName,
		Route:    "/" + plural(strings.ReplaceAll(group, "_", "-")),
	}
	groupFile := "internal/handler/" + group + ".go"
	isNewGroup := !p.exists(groupFile)
	if err := renderAll(p.Root, []output{
		{groupFile, "make/handler_group.go.tmpl"},
		{"internal/handler/" + group + "_" + act + ".go", "make/handler.go.tmpl"},
	}, data, opts.Force, report); err != nil {
		return err
	}
	if isNewGroup {
		report.next("add `%s *handler.%s` to router.Handlers and build it in main.go with handler.New%s()", data.Field, data.Handler, data.Handler)
	}
	report.next("register the route in internal/router/router.go, e.g. v1.GET(\"%s\", h.%s.%s)", data.Route, data.Field, data.Method)
	return nil
}

type serviceData struct {
	Module string
	Type   string
}

func makeService(p *Project, name string, opts MakeOptions, report *Report) error {
	n, err := identifier("service", strings.TrimSuffix(snake(name), "_service"))
	if err != nil {
		return err
	}
	data := serviceData{Module: p.Module, Type: pascal(n) + "Service"}
	if err := renderAll(p.Root, []output{
		{"internal/service/" + n + "_service.go", "make/service.go.tmpl"},
	}, data, opts.Force, report); err != nil {
		return err
	}
	report.next("build it in main.go with service.New%s(...) and pass it to the handlers that use it", data.Type)
	return nil
}

type modelData struct {
	Module     string
	Name       string // order_item
	Type       string // OrderItem
	Var        string // orderItem
	Table      string // order_items
	Fields     []Field
	SoftDelete bool
	JSON       bool // a field is json.RawMessage
}

func makeModel(p *Project, name string, opts MakeOptions, report *Report) error {
	if !p.exists("internal/database/orm/orm.go") {
		return errors.New("internal/database/orm is missing; make model needs a project created with postgres")
	}
	n, err := identifier("model", name)
	if err != nil {
		return err
	}
	table := opts.Table
	if table == "" {
		table = plural(n)
	}
	if table, err = identifier("table", table); err != nil {
		return err
	}
	fields, err := parseFields(opts.Fields)
	if err != nil {
		return err
	}
	data := modelData{
		Module:     p.Module,
		Name:       n,
		Type:       pascal(n),
		Var:        camel(n),
		Table:      table,
		Fields:     fields,
		SoftDelete: opts.SoftDelete,
	}
	for _, f := range fields {
		if f.JSON {
			data.JSON = true
		}
	}
	stamp := migrationStamp(opts.Now)
	if err := renderAll(p.Root, []output{
		{"internal/model/" + n + ".go", "make/model.go.tmpl"},
		{"internal/repository/" + n + "_repository.go", "make/repository.go.tmpl"},
		{"migrations/" + stamp + "_create_" + table + "_table.up.sql", "make/create_table.up.sql.tmpl"},
		{"migrations/" + stamp + "_create_" + table + "_table.down.sql", "make/create_table.down.sql.tmpl"},
	}, data, opts.Force, report); err != nil {
		return err
	}
	report.next("apply migrations/%s_create_%s_table.up.sql to the database", stamp, table)
	report.next("build it in main.go with repository.New%sRepository(db)", data.Type)
	return nil
}

type migrationData struct {
	Name string
}

func makeMigration(p *Project, name string, opts MakeOptions, report *Report) error {
	n, err := identifier("migration", name)
	if err != nil {
		return err
	}
	stamp := migrationStamp(opts.Now)
	return renderAll(p.Root, []output{
		{"migrations/" + stamp + "_" + n + ".up.sql", "make/migration.up.sql.tmpl"},
		{"migrations/" + stamp + "_" + n + ".down.sql", "make/migration.down.sql.tmpl"},
	}, migrationData{Name: n}, opts.Force, report)
}

func migrationStamp(now time.Time) string {
	if now.IsZero() {
		now = time.Now()
	}
	return now.UTC().Format("20060102150405")
}
