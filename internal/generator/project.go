package generator

import (
	"bufio"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"
)

// SettingsFile is written into every project by create. It records how the
// project was laid out, so a later gortisan generates code that fits it.
const SettingsFile = ".gortisan.json"

// Layout is the project layout this gortisan writes. It changes only with a
// new major version of gortisan, and gortisan keeps generating code for
// every layout of its major version (see COMPATIBILITY.md).
const Layout = 1

// Settings is the content of .gortisan.json.
type Settings struct {
	// Layout is the project layout, see Layout.
	Layout int `json:"layout"`
	// Version is the gortisan version the ./gortisan scripts run.
	Version string `json:"version"`
	// With are the components of the project: postgres, redis, migrations,
	// swagger.
	With []string `json:"with"`
	// BasePath is the default BASE_PATH every feature is mounted under.
	BasePath string `json:"basePath"`
}

// Project is the Go module `make` writes into.
type Project struct {
	Root   string
	Module string
	// Settings is nil when the project has no .gortisan.json.
	Settings *Settings
}

// FindProject walks up from dir to the nearest go.mod and reads the
// project's .gortisan.json, when there is one.
func FindProject(dir string) (*Project, error) {
	dir, err := filepath.Abs(dir)
	if err != nil {
		return nil, err
	}
	for {
		gomod := filepath.Join(dir, "go.mod")
		if _, err := os.Stat(gomod); err == nil {
			module, err := readModule(gomod)
			if err != nil {
				return nil, err
			}
			p := &Project{Root: dir, Module: module}
			if p.Settings, err = readSettings(dir); err != nil {
				return nil, err
			}
			return p, nil
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			return nil, errors.New("no go.mod in this directory or any parent; run gortisan inside a project")
		}
		dir = parent
	}
}

func readSettings(root string) (*Settings, error) {
	data, err := os.ReadFile(filepath.Join(root, SettingsFile))
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var s Settings
	if err := json.Unmarshal(data, &s); err != nil {
		return nil, fmt.Errorf("%s: %w", SettingsFile, err)
	}
	return &s, nil
}

func writeSettings(root string, s *Settings) error {
	data, err := json.MarshalIndent(s, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(filepath.Join(root, SettingsFile), append(data, '\n'), 0o644)
}

// Check fails when gortisan cannot generate code for the project: it has no
// .gortisan.json (created before v0.1.0, or not by gortisan), or a layout
// newer than this gortisan knows.
func (p *Project) Check() error {
	switch {
	case p.Settings == nil:
		return fmt.Errorf("%s has no %s: it was not created by gortisan v0.1.0 or newer, so gortisan cannot tell how it is laid out", p.Root, SettingsFile)
	case p.Settings.Layout > Layout:
		return fmt.Errorf("this project uses layout %d, newer than this gortisan (layout %d); run ./gortisan, which runs the version the project pins, or update gortisan", p.Settings.Layout, Layout)
	case p.Settings.Layout < 1:
		return fmt.Errorf("%s: layout %d is not valid", SettingsFile, p.Settings.Layout)
	}
	return nil
}

// Has reports whether the project has component (postgres, redis,
// migrations, swagger).
func (p *Project) Has(component string) bool {
	return p.Settings != nil && slices.Contains(p.Settings.With, component)
}

// need fails with a hint when the project lacks component.
func (p *Project) need(component, why string) error {
	if p.Has(component) {
		return nil
	}
	return fmt.Errorf("%s needs %s; add it with `./gortisan add %s`", why, component, component)
}

func (p *Project) basePath() string {
	if p.Settings == nil || p.Settings.BasePath == "" {
		return DefaultBasePath
	}
	return p.Settings.BasePath
}

func readModule(gomod string) (string, error) {
	file, err := os.Open(gomod)
	if err != nil {
		return "", err
	}
	defer file.Close()
	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := strings.TrimSpace(scanner.Text())
		if rest, ok := strings.CutPrefix(line, "module "); ok {
			return strings.Trim(strings.TrimSpace(rest), `"`), nil
		}
	}
	if err := scanner.Err(); err != nil {
		return "", err
	}
	return "", fmt.Errorf("%s has no module line", gomod)
}

func (p *Project) exists(rel string) bool {
	_, err := os.Stat(filepath.Join(p.Root, filepath.FromSlash(rel)))
	return err == nil
}

func (p *Project) require(path, why string) error {
	if !p.exists(path) {
		return fmt.Errorf("%s is missing; %s", path, why)
	}
	return nil
}
