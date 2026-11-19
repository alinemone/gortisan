package generator

import (
	"errors"
	"fmt"
	"go/ast"
	"go/format"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// errDeclExists is returned by appendGo when the file already declares a
// name the new code declares.
var errDeclExists = errors.New("already declared")

// appendGo adds the declarations of fragment — a complete Go file of the same
// package — to the end of existing, and adds the imports existing lacks.
// Nothing already in existing is changed. It fails with errDeclExists when a
// type, function, method, constant or variable of fragment already exists.
func appendGo(existing, fragment []byte) ([]byte, error) {
	efset, ffset := token.NewFileSet(), token.NewFileSet()
	ef, err := parser.ParseFile(efset, "existing.go", existing, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	ff, err := parser.ParseFile(ffset, "fragment.go", fragment, parser.ParseComments)
	if err != nil {
		return nil, err
	}
	if ef.Name.Name != ff.Name.Name {
		return nil, fmt.Errorf("package %s cannot take code of package %s", ef.Name.Name, ff.Name.Name)
	}

	have := declNames(ef)
	for _, name := range declNames(ff) {
		if slices.Contains(have, name) {
			return nil, fmt.Errorf("%s: %w", name, errDeclExists)
		}
	}

	// The fragment's code: everything after its package clause and imports.
	bodyStart := ff.Name.End()
	for _, decl := range ff.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.IMPORT {
			bodyStart = gen.End()
		}
	}
	body := fragment[ffset.File(ff.Pos()).Offset(bodyStart):]

	src, err := addImports(existing, ef, efset, importsOf(ff))
	if err != nil {
		return nil, err
	}
	out := strings.TrimRight(string(src), "\n") + "\n\n" + strings.TrimLeft(string(body), "\n")
	return format.Source([]byte(out))
}

// addImports returns src with the given imports added where missing.
func addImports(src []byte, file *ast.File, fset *token.FileSet, imports []string) ([]byte, error) {
	present := importsOf(file)
	var missing []string
	for _, spec := range imports {
		if !slices.Contains(present, spec) {
			missing = append(missing, spec)
		}
	}
	if len(missing) == 0 {
		return src, nil
	}
	offset := func(pos token.Pos) int { return fset.File(file.Pos()).Offset(pos) }

	// Standard library imports go in the first group, the others in the
	// last, as goimports would place them.
	var std, other []string
	for _, spec := range missing {
		if isStdImport(spec) {
			std = append(std, spec)
		} else {
			other = append(other, spec)
		}
	}
	block := func(groups ...[]string) string {
		var parts []string
		for _, g := range groups {
			if len(g) > 0 {
				parts = append(parts, "\t"+strings.Join(g, "\n\t")+"\n")
			}
		}
		return "\n\nimport (\n" + strings.Join(parts, "\n") + ")"
	}

	for _, decl := range file.Decls {
		gen, ok := decl.(*ast.GenDecl)
		if !ok || gen.Tok != token.IMPORT {
			continue
		}
		if !gen.Lparen.IsValid() {
			return splice(src, offset(gen.End()), block(std, other)), nil
		}
		// Insert the later position first so the earlier offset stays valid.
		out := src
		if len(other) > 0 {
			out = splice(out, offset(gen.Rparen), "\t"+strings.Join(other, "\n\t")+"\n")
		}
		if len(std) > 0 {
			out = splice(out, offset(gen.Lparen)+1, "\n\t"+strings.Join(std, "\n\t"))
		}
		return out, nil
	}
	return splice(src, offset(file.Name.End()), block(std, other)), nil
}

// isStdImport reports whether an import spec names a standard library
// package: its path's first element has no dot.
func isStdImport(spec string) bool {
	path := spec[strings.Index(spec, `"`):]
	first, _, _ := strings.Cut(strings.Trim(path, `"`), "/")
	return !strings.Contains(first, ".")
}

func splice(src []byte, at int, text string) []byte {
	out := make([]byte, 0, len(src)+len(text))
	out = append(out, src[:at]...)
	out = append(out, text...)
	return append(out, src[at:]...)
}

// importsOf returns the import specs of file as written: `"fmt"` or `name "path"`.
func importsOf(file *ast.File) []string {
	var specs []string
	for _, imp := range file.Imports {
		spec := imp.Path.Value
		if imp.Name != nil {
			spec = imp.Name.Name + " " + spec
		}
		specs = append(specs, spec)
	}
	return specs
}

// declNames lists the top-level names a file declares; methods are
// "Receiver.Method".
func declNames(file *ast.File) []string {
	var names []string
	for _, decl := range file.Decls {
		switch d := decl.(type) {
		case *ast.FuncDecl:
			if d.Recv == nil || len(d.Recv.List) == 0 {
				if d.Name.Name != "init" {
					names = append(names, d.Name.Name)
				}
				continue
			}
			names = append(names, receiverName(d.Recv.List[0].Type)+"."+d.Name.Name)
		case *ast.GenDecl:
			for _, spec := range d.Specs {
				switch s := spec.(type) {
				case *ast.TypeSpec:
					names = append(names, s.Name.Name)
				case *ast.ValueSpec:
					for _, n := range s.Names {
						if n.Name != "_" {
							names = append(names, n.Name)
						}
					}
				}
			}
		}
	}
	return names
}

func receiverName(expr ast.Expr) string {
	switch t := expr.(type) {
	case *ast.StarExpr:
		return receiverName(t.X)
	case *ast.Ident:
		return t.Name
	case *ast.IndexExpr:
		return receiverName(t.X)
	case *ast.IndexListExpr:
		return receiverName(t.X)
	}
	return "?"
}

// writeOrAppend renders tmpl and writes it to rel, or appends its
// declarations to rel when the file exists. A declaration that already
// exists is reported as skipped (or overwritten in a new file with force).
func writeOrAppend(p *Project, rel, tmpl string, data any, report *Report) error {
	content, err := render(tmpl, data)
	if err != nil {
		return err
	}
	target := filepath.Join(p.Root, filepath.FromSlash(rel))
	existing, err := os.ReadFile(target)
	if errors.Is(err, os.ErrNotExist) {
		return write(p.Root, rel, content, false, report)
	}
	if err != nil {
		return err
	}
	merged, err := appendGo(existing, content)
	if errors.Is(err, errDeclExists) {
		report.Skipped = append(report.Skipped, rel+" ("+strings.TrimSuffix(err.Error(), ": "+errDeclExists.Error())+" already exists)")
		return nil
	}
	if err != nil {
		return fmt.Errorf("%s: %w", rel, err)
	}
	if err := os.WriteFile(target, merged, 0o644); err != nil {
		return err
	}
	report.Updated = append(report.Updated, rel)
	return nil
}
