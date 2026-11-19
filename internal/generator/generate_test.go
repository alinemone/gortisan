package generator

import (
	"errors"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// genProject is a project with go.mod, the orm marker file and the given
// files, paths relative to the root.
func genProject(t *testing.T, files map[string]string) *Project {
	t.Helper()
	root := t.TempDir()
	all := map[string]string{
		"go.mod":                                "module example.com/x\n\ngo 1.26\n",
		"internal/platform/database/orm/orm.go": "package orm\n",
	}
	for rel, content := range files {
		all[rel] = content
	}
	for rel, content := range all {
		path := filepath.Join(root, filepath.FromSlash(rel))
		if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
			t.Fatal(err)
		}
	}
	p, err := FindProject(root)
	if err != nil {
		t.Fatal(err)
	}
	return p
}

const orderModel = `package order

import "time"

// Order is a row.
//
//gortisan:table shop_orders
type Order struct {
	ID        int64      ` + "`db:\"id,pk\"`" + `
	UserID    int64      ` + "`db:\"user_id\" json:\"user_id\"`" + `
	Note      *string    ` + "`db:\"note\"`" + `
	Cached    string     ` + "`json:\"-\"`" + `
	Skipped   string     ` + "`db:\"-\"`" + `
	CreatedAt time.Time  ` + "`db:\"created_at,default\"`" + `
	UpdatedAt time.Time  ` + "`db:\"updated_at,default\"`" + `
	DeletedAt *time.Time ` + "`db:\"deleted_at,default\"`" + `
}

// Filter has db tags but no directive, so it is not a table.
type Filter struct {
	Status string ` + "`db:\"status\"`" + `
}
`

const orderRepository = `package order

import (
	"context"

	"example.com/x/internal/platform/database"
)

// Repository is written by hand.
type Repository struct {
	db database.DBTX
}

func (r *Repository) Ping(ctx context.Context) error { return nil }
`

func readFile(t *testing.T, p *Project, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func TestGenerateWritesTheMappingIntoTheRepository(t *testing.T) {
	p := genProject(t, map[string]string{
		"internal/order/order.go":      orderModel,
		"internal/order/repository.go": orderRepository,
	})
	report, err := Generate(p, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Updated) != 1 || report.Updated[0] != "internal/order/repository.go" {
		t.Fatalf("report = %+v", report)
	}
	src := readFile(t, p, "internal/order/repository.go")
	for _, want := range []string{
		"// Repository is written by hand.", // the rest of the file is kept
		`"example.com/x/internal/platform/database/orm"`,
		"// gortisan:table Order begin",
		"var orderTable = orm.Table[Order]{",
		`Name:       "shop_orders"`,
		`"id",
		"user_id",
		"note",
		"created_at",
		"updated_at",
		"deleted_at",`,
		`InsertColumns: []string{
		"user_id",
		"note",
	},`,
		`SoftDeleteColumn: "deleted_at"`,
		"// gortisan:table Order end",
	} {
		if !strings.Contains(src, want) {
			t.Errorf("repository.go lacks %q:\n%s", want, src)
		}
	}
	if strings.Contains(src, "Cached") || strings.Contains(src, "Skipped") || strings.Contains(src, "Filter") {
		t.Errorf("only tagged fields of marked structs are mapped:\n%s", src)
	}
	if _, err := os.Stat(filepath.Join(p.Root, "internal", "order", "order_table_gen.go")); err == nil {
		t.Error("no separate generated file is written")
	}

	// Running again changes nothing, and --check passes.
	again, err := Generate(p, GenerateOptions{})
	if err != nil || len(again.Updated) != 0 {
		t.Fatalf("second run = %+v, %v", again, err)
	}
	if _, err := Generate(p, GenerateOptions{Check: true}); err != nil {
		t.Fatalf("check on fresh files: %v", err)
	}

	// A new field: --check fails without writing, then the region is replaced in place.
	edited := strings.Replace(orderModel, "\tCached    string", "\tTotal int64 `db:\"total\"`\n\tCached    string", 1)
	if err := os.WriteFile(filepath.Join(p.Root, "internal", "order", "order.go"), []byte(edited), 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Generate(p, GenerateOptions{Check: true}); err == nil {
		t.Fatal("check must fail when the struct changed")
	}
	if _, err := Generate(p, GenerateOptions{}); err != nil {
		t.Fatal(err)
	}
	src = readFile(t, p, "internal/order/repository.go")
	if strings.Count(src, "// gortisan:table Order begin") != 1 || !strings.Contains(src, "&m.Total,") {
		t.Fatalf("the region was not replaced in place:\n%s", src)
	}

	// The model loses its directive: its region is removed.
	plain := strings.Replace(edited, "//gortisan:table shop_orders\n", "", 1)
	if err := os.WriteFile(filepath.Join(p.Root, "internal", "order", "order.go"), []byte(plain), 0o644); err != nil {
		t.Fatal(err)
	}
	removed, err := Generate(p, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	src = readFile(t, p, "internal/order/repository.go")
	if strings.Contains(src, "gortisan:table") || len(removed.Next) != 1 {
		t.Fatalf("region not removed (%+v):\n%s", removed, src)
	}
}

func TestGenerateErrors(t *testing.T) {
	bad := map[string]string{
		"no pk":          "type A struct {\n\tID int64 `db:\"id\"`\n}",
		"unknown option": "type A struct {\n\tID int64 `db:\"id,pk,unique\"`\n}",
		"shared tag":     "type A struct {\n\tID int64 `db:\"id,pk\"`\n\tX, Y int `db:\"x\"`\n}",
		"twice":          "type A struct {\n\tID int64 `db:\"id,pk\"`\n\tX int `db:\"x\"`\n\tY int `db:\"x\"`\n}",
		"bad column":     "type A struct {\n\tID int64 `db:\"Id,pk\"`\n}",
		"no columns":     "type A struct {\n\tID int64\n}",
	}
	for name, body := range bad {
		p := genProject(t, map[string]string{"internal/a/a.go": "package a\n\n//gortisan:table as\n" + body + "\n"})
		if _, err := Generate(p, GenerateOptions{}); err == nil {
			t.Errorf("%s: want an error", name)
		}
	}
}

func TestGenerateLeavesHandWrittenMappings(t *testing.T) {
	model := "package user\n\n//gortisan:table users\ntype User struct {\n\tID int64 `db:\"id,pk\"`\n}\n"
	repo := "package user\n\nvar userTable = 1\n"
	p := genProject(t, map[string]string{"internal/user/user.go": model, "internal/user/repository.go": repo})
	report, err := Generate(p, GenerateOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if readFile(t, p, "internal/user/repository.go") != repo || len(report.Next) != 1 || !strings.Contains(report.Next[0], "written by hand") {
		t.Fatalf("a hand-written mapping must be left alone and reported: %+v", report)
	}
}

func TestAppendGo(t *testing.T) {
	existing := "package user\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/labstack/echo/v4\"\n)\n\n// A is kept.\nfunc A() { fmt.Println(echo.GET) }\n"
	fragment := "package user\n\nimport (\n\t\"context\"\n\t\"net/http\"\n\n\t\"example.com/x/internal/validate\"\n\t\"github.com/labstack/echo/v4\"\n)\n\n// B is new.\nfunc B(ctx context.Context) error { _ = validate.Errors{}; return echo.NewHTTPError(http.StatusTeapot) }\n"
	out, err := appendGo([]byte(existing), []byte(fragment))
	if err != nil {
		t.Fatal(err)
	}
	want := "package user\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\t\"net/http\"\n\n\t\"example.com/x/internal/validate\"\n\t\"github.com/labstack/echo/v4\"\n)\n\n// A is kept.\nfunc A() { fmt.Println(echo.GET) }\n\n// B is new.\nfunc B(ctx context.Context) error { _ = validate.Errors{}; return echo.NewHTTPError(http.StatusTeapot) }\n"
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	if _, err := appendGo(out, []byte(fragment)); !errors.Is(err, errDeclExists) {
		t.Fatalf("appending B twice: %v", err)
	}
	if _, err := appendGo(out, []byte("package other\n\nfunc C() {}\n")); err == nil {
		t.Fatal("a fragment of another package must be rejected")
	}
	noImports, err := appendGo([]byte("package user\n"), []byte(fragment))
	if err != nil || !strings.Contains(string(noImports), "import (\n\t\"context\"\n\t\"net/http\"\n\n\t\"example.com/x/internal/validate\"") {
		t.Fatalf("a file without imports gets a grouped block:\n%s\n%v", noImports, err)
	}
}
