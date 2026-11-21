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
	"strconv"
	"strings"
)

// errNoFunc is returned by addToFunc when the file has no such function, for
// example after someone renamed it; the caller then prints the lines instead.
var errNoFunc = errors.New("function not found")

// addToFunc appends lines to the end of the body of function fn in the file
// rel, and adds the imports it lacks. A line already in the body is not
// added again; drop lines are removed (a placeholder such as `_ = g`). With
// separate, the new lines are a block of their own, after a blank line. It
// reports whether the file changed. Nothing else in the file is touched.
func addToFunc(p *Project, rel, fn string, lines, imports, drop []string, separate bool, report *Report) (bool, error) {
	target := filepath.Join(p.Root, filepath.FromSlash(rel))
	src, err := os.ReadFile(target)
	if err != nil {
		return false, err
	}
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, rel, src, parser.ParseComments)
	if err != nil {
		return false, err
	}
	decl := findFunc(file, fn)
	if decl == nil {
		return false, fmt.Errorf("%s: %s: %w", rel, fn, errNoFunc)
	}
	offset := func(pos token.Pos) int { return fset.File(file.Pos()).Offset(pos) }
	bodyStart, bodyEnd := offset(decl.Body.Lbrace)+1, offset(decl.Body.Rbrace)
	body := string(src[bodyStart:bodyEnd])

	var add []string
	for _, line := range lines {
		if !strings.Contains(body, strings.TrimSpace(line)) {
			add = append(add, "\t"+strings.TrimSpace(line))
		}
	}
	kept := body
	for _, d := range drop {
		var out []string
		for _, line := range strings.Split(kept, "\n") {
			if !strings.HasPrefix(strings.TrimSpace(line), d) {
				out = append(out, line)
			}
		}
		kept = strings.Join(out, "\n")
	}
	if len(add) == 0 && kept == body {
		return false, nil
	}
	head := strings.TrimRight(kept, " \t\n")
	if separate && len(add) > 0 && strings.TrimSpace(head) != "" {
		head += "\n" // a blank line between blocks, e.g. one feature and the next
	}
	newBody := head + "\n" + strings.Join(add, "\n") + "\n"
	out := string(src[:bodyStart]) + newBody + string(src[bodyEnd:])

	// Imports go in after the body is spliced, on a fresh parse.
	file, err = parser.ParseFile(fset, rel, out, parser.ParseComments)
	if err != nil {
		return false, err
	}
	withImports, err := addImports([]byte(out), file, fset, imports, p.Module)
	if err != nil {
		return false, err
	}
	formatted, err := format.Source(withImports)
	if err != nil {
		return false, fmt.Errorf("%s: %w", rel, err)
	}
	if err := os.WriteFile(target, formatted, 0o644); err != nil {
		return false, err
	}
	report.Updated = append(report.Updated, rel)
	return true, nil
}

func findFunc(file *ast.File, name string) *ast.FuncDecl {
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok && fn.Recv == nil && fn.Name.Name == name && fn.Body != nil {
			return fn
		}
	}
	return nil
}

// routeGroup reads the RegisterRoutes function of a feature's routes.go and
// returns the variable of its group and the group's path:
// `g := api.Group("/users")` gives "g" and "/users". ok is false when the
// function does not make a group that way.
func routeGroup(p *Project, rel string) (variable, path string, ok bool) {
	src, err := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(rel)))
	if err != nil {
		return "", "", false
	}
	file, err := parser.ParseFile(token.NewFileSet(), rel, src, parser.SkipObjectResolution)
	if err != nil {
		return "", "", false
	}
	decl := findFunc(file, "RegisterRoutes")
	if decl == nil {
		return "", "", false
	}
	for _, stmt := range decl.Body.List {
		assign, isAssign := stmt.(*ast.AssignStmt)
		if !isAssign || len(assign.Lhs) != 1 || len(assign.Rhs) != 1 {
			continue
		}
		name, isIdent := assign.Lhs[0].(*ast.Ident)
		call, isCall := assign.Rhs[0].(*ast.CallExpr)
		if !isIdent || !isCall || len(call.Args) == 0 {
			continue
		}
		sel, isSel := call.Fun.(*ast.SelectorExpr)
		lit, isLit := call.Args[0].(*ast.BasicLit)
		if !isSel || sel.Sel.Name != "Group" || !isLit || lit.Kind != token.STRING {
			continue
		}
		value, err := strconv.Unquote(lit.Value)
		if err != nil {
			continue
		}
		return name.Name, value, true
	}
	return "", "", false
}
