package generator

import (
	"bytes"
	"errors"
	"go/ast"
	"go/parser"
	"go/printer"
	"go/token"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"slices"
	"strconv"
	"strings"
)

// Route is one endpoint found in the project's route files.
type Route struct {
	Method  string // GET
	Path    string // /api/users/:id
	Handler string // user.Handler.Get
	File    string // internal/user/routes.go:12
}

var routeMethods = map[string]bool{
	"GET": true, "POST": true, "PUT": true, "PATCH": true, "DELETE": true,
	"HEAD": true, "OPTIONS": true, "CONNECT": true, "TRACE": true, "Any": true,
}

// Routes lists the endpoints of the project, read from the source: /ping and
// /health (internal/server/health.go), what internal/app/serve.go mounts on e
// (/swagger/*), and the RegisterRoutes function of every feature's routes.go,
// whose paths are under the project's BASE_PATH. It reads the source, it does
// not run it, so routes added in other ways (a loop, a helper) are not seen.
func Routes(p *Project) ([]Route, error) {
	if err := p.Check(); err != nil {
		return nil, err
	}
	var routes []Route
	for _, f := range []struct{ file, fn, receiver string }{
		{"internal/server/health.go", "Register", "server.Health"},
		{appServe, "Serve", ""},
	} {
		found, err := fileRoutes(p, f.file, f.fn, "", f.receiver)
		if err != nil && !errors.Is(err, os.ErrNotExist) {
			return nil, err
		}
		routes = append(routes, found...)
	}

	var files []string
	err := filepath.WalkDir(filepath.Join(p.Root, "internal"), func(path string, d fs.DirEntry, err error) error {
		if err == nil && !d.IsDir() && d.Name() == "routes.go" {
			files = append(files, path)
		}
		return err
	})
	if err != nil {
		return nil, err
	}
	slices.Sort(files)
	for _, file := range files {
		rel, _ := filepath.Rel(p.Root, file)
		rel = filepath.ToSlash(rel)
		feature := strings.TrimPrefix(path.Dir(rel), "internal/")
		found, err := fileRoutes(p, rel, "RegisterRoutes", p.basePath(), feature+".Handler")
		if err != nil {
			return nil, err
		}
		routes = append(routes, found...)
	}
	return routes, nil
}

// fileRoutes reads the routes function fn of the file rel registers, under
// prefix. receiver names what h is, e.g. "user.Handler", in the result.
func fileRoutes(p *Project, rel, fn, prefix, receiver string) ([]Route, error) {
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, filepath.Join(p.Root, filepath.FromSlash(rel)), nil, parser.SkipObjectResolution)
	if err != nil {
		return nil, err
	}
	var routes []Route
	for _, decl := range file.Decls {
		if f, ok := decl.(*ast.FuncDecl); ok && f.Name.Name == fn && f.Body != nil {
			routes = append(routes, routesIn(p, fset, f, prefix, importNames(file))...)
		}
	}
	if receiver != "" {
		for i := range routes {
			if rest, ok := strings.CutPrefix(routes[i].Handler, "h."); ok {
				routes[i].Handler = receiver + "." + rest
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
	for _, name := range []string{"e"} { // app and server register on e directly
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
				handlers[name.Name] = strings.TrimPrefix(strings.TrimPrefix(path, p.Module+"/"), "internal/")
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

// handlerName is "user.Handler.List" for userHandler.List when userHandler
// came from user.NewHandler, otherwise the expression as written.
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
