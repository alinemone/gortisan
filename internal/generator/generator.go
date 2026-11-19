// Package generator writes the files behind `gortisan create` and
// `gortisan make`.
//
// Everything it writes belongs to the generated project: no file imports
// gortisan, and gortisan never appears in the project's go.mod.
package generator

import (
	"bytes"
	"embed"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"strings"
	"text/template"
)

//go:embed templates
var templateFS embed.FS

// Report collects what a command did.
type Report struct {
	Created []string
	Skipped []string
	Next    []string
}

func (r *Report) next(format string, args ...any) {
	r.Next = append(r.Next, fmt.Sprintf(format, args...))
}

// output is one file to write: a path relative to the project root and the
// template that renders it.
type output struct {
	path     string
	template string
}

// render executes templates/<name>. Go sources are gofmt'ed, so a broken
// template fails here instead of in the user's build.
func render(name string, data any) ([]byte, error) {
	text, err := templateFS.ReadFile("templates/" + name)
	if err != nil {
		return nil, err
	}
	tmpl, err := template.New(name).Option("missingkey=error").Parse(string(text))
	if err != nil {
		return nil, fmt.Errorf("parse template %s: %w", name, err)
	}
	var buf bytes.Buffer
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("render template %s: %w", name, err)
	}
	if !strings.HasSuffix(name, ".go.tmpl") {
		return buf.Bytes(), nil
	}
	formatted, err := format.Source(buf.Bytes())
	if err != nil {
		return nil, fmt.Errorf("gofmt %s: %w\n%s", name, err, buf.String())
	}
	return formatted, nil
}

// write writes content to root/rel. An existing file is kept (and reported
// as skipped) unless force is set.
func write(root, rel string, content []byte, force bool, report *Report) error {
	target := filepath.Join(root, filepath.FromSlash(rel))
	if _, err := os.Stat(target); err == nil && !force {
		report.Skipped = append(report.Skipped, rel)
		return nil
	}
	if err := os.MkdirAll(filepath.Dir(target), 0o755); err != nil {
		return err
	}
	if err := os.WriteFile(target, content, 0o644); err != nil {
		return err
	}
	report.Created = append(report.Created, rel)
	return nil
}

// renderAll renders every output with data and writes it under root.
func renderAll(root string, outputs []output, data any, force bool, report *Report) error {
	for _, out := range outputs {
		content, err := render(out.template, data)
		if err != nil {
			return err
		}
		if err := write(root, out.path, content, force, report); err != nil {
			return err
		}
	}
	return nil
}
