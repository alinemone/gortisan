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
	// Target is the directory under cmd/ to run, e.g. "shop" or "worker".
	Target string
	// Args are passed to the program.
	Args []string
	// Watch rebuilds and restarts on changes; without it Run runs once.
	Watch bool
	// Interval is how often files are checked; 500ms when zero.
	Interval time.Duration
	// Stdout and Stderr receive the program's output and the runner's messages.
	Stdout, Stderr io.Writer
}

// ResolveTarget picks the command to run: name when given, otherwise the
// only directory under cmd/ (ignoring worker), or the one named after the
// module.
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
		if e.IsDir() && e.Name() != "worker" {
			candidates = append(candidates, e.Name())
		}
	}
	switch {
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
	binary := filepath.Join(dir, o.Target)
	if runtime.GOOS == "windows" {
		binary += ".exe"
	}

	for {
		before, err := snapshot(o.Root)
		if err != nil {
			return err
		}
		var proc *process
		if err := build(ctx, o, binary); err != nil {
			if ctx.Err() != nil {
				return nil
			}
			if !o.Watch {
				return err
			}
			fmt.Fprintf(o.Stderr, "gortisan run: build failed; waiting for a change\n")
		} else {
			fmt.Fprintf(o.Stderr, "gortisan run: started cmd/%s\n", o.Target)
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
				fmt.Fprintf(o.Stderr, "gortisan run: cmd/%s exited: %v; waiting for a change\n", o.Target, proc.err)
			} else {
				fmt.Fprintf(o.Stderr, "gortisan run: cmd/%s exited; waiting for a change\n", o.Target)
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

func build(ctx context.Context, o Options, binary string) error {
	cmd := exec.CommandContext(ctx, "go", "build", "-o", binary, "./cmd/"+o.Target)
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
