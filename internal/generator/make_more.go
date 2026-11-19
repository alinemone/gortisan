package generator

import (
	"errors"
	"fmt"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"
)

// ensure writes the outputs that do not exist yet, without reporting the
// ones that do: shared files such as internal/validate or internal/cdc/cdc.go
// that several commands need.
func ensure(p *Project, outputs []output, data any, report *Report) error {
	for _, out := range outputs {
		if p.exists(out.path) {
			continue
		}
		if err := renderAll(p.Root, []output{out}, data, false, report); err != nil {
			return err
		}
	}
	return nil
}

func (p *Project) require(path, why string) error {
	if !p.exists(path) {
		return fmt.Errorf("%s is missing; %s", path, why)
	}
	return nil
}

// ensureValidate writes internal/validate when the project has none
// (projects created before it existed).
func ensureValidate(p *Project, report *Report) error {
	return ensure(p, []output{
		{"internal/validate/validate.go", "create/validate.go.tmpl"},
		{"internal/validate/validate_test.go", "create/validate_test.go.tmpl"},
	}, projectData{Module: p.Module}, report)
}

// splitName reads "order/create_order" or "create_order": the file name and
// the Go type name. Like handlers, the group is prefixed unless the action
// already names it (order/create_order -> CreateOrder, user/list -> UserList).
func splitName(kind, name string) (file, typeName string, err error) {
	group, action, ok := strings.Cut(strings.Trim(filepath.ToSlash(name), "/"), "/")
	if !ok {
		n, err := identifier(kind, group)
		return n, pascal(n), err
	}
	if strings.Contains(action, "/") {
		return "", "", fmt.Errorf("%s name must be <name> or <group>/<name>; got %q", kind, name)
	}
	g, err := identifier(kind+" group", group)
	if err != nil {
		return "", "", err
	}
	a, err := identifier(kind, action)
	if err != nil {
		return "", "", err
	}
	if strings.Contains(a, g) {
		return a, pascal(a), nil // order/order_detail -> order_detail.go, OrderDetail
	}
	return g + "_" + a, pascal(g) + pascal(a), nil
}

type namedData struct {
	Module  string
	Func    string
	Type    string
	Message string
}

func makeMiddleware(p *Project, name string, opts MakeOptions, report *Report) error {
	n, err := identifier("middleware", name)
	if err != nil {
		return err
	}
	data := namedData{Module: p.Module, Func: pascal(n)}
	if err := renderAll(p.Root, []output{{"internal/middleware/" + n + ".go", "make/middleware.go.tmpl"}}, data, opts.Force, report); err != nil {
		return err
	}
	report.next("register it in internal/router/router.go, e.g. v1.Use(middleware.%s())", data.Func)
	return nil
}

func makeRequest(p *Project, name string, opts MakeOptions, report *Report) error {
	file, typeName, err := splitName("request", name)
	if err != nil {
		return err
	}
	if err := ensureValidate(p, report); err != nil {
		return err
	}
	data := namedData{Module: p.Module, Type: strings.TrimSuffix(typeName, "Request") + "Request"}
	return renderAll(p.Root, []output{{"internal/request/" + file + ".go", "make/request.go.tmpl"}}, data, opts.Force, report)
}

func makeResponse(p *Project, name string, opts MakeOptions, report *Report) error {
	file, typeName, err := splitName("response", name)
	if err != nil {
		return err
	}
	data := namedData{Module: p.Module, Type: strings.TrimSuffix(typeName, "Response") + "Response"}
	return renderAll(p.Root, []output{{"internal/response/" + file + ".go", "make/response.go.tmpl"}}, data, opts.Force, report)
}

func makeValidator(p *Project, name string, opts MakeOptions, report *Report) error {
	file, typeName, err := splitName("validator", name)
	if err != nil {
		return err
	}
	if err := ensureValidate(p, report); err != nil {
		return err
	}
	data := namedData{Module: p.Module, Type: typeName, Message: "is invalid"}
	if err := renderAll(p.Root, []output{{"internal/validator/" + file + ".go", "make/validator.go.tmpl"}}, data, opts.Force, report); err != nil {
		return err
	}
	report.next("use it in a request's Validate: errs.Check(ctx, \"field\", validator.New%s(...))", typeName)
	return nil
}

// EnumMember is one value of a generated enum.
type EnumMember struct {
	Name  string // Paid
	Label string // paid
	Value string // 2, or "paid" for string enums
}

type enumData struct {
	Type    string
	Plural  string
	Base    string
	Members []EnumMember
}

var enumBases = map[string]bool{
	"string": true, "int": true, "int8": true, "int16": true, "int32": true, "int64": true,
	"uint8": true, "uint16": true, "uint32": true, "uint64": true,
}

func makeEnum(p *Project, name string, opts MakeOptions, report *Report) error {
	n, err := identifier("enum", name)
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
	data := enumData{Type: pascal(n), Plural: pascal(plural(n)), Base: base, Members: members}
	return renderAll(p.Root, []output{{"internal/enum/" + n + ".go", "make/enum.go.tmpl"}}, data, opts.Force, report)
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

type cacheData struct {
	Module      string
	Type        string
	Prefix      string
	Value       string
	ModelImport bool
}

func makeCache(p *Project, name string, opts MakeOptions, report *Report) error {
	if err := p.require("internal/cache/redis.go", "make:cache needs a project created with redis"); err != nil {
		return err
	}
	n, err := identifier("cache", strings.TrimSuffix(snake(name), "_cache"))
	if err != nil {
		return err
	}
	value := opts.Type
	if value == "" {
		value = "string"
	}
	data := cacheData{
		Module:      p.Module,
		Type:        pascal(n) + "Cache",
		Prefix:      n,
		Value:       value,
		ModelImport: strings.Contains(value, "model."),
	}
	if err := renderAll(p.Root, []output{{"internal/cache/" + n + ".go", "make/cache.go.tmpl"}}, data, opts.Force, report); err != nil {
		return err
	}
	report.next("build it in main.go with cache.New%s(rdb, time.Hour, load)", data.Type)
	return nil
}

type clientData struct {
	Package   string
	Title     string
	EnvPrefix string
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
	data := clientData{Package: pkg, Title: title(n), EnvPrefix: strings.ToUpper(n)}
	if err := renderAll(p.Root, []output{{"internal/client/" + pkg + "/client.go", "make/client.go.tmpl"}}, data, opts.Force, report); err != nil {
		return err
	}
	report.next("add %s_BASE_URL to internal/config and build it in main.go with %s.New(%s.Config{BaseURL: ...})", data.EnvPrefix, pkg, pkg)
	return nil
}

type workerData struct {
	Module string
	Name   string
	Type   string
	Topic  string
	Group  string
}

func makeWorker(p *Project, name string, opts MakeOptions, report *Report) error {
	if err := p.require("internal/kafka/kafka.go", "make:worker needs a project created with --with kafka"); err != nil {
		return err
	}
	n, err := identifier("worker", strings.TrimSuffix(snake(name), "_worker"))
	if err != nil {
		return err
	}
	data := workerData{Module: p.Module, Name: n, Type: pascal(n), Topic: opts.Topic, Group: opts.Group}
	if data.Topic == "" {
		data.Topic = n
	}
	if data.Group == "" {
		data.Group = n
	}
	mainFile := "cmd/worker/main.go"
	isFirst := !p.exists(mainFile)
	if err := renderAll(p.Root, []output{{"internal/worker/" + n + ".go", "make/worker.go.tmpl"}}, data, opts.Force, report); err != nil {
		return err
	}
	if isFirst {
		if err := renderAll(p.Root, []output{{mainFile, "make/worker_main.go.tmpl"}}, data, false, report); err != nil {
			return err
		}
	} else {
		report.next("add it to %s:\n\tcase %q:\n\t\treturn worker.New%s(cfg.Kafka, logger).Run(ctx)", mainFile, n, data.Type)
	}
	report.next("run it with: go run ./cmd/worker %s   (or ./gortisan run worker -- %s)", n, n)
	return nil
}

type cdcData struct {
	Module string
	Type   string
	Var    string
	Model  string
	Topic  string
}

func makeCDC(p *Project, name string, opts MakeOptions, report *Report) error {
	if err := p.require("internal/kafka/kafka.go", "make:cdc needs a project created with --with kafka"); err != nil {
		return err
	}
	n, err := identifier("cdc", strings.TrimSuffix(snake(name), "_cdc"))
	if err != nil {
		return err
	}
	if err := p.require("internal/model/"+n+".go", "run make:model "+n+" first"); err != nil {
		return err
	}
	data := cdcData{Module: p.Module, Type: pascal(n) + "CDC", Var: camel(n), Model: pascal(n), Topic: opts.Topic}
	if data.Topic == "" {
		data.Topic = plural(n)
	}
	if err := ensure(p, []output{
		{"internal/cdc/cdc.go", "make/cdc_base.go.tmpl"},
		{"internal/cdc/cdc_test.go", "make/cdc_test.go.tmpl"},
	}, data, report); err != nil {
		return err
	}
	if err := renderAll(p.Root, []output{{"internal/cdc/" + n + ".go", "make/cdc.go.tmpl"}}, data, opts.Force, report); err != nil {
		return err
	}
	report.next("build it in main.go with cdc.New%s(kafka.NewWriter(cfg.Kafka)) and call Created/Updated/Deleted after each commit", data.Type)
	return nil
}
