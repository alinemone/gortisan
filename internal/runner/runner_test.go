package runner

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

func write(t *testing.T, path, content string) {
	t.Helper()
	if err := os.MkdirAll(filepath.Dir(path), 0o755); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(path, []byte(content), 0o644); err != nil {
		t.Fatal(err)
	}
}

func TestResolveTarget(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "cmd", "shop", "main.go"), "package main")
	write(t, filepath.Join(root, "cmd", "worker", "main.go"), "package main")

	if got, err := ResolveTarget(root, "example.com/shop", ""); err != nil || got != "shop" {
		t.Fatalf("got %q, %v; want shop (worker is ignored)", got, err)
	}
	if got, err := ResolveTarget(root, "example.com/shop", "worker"); err != nil || got != "worker" {
		t.Fatalf("got %q, %v; want worker", got, err)
	}
	if _, err := ResolveTarget(root, "example.com/shop", "nope"); err == nil {
		t.Fatal("want an error for a missing command")
	}

	write(t, filepath.Join(root, "cmd", "admin", "main.go"), "package main")
	if got, err := ResolveTarget(root, "example.com/shop", ""); err != nil || got != "shop" {
		t.Fatalf("got %q, %v; want the command named after the module", got, err)
	}
	if _, err := ResolveTarget(root, "example.com/other", ""); err == nil {
		t.Fatal("want an error when several commands match nothing")
	}
}

func TestSnapshotSeesSourceChangesOnly(t *testing.T) {
	root := t.TempDir()
	write(t, filepath.Join(root, "main.go"), "package main")
	write(t, filepath.Join(root, "README.md"), "x")
	write(t, filepath.Join(root, ".git", "x.go"), "x")
	write(t, filepath.Join(root, "migrations", "1.up.sql"), "x")

	before, err := snapshot(root)
	if err != nil {
		t.Fatal(err)
	}
	if len(before) != 1 {
		t.Fatalf("watched %v, want only main.go", before)
	}

	write(t, filepath.Join(root, "README.md"), "changed")
	after, _ := snapshot(root)
	if !sameFiles(before, after) {
		t.Fatal("a README change must not restart")
	}

	write(t, filepath.Join(root, ".env"), "PORT=1")
	after, _ = snapshot(root)
	if sameFiles(before, after) {
		t.Fatal("a new .env must restart")
	}

	before = after
	later := time.Now().Add(2 * time.Second)
	if err := os.Chtimes(filepath.Join(root, "main.go"), later, later); err != nil {
		t.Fatal(err)
	}
	after, _ = snapshot(root)
	if sameFiles(before, after) {
		t.Fatal("an edited .go file must restart")
	}
}

func TestHasRootCommand(t *testing.T) {
	root := t.TempDir()
	if HasRootCommand(root) {
		t.Fatal("no main.go: want false")
	}
	write(t, filepath.Join(root, "main.go"), "package main\n\nfunc main() {}\n")
	if !HasRootCommand(root) {
		t.Fatal("a main package in the root: want true")
	}
	if got := (Options{Target: ".", Args: []string{"serve"}}).label(); got != "go run . serve" {
		t.Fatalf("label = %q", got)
	}
}
