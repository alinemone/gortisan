package generator

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"
)

// newProject creates a project in a temporary directory, without go mod
// tidy, so the test needs no network.
func newProject(t *testing.T, with string) *Project {
	t.Helper()
	dir := t.TempDir()
	if _, err := Create(dir, "svc", CreateOptions{Module: "example.com/svc", With: with, NoTidy: true}); err != nil {
		t.Fatal(err)
	}
	p, err := FindProject(filepath.Join(dir, "svc"))
	if err != nil {
		t.Fatal(err)
	}
	return p
}

func read(t *testing.T, p *Project, rel string) string {
	t.Helper()
	b, err := os.ReadFile(filepath.Join(p.Root, filepath.FromSlash(rel)))
	if err != nil {
		t.Fatal(err)
	}
	return string(b)
}

func mustMake(t *testing.T, p *Project, kind, name string, opts MakeOptions) *Report {
	t.Helper()
	report, err := Make(p, kind, name, opts)
	if err != nil {
		t.Fatalf("make:%s %s: %v", kind, name, err)
	}
	return report
}

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
		got = append(got, f.GoName+" "+f.GoType+" "+f.SQLType+" "+f.Tag)
	}
	want := "UserID int64 BIGINT user_id|PaidAt *time.Time TIMESTAMPTZ paid_at|Meta json.RawMessage JSONB meta,type:jsonb,nullzero"
	if strings.Join(got, "|") != want {
		t.Errorf("got %s\nwant %s", strings.Join(got, "|"), want)
	}
	for _, bad := range []string{"id:int64", "a:money", "a", "a:int,a:int", "1a:int"} {
		if _, err := parseFields(bad); err == nil {
			t.Errorf("parseFields(%q): want an error", bad)
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

func TestParseFeature(t *testing.T) {
	f, item, err := splitItem("enum", "sales/order_item/status")
	if err != nil || f.Path != "sales/orderitem" || f.Pkg != "orderitem" || f.Parent != "sales" || f.Type != "OrderItem" || item != "status" {
		t.Fatalf("splitItem: %+v %q %v", f, item, err)
	}
	if f.Dir() != "internal/sales/orderitem" || f.Label() != "sales.orderitem" || f.Route() != "/sales/order-items" {
		t.Fatalf("dir %q label %q route %q", f.Dir(), f.Label(), f.Route())
	}
	for _, bad := range []string{"", "server", "platform/x", "app", "type", "1x"} {
		if _, err := parseFeature(bad); err == nil {
			t.Errorf("parseFeature(%q): want an error", bad)
		}
	}
}

func TestRouteFor(t *testing.T) {
	for action, want := range map[string]string{
		"List": "GET ", "Get": "GET /:id", "Create": "POST ",
		"Update": "PUT /:id", "Delete": "DELETE /:id", "ResetPassword": "GET /reset-password",
	} {
		if m, p := routeFor(action); m+" "+p != want {
			t.Errorf("routeFor(%q) = %q, want %q", action, m+" "+p, want)
		}
	}
}

func TestCreateWritesSettings(t *testing.T) {
	p := newProject(t, "postgres,redis,swagger")
	if p.Settings == nil || p.Settings.Layout != Layout || p.Settings.BasePath != "/api" {
		t.Fatalf("settings = %+v", p.Settings)
	}
	if !slices.Equal(p.Settings.With, []string{"postgres", "redis", "swagger"}) {
		t.Fatalf("with = %v", p.Settings.With)
	}
	if !p.Has("postgres") || p.Has("migrations") {
		t.Fatal("Has does not follow .gortisan.json")
	}
	for _, rel := range []string{"internal/migrations", "cmd/migrate.go", "internal/app/migrate.go"} {
		if p.exists(rel) {
			t.Errorf("%s written without migrations", rel)
		}
	}
	if strings.Contains(read(t, p, "cmd/root.go"), "migrateCommand") {
		t.Error("cmd/root.go lists migrate without migrations")
	}

	// A project without .gortisan.json, or with a newer layout, is refused.
	p.Settings = nil
	if err := p.Check(); err == nil {
		t.Error("a project without .gortisan.json must be refused")
	}
	p.Settings = &Settings{Layout: Layout + 1}
	if err := p.Check(); err == nil || !strings.Contains(err.Error(), "newer") {
		t.Errorf("a newer layout: err = %v", err)
	}
	if _, err := Create(t.TempDir(), "x", CreateOptions{With: "migrations", NoTidy: true}); err == nil {
		t.Error("migrations without postgres must be refused")
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
		for _, name := range scripts {
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
	}
}

func TestUpgrade(t *testing.T) {
	p := newProject(t, "none")
	report, err := Upgrade(p, "v1.4.0")
	if err != nil {
		t.Fatal(err)
	}
	for _, name := range scripts {
		if !strings.Contains(read(t, p, name), ToolPackage+"@v1.4.0") {
			t.Errorf("%s still pins the old version:\n%s", name, read(t, p, name))
		}
	}
	again, _ := FindProject(p.Root)
	if again.Settings.Version != "v1.4.0" || len(report.Updated) != 4 {
		t.Fatalf("settings %+v, report %+v", again.Settings, report)
	}
	if _, err := Upgrade(p, "1.4"); err == nil {
		t.Error("a version without v must be refused")
	}
}

func TestFeatureIsWired(t *testing.T) {
	p := newProject(t, "")
	report := mustMake(t, p, "feature", "user", MakeOptions{Fields: "mobile:string", SoftDelete: true})
	for _, rel := range []string{"user.go", "service.go", "postgres.go", "handler.go", "dto.go", "routes.go", "service_test.go"} {
		if !p.exists("internal/user/" + rel) {
			t.Errorf("internal/user/%s was not written", rel)
		}
	}
	migration := ""
	for _, c := range report.Created {
		if strings.HasSuffix(c, "_create_users.sql") {
			migration = c
		}
	}
	if migration == "" || !strings.Contains(read(t, p, migration), "deleted_at TIMESTAMPTZ") {
		t.Fatalf("no create-table migration with deleted_at in %v", report.Created)
	}
	serve := read(t, p, appServe)
	for _, want := range []string{`"example.com/svc/internal/user"`, "userService := user.NewService(user.NewPostgres(db))", "user.RegisterRoutes(api, user.NewHandler(userService))"} {
		if !strings.Contains(serve, want) {
			t.Errorf("%s lacks %q:\n%s", appServe, want, serve)
		}
	}

	mustMake(t, p, "handler", "user/list", MakeOptions{})
	mustMake(t, p, "handler", "user/get", MakeOptions{})
	routes := read(t, p, "internal/user/routes.go")
	for _, want := range []string{`g.GET("", h.List)`, `g.GET("/:id", h.Get)`} {
		if !strings.Contains(routes, want) {
			t.Errorf("routes.go lacks %q:\n%s", want, routes)
		}
	}
	if strings.Contains(routes, "_ = g") {
		t.Errorf("routes.go kept its placeholder:\n%s", routes)
	}
	handler := read(t, p, "internal/user/handler.go")
	for _, want := range []string{"func (h *Handler) List(", "@Router\t\t/users [get]", "@Router\t\t/users/{id} [get]", "@Param\t\tid\tpath"} {
		if !strings.Contains(handler, want) {
			t.Errorf("handler.go lacks %q:\n%s", want, handler)
		}
	}
	dto := read(t, p, "internal/user/dto.go")
	for _, want := range []string{"type ListRequest struct", "type GetResponse struct"} {
		if !strings.Contains(dto, want) {
			t.Errorf("dto.go lacks %q", want)
		}
	}

	// Making the same handler again changes nothing and says so.
	before := read(t, p, "internal/user/handler.go") + read(t, p, "internal/user/routes.go")
	again := mustMake(t, p, "handler", "user/list", MakeOptions{})
	if read(t, p, "internal/user/handler.go")+read(t, p, "internal/user/routes.go") != before || len(again.Skipped) != 3 {
		t.Fatalf("a second make:handler user/list changed files or did not report them: %+v", again)
	}
	// A feature is not made twice without --force.
	if _, err := Make(p, "feature", "user", MakeOptions{}); err == nil {
		t.Error("make:feature on an existing feature must be refused")
	}

	found, err := Routes(p)
	if err != nil {
		t.Fatal(err)
	}
	var got []string
	for _, r := range found {
		got = append(got, r.Method+" "+r.Path+" "+r.Handler)
	}
	for _, want := range []string{"GET /ping server.Health.Ping", "GET /swagger/* echoSwagger.WrapHandler", "GET /api/users user.Handler.List", "GET /api/users/:id user.Handler.Get"} {
		if !slices.Contains(got, want) {
			t.Errorf("route:list lacks %q: %v", want, got)
		}
	}
}

func TestMakeNeedsItsComponent(t *testing.T) {
	p := newProject(t, "none")
	for kind, name := range map[string]string{"cache": "user/count", "migration": "add_index"} {
		if _, err := Make(p, kind, name, MakeOptions{}); err == nil || !strings.Contains(err.Error(), "./gortisan add") {
			t.Errorf("make:%s without its component: err = %v", kind, err)
		}
	}
	// Without postgres a feature has no storage and is wired with a nil Repository.
	report := mustMake(t, p, "feature", "note", MakeOptions{})
	if p.exists("internal/note/postgres.go") || !strings.Contains(read(t, p, appServe), "note.NewService(nil") {
		t.Errorf("a feature without postgres: %+v", report)
	}
}

func TestMigrationsInTheSameSecondGetDistinctVersions(t *testing.T) {
	p := newProject(t, "postgres,migrations")
	now := time.Date(2026, 1, 2, 3, 4, 5, 0, time.UTC)
	for _, name := range []string{"first", "second", "third"} {
		mustMake(t, p, "migration", name, MakeOptions{Now: now})
	}
	files, _ := filepath.Glob(filepath.Join(p.Root, "internal", "migrations", "*.sql"))
	seen := map[string]bool{}
	for _, f := range files {
		version, _, _ := strings.Cut(filepath.Base(f), "_")
		if seen[version] {
			t.Fatalf("version %s used twice: %v", version, files)
		}
		seen[version] = true
	}
	if len(seen) != 4 { // the three, and the first migration create writes
		t.Fatalf("want 4 versions, got %v", files)
	}
}

func TestMakeKeepsExistingFiles(t *testing.T) {
	p := newProject(t, "none")
	mustMake(t, p, "middleware", "request_id", MakeOptions{})
	path := filepath.Join(p.Root, "internal", "server", "middleware", "request_id.go")
	if err := os.WriteFile(path, []byte("package middleware // edited\n"), 0o644); err != nil {
		t.Fatal(err)
	}
	report := mustMake(t, p, "middleware", "request_id", MakeOptions{})
	if len(report.Skipped) != 1 || len(report.Created) != 0 {
		t.Fatalf("report = %+v, want the file skipped", report)
	}
	if content, _ := os.ReadFile(path); !strings.Contains(string(content), "edited") {
		t.Fatal("make overwrote an edited file without --force")
	}
	mustMake(t, p, "middleware", "request_id", MakeOptions{Force: true})
	if content, _ := os.ReadFile(path); strings.Contains(string(content), "edited") {
		t.Fatal("--force did not overwrite")
	}
}

func TestMakeValidator(t *testing.T) {
	p := newProject(t, "none")
	for _, name := range []string{"address/mobile", "mobile", "sales/order/max_weight", "address/mobile"} {
		mustMake(t, p, "validator", name, MakeOptions{})
	}
	for file, want := range map[string]string{
		"internal/server/validator/address/mobile.go":         `package address`,
		"internal/server/validator/mobile.go":                 `package validator`,
		"internal/server/validator/sales/order/max_weight.go": `Code: "sales.order:max-weight"`,
	} {
		if !strings.Contains(read(t, p, file), want) {
			t.Errorf("%s: want %q", file, want)
		}
	}
	src := read(t, p, langPath)
	var phrases map[string]string
	if err := json.Unmarshal([]byte(src), &phrases); err != nil {
		t.Fatalf("fa.json is not valid JSON: %v\n%s", err, src)
	}
	for _, key := range []string{"general:invalid-data", "address:mobile", "validation:mobile", "sales.order:max-weight"} {
		if phrases[key] == "" {
			t.Errorf("fa.json has no %q:\n%s", key, src)
		}
	}
	if n := strings.Count(src, `"address:mobile"`); n != 1 {
		t.Errorf("address:mobile is in fa.json %d times", n)
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

// TestGeneratedProjectsBuild creates a project for several --with choices,
// adds every kind of file, and checks that it builds, vets and passes its
// tests — the dependency rules included — without importing gortisan. It
// downloads modules, so -short skips it.
func TestGeneratedProjectsBuild(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads modules")
	}
	for _, with := range []string{"all", "postgres,redis,swagger", "postgres,migrations", "redis", "swagger", "none"} {
		t.Run(with, func(t *testing.T) {
			dir := t.TempDir()
			if _, err := Create(dir, "demo-svc", CreateOptions{Module: "example.com/demo-svc", With: with}); err != nil {
				t.Fatal(err)
			}
			p, err := FindProject(filepath.Join(dir, "demo-svc", "internal"))
			if err != nil {
				t.Fatal(err)
			}
			type step struct {
				kind, name string
				opts       MakeOptions
			}
			steps := []step{
				{kind: "feature", name: "order", opts: MakeOptions{Fields: "user_id:int64,status:string,paid_at:time?,meta:json", SoftDelete: true}},
				{kind: "handler", name: "order/list"},
				{kind: "handler", name: "order/get"},
				{kind: "handler", name: "order/create"},
				{kind: "handler", name: "order/mark_paid"},
				{kind: "test", name: "order"},
				{kind: "test", name: "order/get"},
				{kind: "feature", name: "sales/payment"}, // a grouped feature
				{kind: "handler", name: "sales/payment/get"},
				{kind: "enum", name: "order/status", opts: MakeOptions{Members: "pending,paid=5,sent"}},
				{kind: "enum", name: "order/currency", opts: MakeOptions{Type: "string", Members: "irr,usd=USD"}},
				{kind: "validator", name: "address/mobile"},
				{kind: "validator", name: "mobile"},
				{kind: "middleware", name: "request_id"},
				{kind: "client", name: "wallet"},
				{kind: "external", name: "bale-messenger"},
			}
			if p.Has("redis") {
				steps = append(steps, step{kind: "cache", name: "order/count", opts: MakeOptions{Type: "int64"}},
					step{kind: "cache", name: "order/detail", opts: MakeOptions{Type: "*Order"}})
			}
			if p.Has("migrations") {
				steps = append(steps, step{kind: "migration", name: "add_index_to_orders"})
			}
			for _, s := range steps {
				mustMake(t, p, s.kind, s.name, s.opts)
			}
			if _, err := Routes(p); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"mod", "tidy"}, {"vet", "./..."}, {"test", "./..."}} {
				cmd := exec.Command("go", args...)
				cmd.Dir = p.Root
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}
			assertNoGortisan(t, p.Root)
			if p.Has("postgres") {
				assertORMStandalone(t, p)
			}
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

// assertORMStandalone checks that internal/platform/orm could become a
// module of its own: it imports nothing of the project and only bun, pgx and
// the standard library from outside it.
func assertORMStandalone(t *testing.T, p *Project) {
	t.Helper()
	cmd := exec.Command("go", "list", "-f", "{{range .Imports}}{{.}} {{end}}", "./"+ormDir)
	cmd.Dir = p.Root
	out, err := cmd.Output()
	if err != nil {
		t.Fatal(err)
	}
	for _, imp := range strings.Fields(string(out)) {
		first, _, _ := strings.Cut(imp, "/")
		std := !strings.Contains(first, ".")
		allowed := strings.HasPrefix(imp, "github.com/uptrace/bun") || strings.HasPrefix(imp, "github.com/jackc/pgx/v5")
		if (!std && !allowed) || strings.HasPrefix(imp, p.Module+"/") {
			t.Errorf("%s imports %s; it may import only bun, pgx and the standard library", ormDir, imp)
		}
	}
}

// TestAddComponent adds every component to a project created with none. It
// runs go get and go mod tidy, so -short skips it.
func TestAddComponent(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads modules")
	}
	dir := t.TempDir()
	if _, err := Create(dir, "svc", CreateOptions{Module: "example.com/svc", With: "none"}); err != nil {
		t.Fatal(err)
	}
	p, err := FindProject(filepath.Join(dir, "svc"))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := Add(p, "migrations"); err == nil {
		t.Error("migrations before postgres must be refused")
	}
	for _, component := range []string{"redis", "postgres", "migrations", "swagger"} {
		report, err := Add(p, component)
		if err != nil {
			t.Fatalf("add %s: %v", component, err)
		}
		if len(report.Next) == 0 {
			t.Errorf("add %s printed no wiring hints", component)
		}
		if _, err := Add(p, component); err == nil {
			t.Errorf("adding %s twice: want an error", component)
		}
		// Do what the printed hint asks for config.go, as a user would.
		if field := map[string]string{"redis": "Redis", "postgres": "Postgres"}[component]; field != "" {
			config := read(t, p, "internal/config/config.go")
			config = strings.Replace(config, "type Config struct {", "type Config struct {\n\t"+field+" "+field, 1)
			config = strings.Replace(config, "cfg := Config{", "cfg := Config{\n\t\t"+field+": r."+component+"(),", 1)
			if err := os.WriteFile(filepath.Join(p.Root, "internal", "config", "config.go"), []byte(config), 0o644); err != nil {
				t.Fatal(err)
			}
		}
	}
	again, _ := FindProject(p.Root)
	if !slices.Equal(again.Settings.With, Components) {
		t.Errorf(".gortisan.json with = %v", again.Settings.With)
	}
	for _, rel := range []string{"internal/platform/redis.go", "internal/config/redis.go", "internal/platform/orm/orm.go", "internal/config/postgres.go", "cmd/migrate.go", "docs/docs.go"} {
		if !p.exists(rel) {
			t.Errorf("%s was not written", rel)
		}
	}
	env := read(t, p, ".env.example")
	if !strings.Contains(env, "POSTGRES_HOST") || !strings.Contains(env, "REDIS_ADDR") || !strings.Contains(env, "RUN_MIGRATIONS") {
		t.Errorf(".env.example lacks the settings:\n%s", env)
	}
	// The project still builds before the hinted lines are added by hand.
	cmd := exec.Command("go", "vet", "./...")
	cmd.Dir = p.Root
	if out, err := cmd.CombinedOutput(); err != nil {
		t.Fatalf("go vet ./...: %v\n%s", err, out)
	}
	if _, err := Add(p, "kafka"); err == nil {
		t.Error("an unknown component must be rejected")
	}
}
