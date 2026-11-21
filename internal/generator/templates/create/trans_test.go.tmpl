package trans

import (
	"go/ast"
	"go/parser"
	"go/token"
	"io/fs"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestT(t *testing.T) {
	if got := Message("general:field-required", "موبایل"); got != "موبایل اجباری است." {
		t.Errorf("with an argument: %q", got)
	}
	if got := Message("general:min", "تعداد", 1); got != "تعداد نباید کمتر از 1 باشد." {
		t.Errorf("with a number: %q", got)
	}
	if got := Message("no:such-key", "x"); got != "no:such-key" {
		t.Errorf("a missing key must come back as it is, got %q", got)
	}
}

// TestKeysExist fails when the code uses a key that lang/fa.json does not
// have: trans.Message("<key>", ...) and the Code of a validate.Base.
func TestKeysExist(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	err := filepath.WalkDir(root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if name := d.Name(); path != root && (strings.HasPrefix(name, ".") || name == "vendor" || name == "testdata") {
				return filepath.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return nil
		}
		fset := token.NewFileSet()
		parsed, err := parser.ParseFile(fset, path, nil, 0)
		if err != nil {
			return err
		}
		for _, key := range usedKeys(parsed, fset) {
			if !Has(key.name) {
				t.Errorf("%s: %q is not in %s", key.pos, key.name, file)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}

type usedKey struct {
	name string
	pos  token.Position
}

func usedKeys(file *ast.File, fset *token.FileSet) []usedKey {
	var keys []usedKey
	add := func(expr ast.Expr) {
		if lit, ok := expr.(*ast.BasicLit); ok && lit.Kind == token.STRING {
			if s, err := strconv.Unquote(lit.Value); err == nil {
				keys = append(keys, usedKey{s, fset.Position(lit.Pos())})
			}
		}
	}
	ast.Inspect(file, func(n ast.Node) bool {
		switch n := n.(type) {
		case *ast.CallExpr: // trans.Message("key", ...)
			if isSelector(n.Fun, "trans", "Message") && len(n.Args) > 0 {
				add(n.Args[0])
			}
		case *ast.CompositeLit: // validate.Base{Code: "key"}
			if isSelector(n.Type, "validate", "Base") {
				for _, elt := range n.Elts {
					if kv, ok := elt.(*ast.KeyValueExpr); ok {
						if id, ok := kv.Key.(*ast.Ident); ok && id.Name == "Code" {
							add(kv.Value)
						}
					}
				}
			}
		}
		return true
	})
	return keys
}

func isSelector(expr ast.Expr, pkg, name string) bool {
	sel, ok := expr.(*ast.SelectorExpr)
	if !ok || sel.Sel.Name != name {
		return false
	}
	id, ok := sel.X.(*ast.Ident)
	return ok && id.Name == pkg
}
