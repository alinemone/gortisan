package generator

import (
	"io/fs"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"
)

// compatDir holds one project per layout, created by the first release of
// that layout and never regenerated: it is what users have on disk. See
// COMPATIBILITY.md.
const compatDir = "testdata/compat"

// TestWriteCompatFixture writes the project of the current layout into
// testdata/compat/layout<N>, once, when GORTISAN_WRITE_COMPAT=1. It refuses
// to overwrite an existing one: that would hide a break.
func TestWriteCompatFixture(t *testing.T) {
	if os.Getenv("GORTISAN_WRITE_COMPAT") != "1" {
		t.Skip("set GORTISAN_WRITE_COMPAT=1 to write the fixture of a new layout")
	}
	dir, err := filepath.Abs(filepath.Join(compatDir, "layout"+strconv.Itoa(Layout)))
	if err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(dir); err == nil {
		t.Fatalf("%s exists; a fixture is written once per layout and never replaced", dir)
	}
	now := time.Date(2026, 11, 21, 12, 0, 0, 0, time.UTC)
	if _, err := Create(filepath.Dir(dir), filepath.Base(dir), CreateOptions{Module: "example.com/compat", NoTidy: true, Version: "v0.1.0", Now: now}); err != nil {
		t.Fatal(err)
	}
	p, err := FindProject(dir)
	if err != nil {
		t.Fatal(err)
	}
	mustMake(t, p, "feature", "user", MakeOptions{Fields: "mobile:string,name:string", SoftDelete: true, Now: now.Add(time.Minute)})
	mustMake(t, p, "handler", "user/list", MakeOptions{})
}

// TestCompatibility runs the current gortisan on every layout fixture: each
// make command must still write code that builds and passes its tests. It
// downloads modules, so -short skips it.
func TestCompatibility(t *testing.T) {
	if testing.Short() {
		t.Skip("downloads modules")
	}
	fixtures, _ := filepath.Glob(filepath.Join(compatDir, "layout*"))
	if len(fixtures) == 0 {
		t.Fatal("no fixture in " + compatDir)
	}
	for _, fixture := range fixtures {
		t.Run(filepath.Base(fixture), func(t *testing.T) {
			root := filepath.Join(t.TempDir(), "compat")
			copyDir(t, fixture, root)
			p, err := FindProject(root)
			if err != nil {
				t.Fatal(err)
			}
			for _, s := range []struct {
				kind, name string
				opts       MakeOptions
			}{
				{"feature", "sales/order", MakeOptions{Fields: "amount:int64,paid_at:time?"}},
				{"handler", "user/get", MakeOptions{}},
				{"handler", "sales/order/create", MakeOptions{}},
				{"migration", "add_index_to_users", MakeOptions{}},
				{"enum", "user/status", MakeOptions{Members: "active,blocked"}},
				{"cache", "user/profile", MakeOptions{Type: "*User"}},
				{"validator", "address/mobile", MakeOptions{}},
				{"middleware", "request_id", MakeOptions{}},
				{"client", "wallet", MakeOptions{}},
				{"external", "bale", MakeOptions{}},
				{"test", "user/get", MakeOptions{}},
			} {
				mustMake(t, p, s.kind, s.name, s.opts)
			}
			if _, err := Routes(p); err != nil {
				t.Fatal(err)
			}
			for _, args := range [][]string{{"mod", "tidy"}, {"vet", "./..."}, {"test", "./..."}} {
				cmd := exec.Command("go", args...)
				cmd.Dir = root
				if out, err := cmd.CombinedOutput(); err != nil {
					t.Fatalf("go %s: %v\n%s", strings.Join(args, " "), err, out)
				}
			}
		})
	}
}

func copyDir(t *testing.T, from, to string) {
	t.Helper()
	err := filepath.WalkDir(from, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		rel, _ := filepath.Rel(from, path)
		target := filepath.Join(to, rel)
		if d.IsDir() {
			return os.MkdirAll(target, 0o755)
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		return os.WriteFile(target, data, 0o644)
	})
	if err != nil {
		t.Fatal(err)
	}
}
