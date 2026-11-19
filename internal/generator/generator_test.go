package generator

import (
	"os"
	"os/exec"
	"path/filepath"
	"slices"
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
	for _, with := range []string{"all", "", "postgres", "redis", "kafka", "none"} {
		name := with
		if name == "" {
			name = "default"
		}
		t.Run(name, func(t *testing.T) {
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
			has := func(component string) bool {
				switch with {
				case "all":
					return true
				case "":
					return slices.Contains(DefaultComponents, component)
				}
				return slices.Contains(strings.Split(with, ","), component)
			}

			now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
			type step struct {
				kind, name string
				opts       MakeOptions
			}
			steps := []step{
				{kind: "handler", name: "order/get_order"},
				{kind: "handler", name: "user/list"},
				{kind: "service", name: "order"},
				{kind: "middleware", name: "request_id"},
				{kind: "request", name: "order/create_order"},
				{kind: "request", name: "ping"},
				{kind: "response", name: "order/order_detail"},
				{kind: "validator", name: "order/not_paid"},
				{kind: "enum", name: "order_status", opts: MakeOptions{Members: "pending,paid=5,sent"}},
				{kind: "enum", name: "currency", opts: MakeOptions{Type: "string", Members: "irr,usd=USD"}},
				{kind: "client", name: "salam-pay"},
			}
			if has("postgres") {
				steps = append(steps,
					step{kind: "model", name: "order", opts: MakeOptions{Fields: "user_id:int64,status:string,paid_at:time?,meta:json", SoftDelete: true}},
					step{kind: "model", name: "tag"},
					step{kind: "migration", name: "add_index"},
				)
			}
			if has("redis") {
				steps = append(steps, step{kind: "cache", name: "vendor_order_count", opts: MakeOptions{Type: "int64"}})
				if has("postgres") {
					steps = append(steps, step{kind: "cache", name: "order", opts: MakeOptions{Type: "*model.Order"}})
				}
			}
			if has("kafka") {
				steps = append(steps,
					step{kind: "worker", name: "send_notifications", opts: MakeOptions{Group: "notification_worker"}},
					step{kind: "worker", name: "order_paid", opts: MakeOptions{Topic: "orders.paid"}},
				)
				if has("postgres") {
					steps = append(steps, step{kind: "cdc", name: "order"}, step{kind: "cdc", name: "tag"})
				}
			}
			for _, s := range steps {
				s.opts.Now = now
				if _, err := Make(project, s.kind, s.name, s.opts); err != nil {
					t.Fatalf("make %s %s: %v", s.kind, s.name, err)
				}
			}

			// Kinds that need a missing component must say so instead of writing broken code.
			missing := map[string]string{"model": "postgres", "cache": "redis", "worker": "kafka", "cdc": "kafka"}
			for kind, component := range missing {
				if !has(component) {
					if _, err := Make(project, kind, "order", MakeOptions{}); err == nil {
						t.Errorf("make %s without %s: want an error", kind, component)
					}
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

func TestParseMembers(t *testing.T) {
	members, err := parseMembers("pending, paid=5, sent, refunded=-1, void", false)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, m := range members {
		got = append(got, m.Name+"="+m.Value)
	}
	if want := "Pending=1 Paid=5 Sent=6 Refunded=-1 Void=0"; strings.Join(got, " ") != want {
		t.Errorf("got %s, want %s", strings.Join(got, " "), want)
	}
	strs, err := parseMembers("irr,usd=USD", true)
	if err != nil || strs[0].Value != `"irr"` || strs[1].Value != `"USD"` {
		t.Errorf("string members = %+v, %v", strs, err)
	}
	for _, bad := range []string{"", "a,a", "a=x", "1a"} {
		if _, err := parseMembers(bad, false); err == nil {
			t.Errorf("parseMembers(%q): want an error", bad)
		}
	}
}

func TestSplitName(t *testing.T) {
	cases := map[string][2]string{
		"order/create_order": {"create_order", "CreateOrder"},
		"user/list":          {"user_list", "UserList"},
		"ping":               {"ping", "Ping"},
	}
	for in, want := range cases {
		file, typeName, err := splitName("request", in)
		if err != nil || file != want[0] || typeName != want[1] {
			t.Errorf("splitName(%q) = %q, %q, %v; want %q, %q", in, file, typeName, err, want[0], want[1])
		}
	}
	if _, _, err := splitName("request", "a/b/c"); err == nil {
		t.Error("want an error for a/b/c")
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
