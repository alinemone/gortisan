// Package runner builds and runs a command of a project, rebuilding and
// restarting it when a source file changes — `gortisan run`.
//
// It polls file modification times instead of using OS file events, so it
// needs nothing outside the standard library and behaves the same on every
// platform. Nothing of it ends up in the project.
package runner

import (
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"
)

// Options configure Run.
type Options struct {
	// Root is the project root (the directory of go.mod).
	Root string
	// Target is what to build: "." for the root package (a project whose
	// main.go runs Cobra commands), or a directory under cmd/, e.g. "worker".
	Target string
	// Args are passed to the program.
	Args []string
	// Watch rebuilds and restarts on changes; without it Run runs once.
	Watch bool
	// Interval is how often files are checked; 500ms when zero.
	Interval time.Duration
	// Stdout and Stderr receive the program's output and the runner's messages.
	Stdout, Stderr io.Writer
	// BeforeBuild runs before every build, e.g. to regenerate code. An error
	// is reported like a failed build.
	BeforeBuild func() error
}

// label names what runs, for messages: "go run . serve" for the root
// package, "cmd/worker" for a command directory.
func (o Options) label() string {
	if o.Target == "." {
		return strings.TrimSpace("go run . " + strings.Join(o.Args, " "))
	}
	return "cmd/" + o.Target
}

// HasRootCommand reports whether the project's root package is the program:
// a main.go next to go.mod, as in projects whose commands are Cobra
// subcommands (serve, migrate, ...).
func HasRootCommand(root string) bool {
	src, err := os.ReadFile(filepath.Join(root, "main.go"))
	return err == nil && strings.Contains(string(src), "package main")
}

// ResolveTarget picks the command to run: name when given, otherwise api,
// the only directory under cmd/ that is not a helper (worker, migrate, seed),
// or the one named after the module.
func ResolveTarget(root, module, name string) (string, error) {
	if name != "" {
		if _, err := os.Stat(filepath.Join(root, "cmd", name)); err != nil {
			return "", fmt.Errorf("cmd/%s does not exist", name)
		}
		return name, nil
	}
	entries, err := os.ReadDir(filepath.Join(root, "cmd"))
	if err != nil {
		return "", errors.New("no cmd/ directory; pass the command to run")
	}
	var candidates []string
	for _, e := range entries {
		if e.IsDir() && !helperCommands[e.Name()] {
			candidates = append(candidates, e.Name())
		}
	}
	switch {
	case slices.Contains(candidates, "api"):
		return "api", nil
	case len(candidates) == 1:
		return candidates[0], nil
	case slices.Contains(candidates, path.Base(module)):
		return path.Base(module), nil
	case len(candidates) == 0:
		return "", errors.New("cmd/ has no command; pass the command to run")
	default:
		return "", fmt.Errorf("cmd/ has several commands (%s); pass one", strings.Join(candidates, ", "))
	}
}

// helperCommands are not what `gortisan run` starts by default.
var helperCommands = map[string]bool{"worker": true, "migrate": true, "seed": true}

// Run builds cmd/<Target> and runs it until ctx is done. With Watch it
// rebuilds and restarts the program whenever a watched file changes; a
// failed build is reported and retried on the next change.
func Run(ctx context.Context, o Options) error {
	if o.Interval == 0 {
		o.Interval = 500 * time.Millisecond
	}
	dir, err := os.MkdirTemp("", "gortisan-run-")
	if err != nil {
		return err
	}
	defer os.RemoveAll(dir)
	binary := filepath.Join(dir, "program")
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	for {
		// Before the snapshot, so files BeforeBuild writes do not count as a change.
		var prepareErr error
		if o.BeforeBuild != nil {
			prepareErr = o.BeforeBuild()
		}
		before, err := snapshot(o.Root)
		if err != nil {
			return err
		}
		var proc *process
		if err := errors.Join(prepareErr, buildIfPrepared(ctx, o, binary, prepareErr)); err != nil {
			if prepareErr != nil {
				fmt.Fprintf(o.Stderr, "gortisan run: %v\n", prepareErr)
			}
			if ctx.Err() != nil {
				return nil
			}
			if !o.Watch {
				return err
			}
			fmt.Fprintf(o.Stderr, "gortisan run: build failed; waiting for a change\n")
		} else {
			fmt.Fprintf(o.Stderr, "gortisan run: started %s\n", o.label())
			if proc, err = start(binary, o); err != nil {
				return err
			}
		}

		if !o.Watch {
			select {
			case <-ctx.Done():
				return proc.stop()
			case <-proc.done:
				return proc.err
			}
		}

		if err := waitForChange(ctx, o, before, proc); err != nil {
			return err
		}
		if err := proc.stop(); err != nil {
			fmt.Fprintf(o.Stderr, "gortisan run: %v\n", err)
		}
		if ctx.Err() != nil {
			return nil
		}
		fmt.Fprintf(o.Stderr, "gortisan run: change detected, rebuilding\n")
	}
}

// waitForChange returns when a watched file changes or ctx is done. It
// reports when the program exits on its own and keeps watching.
func waitForChange(ctx context.Context, o Options, before map[string]stamp, proc *process) error {
	ticker := time.NewTicker(o.Interval)
	defer ticker.Stop()
	var exited <-chan struct{}
	if proc != nil {
		exited = proc.done
	}
	for {
		select {
		case <-ctx.Done():
			return nil
		case <-exited:
			exited = nil
			if proc.err != nil {
				fmt.Fprintf(o.Stderr, "gortisan run: %s exited: %v; waiting for a change\n", o.label(), proc.err)
			} else {
				fmt.Fprintf(o.Stderr, "gortisan run: %s exited; waiting for a change\n", o.label())
			}
		case <-ticker.C:
			now, err := snapshot(o.Root)
			if err != nil {
				return err
			}
			if !sameFiles(before, now) {
				return nil
			}
		}
	}
}

func buildIfPrepared(ctx context.Context, o Options, binary string, prepareErr error) error {
	if prepareErr != nil {
		return nil
	}
	return build(ctx, o, binary)
}

func build(ctx context.Context, o Options, binary string) error {
	pkg := "./cmd/" + o.Target
	if o.Target == "." {
		pkg = "."
	}
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, pkg)
	cmd.Dir = o.Root
	cmd.Stdout = o.Stderr
	cmd.Stderr = o.Stderr
	return cmd.Run()
}

type process struct {
	cmd  *exec.Cmd
	done chan struct{}
	err  error
}

func start(binary string, o Options) (*process, error) {
	cmd := exec.Command(binary, o.Args...)
	cmd.Dir = o.Root // so the program finds .env
	cmd.Stdin = os.Stdin
	cmd.Stdout = o.Stdout
	cmd.Stderr = o.Stderr
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	p := &process{cmd: cmd, done: make(chan struct{})}
	go func() {
		p.err = cmd.Wait()
		close(p.done)
	}()
	return p, nil
}

// stop asks the program to stop (an interrupt, so it can shut down
// gracefully), and kills it after 10 seconds. Windows cannot deliver an
// interrupt to another process, so there it is killed right away.
func (p *process) stop() error {
	if p == nil {
		return nil
	}
	select {
	case <-p.done:
		return nil
	default:
	}
	if runtime.GOOS == "windows" || p.cmd.Process.Signal(os.Interrupt) != nil {
		_ = p.cmd.Process.Kill()
	}
	select {
	case <-p.done:
	case <-time.After(10 * time.Second):
		_ = p.cmd.Process.Kill()
		<-p.done
	}
	return nil
}

type stamp struct {
	mod  time.Time
	size int64
}

// skipDirs are never watched.
var skipDirs = map[string]bool{
	".git": true, ".idea": true, ".vscode": true, "node_modules": true,
	"vendor": true, "bin": true, "tmp": true, "migrations": true,
}

// watched reports whether a change to the file named name needs a restart.
func watched(name string) bool {
	return strings.HasSuffix(name, ".go") || name == "go.mod" || name == "go.sum" || name == ".env"
}

func snapshot(root string) (map[string]stamp, error) {
	files := map[string]stamp{}
	err := filepath.WalkDir(root, func(p string, d fs.DirEntry, err error) error {
		if err != nil {
			if errors.Is(err, fs.ErrNotExist) {
				return nil // removed while walking
			}
			return err
		}
		if d.IsDir() {
			if p != root && (skipDirs[d.Name()] || strings.HasPrefix(d.Name(), ".")) {
				return filepath.SkipDir
			}
			return nil
		}
		if !watched(d.Name()) {
			return nil
		}
		info, err := d.Info()
		if err != nil {
			return nil
		}
		files[p] = stamp{mod: info.ModTime(), size: info.Size()}
		return nil
	})
	return files, err
}

func sameFiles(a, b map[string]stamp) bool {
	if len(a) != len(b) {
		return false
	}
	for p, s := range a {
		if t, ok := b[p]; !ok || !t.mod.Equal(s.mod) || t.size != s.size {
			return false
		}
	}
	return true
}
