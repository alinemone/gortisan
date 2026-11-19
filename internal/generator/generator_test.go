package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"
)

func TestNaming(t *testing.T) {
	cases := []struct{ in, snake, pascal, camel string }{
		{"order_item", "order_item", "OrderItem", "orderItem"},
		{"GetOrder", "get_order", "GetOrder", "getOrder"},
		{"user_id", "user_id", "UserID", "userID"},
		{"id", "id", "ID", "id"},
		{"HTTPClient", "http_client", "HTTPClient", "httpClient"},
		{"get-order", "get_order", "GetOrder", "getOrder"},
	}
	for _, c := range cases {
		if got := snake(c.in); got != c.snake {
			t.Errorf("snake(%q) = %q, want %q", c.in, got, c.snake)
		}
		if got := pascal(c.in); got != c.pascal {
			t.Errorf("pascal(%q) = %q, want %q", c.in, got, c.pascal)
		}
		if got := camel(c.in); got != c.camel {
			t.Errorf("camel(%q) = %q, want %q", c.in, got, c.camel)
		}
	}
	for in, want := range map[string]string{"order": "orders", "category": "categories", "box": "boxes", "key": "keys", "status": "statuses"} {
		if got := plural(in); got != want {
			t.Errorf("plural(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestParseFields(t *testing.T) {
	fields, err := parseFields("user_id:int64, paid_at:time?, meta:json?")
	if err != nil {
		t.Fatal(err)
	}
	got := []string{}
	for _, f := range fields {
		got = append(got, f.GoName+" "+f.GoType+" "+f.SQLType)
	}
	want := "UserID int64 BIGINT|PaidAt *time.Time TIMESTAMPTZ|Meta json.RawMessage JSONB"
	if strings.Join(got, "|") != want {
		t.Errorf("got %s\nwant %s", strings.Join(got, "|"), want)
	}
	for _, bad := range []string{"id:int64", "a:money", "a", "a:int,a:int", "1a:int"} {
		if _, err := parseFields(bad); err == nil {
			t.Errorf("parseFields(%q): want an error", bad)
		}
	}
}

// TestGeneratedProjectsBuild creates a project for every --with choice, adds
// every kind of file, and checks that it builds, vets and passes its tests
// without importing gortisan. It downloads modules, so -short skips it.
func TestGeneratedProjectsBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads modules")
	}
	for _, with := range []string{"all", "postgres", "redis", "none"} {
		t.Run(with, func(t *testing.T) {
			dir := t.TempDir()
			if _, err := Create(dir, "demo-svc", CreateOptions{Module: "example.com/demo-svc", With: with}); err != nil {
				t.Fatal(err)
			}
			project, err := FindProject(filepath.Join(dir, "demo-svc", "internal"))
			if err != nil {
				t.Fatal(err)
			}
			if project.Module != "example.com/demo-svc" {
				t.Fatalf("module = %q", project.Module)
			}
			now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			makes := [][2]string{{"handler", "order/get_order"}, {"handler", "user/list"}, {"service", "order"}}
			if with == "all" || with == "postgres" {
				makes = append(makes, [2]string{"model", "order"}, [2]string{"model", "tag"}, [2]string{"migration", "add_index"})
			}
			for _, m := range makes {
				opts := MakeOptions{Now: now}
				if m[0] == "model" && m[1] == "order" {
					opts.Fields, opts.SoftDelete = "user_id:int64,status:string,paid_at:time?,meta:json", true
				}
				if _, err := Make(project, m[0], m[1], opts); err != nil {
					t.Fatalf("make %s %s: %v", m[0], m[1], err)
				}
			}
			if with == "redis" || with == "none" {
				if _, err := Make(project, "model", "order", MakeOptions{}); err == nil {
					t.Error("make model without postgres: want an error")
				}
			}

			for _, args := range [][]string{{"mod", "tidy"}, {"vet", "./..."}, {"test", "./..."}} {
				cmd := exec.Command("go", args...)
				cmd.Dir = project.Root
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}
			assertNoGortisan(t, project.Root)
		})
	}
}

func assertNoGortisan(t *testing.T, root string) {
	t.Helper()
	gomod, err := os.ReadFile(filepath.Join(root, "go.mod"))
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(gomod), "gortisan") {
		t.Errorf("go.mod mentions gortisan:\n%s", gomod)
	}
	cmd := exec.Command("go", "list", "-deps", "./...")
	cmd.Dir = root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(string(out), "gortisan") {
		t.Errorf("the project depends on gortisan:\n%s", out)
	}
}

func TestMakeKeepsExistingFiles(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, "go.mod"), []byte("module example.com/x\n\ngo 1.26\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	project, err := FindProject(root)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Make(project, "service", "order", MakeOptions{}); err != nil {
		t.Fatal(err)
	}
	path := filepath.Join(root, "internal", "service", "order_service.go")
	if err := os.WriteFile(path, []byte("package service // edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report, err := Make(project, "service", "order", MakeOptions{})
	if err != nil {
		t.Fatal(err)
	}
	if len(report.Skipped) != 1 || len(report.Created) != 0 {
		t.Fatalf("report = %+v, want the file skipped", report)
	}
	if content, _ := os.ReadFile(path); !strings.Contains(string(content), "edited") {
		t.Fatal("make overwrote an edited file without --force")
	}
	if _, err := Make(project, "service", "order", MakeOptions{Force: true}); err != nil {
		t.Fatal(err)
	}
	if content, _ := os.ReadFile(path); strings.Contains(string(content), "edited") {
		t.Fatal("--force did not overwrite")
	}
}

func TestCreateWritesPinnedWrappers(t *testing.T) {
	for version, want := range map[string]string{
		"v0.2.0":   ToolPackage + "@v0.2.0",
		"(devel)":  ToolPackage + "@latest",
		"":         ToolPackage + "@latest",
		"v1+dirty": ToolPackage + "@latest",
	} {
		dir := t.TempDir()
		if _, err := Create(dir, "svc", CreateOptions{With: "none", NoTidy: true, Version: version}); err != nil {
			t.Fatal(err)
		}
		for _, name := range []string{"gortisan", "gortisan.bat", "gortisan.ps1"} {
			content, err := os.ReadFile(filepath.Join(dir, "svc", name))
			if err != nil {
				t.Fatal(err)
			}
			if !strings.Contains(string(content), "go run "+want) {
				t.Errorf("version %q: %s does not run %s:\n%s", version, name, want, content)
			}
		}
		sh, _ := os.ReadFile(filepath.Join(dir, "svc", "gortisan"))
		if strings.Contains(string(sh), "\r") || !strings.HasPrefix(string(sh), "#!/usr/bin/env sh\n") {
			t.Errorf("the sh script must use LF line endings:\n%q", sh)
		}
		bat, _ := os.ReadFile(filepath.Join(dir, "svc", "gortisan.bat"))
		if !strings.Contains(string(bat), "\r\n") {
			t.Errorf("the bat script should use CRLF line endings:\n%q", bat)
		}
	}
}

func TestCreateRefusesNonEmptyDir(t *testing.T) {
	dir := t.TempDir()
	if err := os.MkdirAll(filepath.Join(dir, "svc"), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(dir, "svc", "keep.txt"), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if _, err := Create(dir, "svc", CreateOptions{NoTidy: true}); err == nil {
		t.Fatal("want an error for a non-empty directory")
	}
}
