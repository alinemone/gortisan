package generator

import (
	"bytes"
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"io/fs"
	"os"
	"path/filepath"
	"reflect"
	"slices"
	"strconv"
	"strings"
)

// tableDirective marks a struct as a table: //gortisan:table users
const tableDirective = "//gortisan:table "

// Region markers around a generated mapping inside repository.go.
const (
	regionBegin = "// gortisan:table %s begin"
	regionEnd   = "// gortisan:table %s end"
)

type genColumn struct {
	Column string // user_id
	Field  string // UserID
}

type tableSpec struct {
	Module     string
	Pkg        string // user
	Dir        string // internal/user
	File       string // user.go
	Type       string // User
	Var        string // user (userTable)
	Table      string // users
	PK         string
	Columns    []genColumn
	Insert     []genColumn
	UpdatedAt  string
	SoftDelete string
}

// GenerateOptions configure Generate.
type GenerateOptions struct {
	// Check reports out-of-date mappings as an error instead of writing them.
	Check bool
}

// Generate keeps the table mapping of every model — a struct marked with
// //gortisan:table — in step with its fields. The mapping lives in the
// repository.go of the model's package, between "// gortisan:table <Type>
// begin" and "end" lines; nothing else in the file is touched. Files are only
// written when their content changes.
func Generate(p *Project, opts GenerateOptions) (*Report, error) {
	report := &Report{}
	if err := p.require("internal/database/orm/orm.go", "generate maps models for the orm of a project created with postgres"); err != nil {
		return report, err
	}
	byDir, err := readModels(p)
	if err != nil {
		return report, err
	}

	dirs := make([]string, 0, len(byDir))
	for dir := range byDir {
		dirs = append(dirs, dir)
	}
	// Repositories that still hold a mapping whose model is gone, so it can
	// be removed even when the package has no model left.
	repos, _ := filepath.Glob(filepath.Join(p.Root, "internal", "*", "repository.go"))
	for _, path := range repos {
		content, err := os.ReadFile(path)
		if err != nil || len(regionTypes(content)) == 0 {
			continue
		}
		rel, _ := filepath.Rel(p.Root, filepath.Dir(path))
		if dir := filepath.ToSlash(rel); !slices.Contains(dirs, dir) {
			dirs = append(dirs, dir)
		}
	}
	slices.Sort(dirs)

	var stale []string
	for _, dir := range dirs {
		rel := dir + "/repository.go"
		target := filepath.Join(p.Root, filepath.FromSlash(rel))
		old, err := os.ReadFile(target)
		if errors.Is(err, os.ErrNotExist) {
			report.next("%s has no repository.go for the mapping of %s; create one (make:model writes it)", dir, specNames(byDir[dir]))
			continue
		}
		if err != nil {
			return report, err
		}
		updated, notes, err := syncRepository(old, byDir[dir])
		if err != nil {
			return report, fmt.Errorf("%s: %w", rel, err)
		}
		for _, note := range notes {
			report.next("%s: %s", rel, note)
		}
		if bytes.Equal(old, updated) {
			continue
		}
		if opts.Check {
			stale = append(stale, rel)
			continue
		}
		if err := os.WriteFile(target, updated, 0o644); err != nil {
			return report, err
		}
		report.Updated = append(report.Updated, rel)
	}

	if len(stale) > 0 {
		return report, fmt.Errorf("out of date, run gortisan generate: %s", strings.Join(stale, ", "))
	}
	return report, nil
}

func specNames(specs []tableSpec) string {
	var names []string
	for _, s := range specs {
		names = append(names, s.Type)
	}
	return strings.Join(names, ", ")
}

// syncRepository rewrites the mapping regions of src for specs: replaces
// existing regions, appends missing ones, and removes regions whose model is
// gone. A mapping written by hand (a xTable variable outside a region) is
// left alone and reported.
func syncRepository(src []byte, specs []tableSpec) ([]byte, []string, error) {
	var notes []string
	out := src
	wanted := map[string]bool{}
	for _, spec := range specs {
		wanted[spec.Type] = true
		fragment, err := render("make/table_region.go.tmpl", spec)
		if err != nil {
			return nil, nil, err
		}
		region, err := regionOf(fragment, spec.Type)
		if err != nil {
			return nil, nil, err
		}

		if start, end, ok := findRegion(out, spec.Type); ok {
			out = append(append(append([]byte{}, out[:start]...), region...), out[end:]...)
			if out, err = ensureImports(out, fragment); err != nil {
				return nil, nil, err
			}
			continue
		}
		merged, err := appendGo(out, fragment)
		if errors.Is(err, errDeclExists) {
			notes = append(notes, spec.Var+"Table is written by hand; delete it to let generate keep it in step with "+spec.Type)
			continue
		}
		if err != nil {
			return nil, nil, err
		}
		out = merged
	}

	for _, typeName := range regionTypes(out) {
		if !wanted[typeName] {
			start, end, _ := findRegion(out, typeName)
			out = append(append([]byte{}, out[:start]...), out[end:]...)
			notes = append(notes, "removed the mapping of "+typeName+", which is no longer a model")
		}
	}

	formatted, err := format.Source(out)
	if err != nil {
		return nil, nil, err
	}
	return formatted, notes, nil
}

// regionOf cuts the marked region out of a rendered fragment.
func regionOf(fragment []byte, typeName string) ([]byte, error) {
	start, end, ok := findRegion(fragment, typeName)
	if !ok {
		return nil, errors.New("table_region template has no region markers")
	}
	return fragment[start:end], nil
}

// findRegion returns the byte range of the region of typeName, from the start
// of its begin line to the end of its end line.
func findRegion(src []byte, typeName string) (int, int, bool) {
	begin := []byte(fmt.Sprintf(regionBegin, typeName))
	end := []byte(fmt.Sprintf(regionEnd, typeName))
	start := lineStartWith(src, begin, 0)
	if start < 0 {
		return 0, 0, false
	}
	stop := lineStartWith(src, end, start)
	if stop < 0 {
		return 0, 0, false
	}
	if nl := bytes.IndexByte(src[stop:], '\n'); nl >= 0 {
		stop += nl + 1
	} else {
		stop = len(src)
	}
	return start, stop, true
}

// lineStartWith finds the first line at or after from that starts with prefix
// followed by a space, a newline or the end of the file.
func lineStartWith(src, prefix []byte, from int) int {
	for i := from; i < len(src); {
		if (i == 0 || src[i-1] == '\n') && bytes.HasPrefix(src[i:], prefix) {
			rest := src[i+len(prefix):]
			if len(rest) == 0 || rest[0] == ' ' || rest[0] == '\n' || rest[0] == '\r' {
				return i
			}
		}
		nl := bytes.IndexByte(src[i:], '\n')
		if nl < 0 {
			return -1
		}
		i += nl + 1
	}
	return -1
}

func regionTypes(src []byte) []string {
	var types []string
	for _, line := range strings.Split(string(src), "\n") {
		if rest, ok := strings.CutPrefix(line, "// gortisan:table "); ok {
			if name, kind, _ := strings.Cut(rest, " "); strings.HasPrefix(kind, "begin") {
				types = append(types, name)
			}
		}
	}
	return types
}

// ensureImports adds to src the imports of fragment it lacks.
func ensureImports(src, fragment []byte) ([]byte, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "repository.go", src, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	frag, err := parser.ParseFile(token.NewFileSet(), "fragment.go", fragment, parser.ImportsOnly)
	if err != nil {
		return nil, err
	}
	return addImports(src, file, fset, importsOf(frag))
}

// readModels finds every struct marked //gortisan:table under internal/,
// grouped by package directory ("internal/user").
func readModels(p *Project) (map[string][]tableSpec, error) {
	byDir := map[string][]tableSpec{}
	root := filepath.Join(p.Root, "internal")
	fset := token.NewFileSet()
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil
			}
			return err
		}
		if d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		content, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		if !bytes.Contains(content, []byte(tableDirective)) {
			return nil
		}
		file, err := parser.ParseFile(fset, path, content, parser.ParseComments|parser.SkipObjectResolution)
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(p.Root, filepath.Dir(path))
		dir := filepath.ToSlash(rel)
		for _, decl := range file.Decls {
			gen, ok := decl.(*ast.GenDecl)
			if !ok || gen.Tok != token.TYPE {
				continue
			}
			for _, s := range gen.Specs {
				ts := s.(*ast.TypeSpec)
				st, ok := ts.Type.(*ast.StructType)
				if !ok {
					continue
				}
				doc := ts.Doc
				if doc == nil && len(gen.Specs) == 1 {
					doc = gen.Doc
				}
				table, marked := directive(doc)
				if !marked {
					continue
				}
				spec, err := readModel(ts.Name.Name, table, st)
				if err != nil {
					return fmt.Errorf("%s: %w", fset.Position(ts.Pos()), err)
				}
				spec.Module = p.Module
				spec.Pkg = file.Name.Name
				spec.Dir = dir
				spec.File = filepath.Base(path)
				byDir[dir] = append(byDir[dir], spec)
			}
		}
		return nil
	})
	return byDir, err
}

func directive(doc *ast.CommentGroup) (string, bool) {
	if doc == nil {
		return "", false
	}
	for _, c := range doc.List {
		if table, ok := strings.CutPrefix(c.Text, tableDirective); ok {
			return strings.TrimSpace(table), true
		}
	}
	return "", false
}

func readModel(typeName, table string, st *ast.StructType) (tableSpec, error) {
	spec := tableSpec{Type: typeName, Var: camel(typeName), Table: table}
	if !identifierRE.MatchString(table) {
		return spec, fmt.Errorf("%s: //gortisan:table needs a snake_case table name, got %q", typeName, table)
	}
	for _, field := range st.Fields.List {
		if field.Tag == nil {
			continue
		}
		raw, err := strconv.Unquote(field.Tag.Value)
		if err != nil {
			return spec, err
		}
		tag, ok := reflect.StructTag(raw).Lookup("db")
		if !ok || tag == "-" {
			continue
		}
		parts := strings.Split(tag, ",")
		column := strings.TrimSpace(parts[0])
		if !identifierRE.MatchString(column) {
			return spec, fmt.Errorf("%s: db tag %q must start with a snake_case column name", typeName, tag)
		}
		if len(field.Names) == 0 {
			return spec, fmt.Errorf("%s: an embedded field cannot have a db tag; list its columns in the struct", typeName)
		}
		if len(field.Names) > 1 {
			return spec, fmt.Errorf("%s: fields %s share one db tag; give each its own line", typeName, names(field.Names))
		}
		name := field.Names[0].Name
		col := genColumn{Column: column, Field: name}
		pk, dbDefault := false, false
		for _, option := range parts[1:] {
			switch strings.TrimSpace(option) {
			case "pk":
				pk = true
			case "default":
				dbDefault = true
			case "":
			default:
				return spec, fmt.Errorf("%s.%s: unknown db tag option %q (use pk or default)", typeName, name, option)
			}
		}
		if pk {
			if spec.PK != "" {
				return spec, fmt.Errorf("%s: more than one pk column", typeName)
			}
			spec.PK = column
		}
		for _, existing := range spec.Columns {
			if existing.Column == column {
				return spec, fmt.Errorf("%s: column %q is mapped twice", typeName, column)
			}
		}
		spec.Columns = append(spec.Columns, col)
		if !pk && !dbDefault {
			spec.Insert = append(spec.Insert, col)
		}
		switch column {
		case "updated_at":
			spec.UpdatedAt = column
		case "deleted_at":
			spec.SoftDelete = column
		}
	}
	if len(spec.Columns) == 0 {
		return spec, fmt.Errorf("%s: marked //gortisan:table but has no db tags", typeName)
	}
	if spec.PK == "" {
		return spec, errors.New(typeName + `: mark the primary key, e.g. ID int64 ` + "`" + `db:"id,pk"` + "`")
	}
	return spec, nil
}

func names(idents []*ast.Ident) string {
	var out []string
	for _, id := range idents {
		out = append(out, id.Name)
	}
	return strings.Join(out, ", ")
}
