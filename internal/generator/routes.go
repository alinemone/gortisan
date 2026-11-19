package generator

import (
	"bytes"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Route is one endpoint found in the project's route files.
type Route struct {
	Method  string // GET
	Path    string // /v1/users/:id
	Handler string // v1/users.Handler.Get
	File    string // internal/api/v1/routes.go:31
}

var routeMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true, "CONNECT": true, "TRACE": true, "Any": true,
}

// Routes reads internal/api/router.go and the Register function of every
// package it mounts (internal/api/v1/routes.go, ...) and lists the routes
// they register. It reads the source, it does not run it, so routes added
// in other ways (a loop, a helper) are not seen.
func Routes(p *Project) ([]Route, error) {
	router := filepath.Join(p.Root, "internal", "api", "router.go")
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, router, nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	imports := importNames(file)

	var routes []Route
	for _, decl := range file.Decls {
		fn, ok := decl.(*ast.FuncDecl)
		if !ok || fn.Body == nil {
			continue
		}
		routes = append(routes, routesIn(p, fset, fn, "", nil)...)
		// v1.Register(e.Group("/v1"), r.V1) mounts a version package.
		ast.Inspect(fn.Body, func(n ast.Node) bool {
			call, ok := n.(*ast.CallExpr)
			if !ok || len(call.Args) == 0 {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			pkg, isIdent := identOf(sel)
			if !ok || !isIdent || sel.Sel.Name != "Register" {
				return true
			}
			importPath, known := imports[pkg]
			rel, inProject := strings.CutPrefix(importPath, p.Module+"/")
			if !known || !inProject {
				return true
			}
			prefix := groupPrefix(call.Args[0])
			found, err := packageRoutes(p, rel, prefix)
			if err == nil {
				routes = append(routes, found...)
			}
			return true
		})
	}
	return routes, nil
}

// packageRoutes reads the Register function of the package in dir.
func packageRoutes(p *Project, dir, prefix string) ([]Route, error) {
	files, _ := filepath.Glob(filepath.Join(p.Root, filepath.FromSlash(dir), "*.go"))
	slices.Sort(files)
	fset := token.NewFileSet()
	var routes []Route
	for _, path := range files {
		if strings.HasSuffix(path, "_test.go") {
			continue
		}
		file, err := parser.ParseFile(fset, path, nil, parser.SkipObjectResolution)
		if err != nil {
			return nil, err
		}
		imports := importNames(file)
		for _, decl := range file.Decls {
			if fn, ok := decl.(*ast.FuncDecl); ok && fn.Name.Name == "Register" && fn.Recv == nil && fn.Body != nil {
				routes = append(routes, routesIn(p, fset, fn, prefix, imports)...)
			}
		}
	}
	return routes, nil
}

// routesIn lists the routes fn registers: calls like g.GET("/x", h.List)
// on its first parameter or on groups made from it (admin := g.Group("/admin")).
func routesIn(p *Project, fset *token.FileSet, fn *ast.FuncDecl, prefix string, imports map[string]string) []Route {
	groups := map[string]string{} // variable -> path prefix
	if params := fn.Type.Params.List; len(params) > 0 && len(params[0].Names) > 0 {
		groups[params[0].Names[0].Name] = prefix
	}
	for _, name := range []string{"e"} { // router.go registers on e directly
		if _, ok := groups[name]; !ok {
			groups[name] = prefix
		}
	}
	handlers := map[string]string{} // variable -> package that built it

	var routes []Route
	ast.Inspect(fn.Body, func(n ast.Node) bool {
		switch node := n.(type) {
		case *ast.AssignStmt:
			if len(node.Lhs) != 1 || len(node.Rhs) != 1 {
				return true
			}
			name, ok := node.Lhs[0].(*ast.Ident)
			call, isCall := node.Rhs[0].(*ast.CallExpr)
			if !ok || !isCall {
				return true
			}
			sel, ok := call.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			recv, _ := identOf(sel)
			if base, isGroup := groups[recv]; isGroup && sel.Sel.Name == "Group" && len(call.Args) > 0 {
				groups[name.Name] = base + stringArg(call.Args[0])
			} else if path, isPkg := imports[recv]; isPkg && strings.HasPrefix(sel.Sel.Name, "New") {
				handlers[name.Name] = strings.TrimPrefix(strings.TrimPrefix(path, p.Module+"/"), "internal/api/")
			}
		case *ast.CallExpr:
			sel, ok := node.Fun.(*ast.SelectorExpr)
			if !ok {
				return true
			}
			recv, _ := identOf(sel)
			base, isGroup := groups[recv]
			if !isGroup {
				return true
			}
			method, args := sel.Sel.Name, node.Args
			if method == "Add" && len(args) >= 3 {
				method, args = strings.Trim(exprString(fset, args[0]), `"`), args[1:]
			}
			if !routeMethods[method] || len(args) < 2 {
				return true
			}
			pos := fset.Position(node.Pos())
			rel, _ := filepath.Rel(p.Root, pos.Filename)
			routes = append(routes, Route{
				Method:  strings.ToUpper(method),
				Path:    base + stringArg(args[0]),
				Handler: handlerName(fset, args[1], handlers),
				File:    filepath.ToSlash(rel) + ":" + strconv.Itoa(pos.Line),
			})
		}
		return true
	})
	return routes
}

// handlerName is "v1/users.Handler.List" for userHandler.List when
// userHandler came from users.NewHandler, otherwise the expression as written.
func handlerName(fset *token.FileSet, expr ast.Expr, handlers map[string]string) string {
	if sel, ok := expr.(*ast.SelectorExpr); ok {
		if recv, ok := identOf(sel); ok {
			if pkg, found := handlers[recv]; found {
				return pkg + ".Handler." + sel.Sel.Name
			}
		}
	}
	return exprString(fset, expr)
}

func identOf(sel *ast.SelectorExpr) (string, bool) {
	if sel == nil {
		return "", false
	}
	id, ok := sel.X.(*ast.Ident)
	if !ok {
		return "", false
	}
	return id.Name, true
}

// groupPrefix reads "/v1" from e.Group("/v1").
func groupPrefix(expr ast.Expr) string {
	if call, ok := expr.(*ast.CallExpr); ok {
		if sel, ok := call.Fun.(*ast.SelectorExpr); ok && sel.Sel.Name == "Group" && len(call.Args) > 0 {
			return stringArg(call.Args[0])
		}
	}
	return ""
}

func stringArg(expr ast.Expr) string {
	if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
		if s, err := strconv.Unquote(lit.Value); err == nil {
			return s
		}
	}
	return "{?}"
}

func exprString(fset *token.FileSet, expr ast.Expr) string {
	var buf bytes.Buffer
	_ = printer.Fprint(&buf, fset, expr)
	return buf.String()
}

// importNames maps the name a file uses for each import to its path.
func importNames(file *ast.File) map[string]string {
	names := map[string]string{}
	for _, imp := range file.Imports {
		path, _ := strconv.Unquote(imp.Path.Value)
		name := filepath.Base(path)
		if imp.Name != nil {
			name = imp.Name.Name
		}
		names[name] = path
	}
	return names
}
