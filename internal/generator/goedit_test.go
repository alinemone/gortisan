package generator

import (
	"errors"
	"strings"
	"testing"
)

func TestAppendGo(t *testing.T) {
	existing := "package user\n\nimport (\n\t\"fmt\"\n\n\t\"github.com/labstack/echo/v5\"\n)\n\n// A is kept.\nfunc A() { fmt.Println(echo.GET) }\n"
	fragment := "package user\n\nimport (\n\t\"context\"\n\t\"net/http\"\n\n\t\"example.com/x/internal/validate\"\n\t\"github.com/labstack/echo/v5\"\n)\n\n// B is new.\nfunc B(ctx context.Context) error { _ = validate.Errors{}; return echo.NewHTTPError(http.StatusTeapot) }\n"
	out, err := appendGo([]byte(existing), []byte(fragment), "example.com/x")
	if err != nil {
		t.Fatal(err)
	}
	want := "package user\n\nimport (\n\t\"context\"\n\t\"fmt\"\n\t\"net/http\"\n\n\t\"example.com/x/internal/validate\"\n\t\"github.com/labstack/echo/v5\"\n)\n\n// A is kept.\nfunc A() { fmt.Println(echo.GET) }\n\n// B is new.\nfunc B(ctx context.Context) error { _ = validate.Errors{}; return echo.NewHTTPError(http.StatusTeapot) }\n"
	if string(out) != want {
		t.Errorf("got:\n%s\nwant:\n%s", out, want)
	}
	if _, err := appendGo(out, []byte(fragment), "example.com/x"); !errors.Is(err, errDeclExists) {
		t.Fatalf("appending B twice: %v", err)
	}
	if _, err := appendGo(out, []byte("package other\n\nfunc C() {}\n"), "example.com/x"); err == nil {
		t.Fatal("a fragment of another package must be rejected")
	}
	noImports, err := appendGo([]byte("package user\n"), []byte(fragment), "example.com/x")
	if err != nil || !strings.Contains(string(noImports), "import (\n\t\"context\"\n\t\"net/http\"\n\n\t\"example.com/x/internal/validate\"") {
		t.Fatalf("a file without imports gets a grouped block:\n%s\n%v", noImports, err)
	}
}

func TestAppendGoMergesASingleImport(t *testing.T) {
	// "shop" has no dot, like a standard library path; it must still be grouped
	// with the non-standard imports.
	existing := "package users\n\nimport \"shop/internal/domain/user\"\n\ntype Handler struct{ s *user.Service }\n"
	fragment := "package users\n\nimport (\n\t\"net/http\"\n\n\t\"github.com/labstack/echo/v5\"\n)\n\nfunc (h *Handler) List(c *echo.Context) error { return c.NoContent(http.StatusOK) }\n"
	out, err := appendGo([]byte(existing), []byte(fragment), "shop")
	if err != nil {
		t.Fatal(err)
	}
	want := "import (\n\t\"net/http\"\n\n\t\"github.com/labstack/echo/v5\"\n\t\"shop/internal/domain/user\"\n)"
	if !strings.Contains(string(out), want) || strings.Count(string(out), "import") != 1 {
		t.Fatalf("want one grouped import block, got:\n%s", out)
	}
}
