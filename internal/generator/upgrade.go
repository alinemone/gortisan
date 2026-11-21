package generator

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"
)

// toolModule is the module ToolPackage belongs to.
var toolModule = strings.TrimSuffix(ToolPackage, "/cmd/gortisan")

// scripts are the files that run the pinned gortisan.
var scripts = []string{"gortisan", "gortisan.bat", "gortisan.ps1"}

// Upgrade pins another gortisan version in the project's ./gortisan scripts
// and .gortisan.json. version is a tag (v1.2.0) or "latest", which is
// resolved to the newest tag so the pin stays exact. The project's code is
// not touched: gortisan keeps generating code that fits the project's
// layout (see COMPATIBILITY.md).
func Upgrade(p *Project, version string) (*Report, error) {
	report := &Report{}
	if err := p.Check(); err != nil {
		return report, err
	}
	if version == "" || version == "latest" {
		resolved, err := latestVersion(p.Root)
		if err != nil {
			return report, err
		}
		version = resolved
	}
	if !strings.HasPrefix(version, "v") {
		return report, fmt.Errorf("version %q must look like v1.2.3", version)
	}

	pinned := regexp.MustCompile(regexp.QuoteMeta(ToolPackage) + `@\S+`)
	for _, rel := range scripts {
		path := filepath.Join(p.Root, rel)
		src, err := os.ReadFile(path)
		if err != nil {
			report.next("%s is missing; create it like the other scripts, running go run %s@%s", rel, ToolPackage, version)
			continue
		}
		out := pinned.ReplaceAll(src, []byte(ToolPackage+"@"+version))
		if string(out) == string(src) {
			continue
		}
		info, _ := os.Stat(path)
		if err := os.WriteFile(path, out, info.Mode().Perm()); err != nil {
			return report, err
		}
		report.Updated = append(report.Updated, rel)
	}
	if p.Settings.Version != version {
		p.Settings.Version = version
		if err := writeSettings(p.Root, p.Settings); err != nil {
			return report, err
		}
		report.Updated = append(report.Updated, SettingsFile)
	}
	if len(report.Updated) == 0 {
		report.next("already on %s", version)
	}
	return report, nil
}

// latestVersion asks the Go module proxy for the newest gortisan release.
func latestVersion(dir string) (string, error) {
	cmd := exec.Command("go", "list", "-m", "-json", toolModule+"@latest")
	cmd.Dir = dir
	out, err := cmd.Output()
	if err != nil {
		return "", fmt.Errorf("find the latest gortisan: %w", err)
	}
	var info struct{ Version string }
	if err := json.Unmarshal(out, &info); err != nil {
		return "", err
	}
	return info.Version, nil
}
