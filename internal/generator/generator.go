// Package generator renders CRUD repository code from parsed model metadata.
package generator

import (
	"bytes"
	"fmt"
	"go/format"
	"os"
	"path/filepath"
	"text/template"

	"crud-gen/internal/templates"
)

// Generate renders repository (and optionally test) files from pre-built TemplateData.
func Generate(data *TemplateData) error {
	repoTmpl, err := loadTemplate(data.RepoTemplatePath, templates.Repository)
	if err != nil {
		return fmt.Errorf("load repository template: %w", err)
	}
	repoSrc, err := render(repoTmpl, data)
	if err != nil {
		return fmt.Errorf("render repository: %w", err)
	}
	formatted, err := format.Source(repoSrc)
	if err != nil {
		// Include raw output so the user can debug template issues
		return fmt.Errorf("gofmt repository: %w\n\n--- raw output ---\n%s", err, repoSrc)
	}
	if err := writeOrPrint(data.OutputPath, formatted); err != nil {
		return err
	}

	if data.WithTests {
		testTmpl, err := loadTemplate(data.TestTemplatePath, templates.Test)
		if err != nil {
			return fmt.Errorf("load test template: %w", err)
		}
		testSrc, err := render(testTmpl, data)
		if err != nil {
			return fmt.Errorf("render tests: %w", err)
		}
		fmtTest, err := format.Source(testSrc)
		if err != nil {
			return fmt.Errorf("gofmt tests: %w\n\n--- raw ---\n%s", err, testSrc)
		}
		if err := writeOrPrint(testPath(data.OutputPath), fmtTest); err != nil {
			return err
		}
	}
	return nil
}

// loadTemplate returns the built-in template source when path is empty,
// otherwise reads the file at path.
func loadTemplate(path, builtin string) (string, error) {
	if path == "" {
		return builtin, nil
	}
	b, err := os.ReadFile(path)
	if err != nil {
		return "", fmt.Errorf("read template %s: %w", path, err)
	}
	return string(b), nil
}

func render(src string, data *TemplateData) ([]byte, error) {
	t, err := template.New("crud").Parse(src)
	if err != nil {
		return nil, fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	if err := t.Execute(&buf, data); err != nil {
		return nil, fmt.Errorf("execute template: %w", err)
	}
	return buf.Bytes(), nil
}

func writeOrPrint(path string, src []byte) error {
	if path == "" {
		_, err := os.Stdout.Write(src)
		return err
	}
	if err := os.MkdirAll(filepath.Dir(path), 0755); err != nil {
		return fmt.Errorf("mkdir %s: %w", filepath.Dir(path), err)
	}
	return os.WriteFile(path, src, 0644)
}

func testPath(repoPath string) string {
	if repoPath == "" {
		return ""
	}
	ext := filepath.Ext(repoPath)
	return repoPath[:len(repoPath)-len(ext)] + "_test" + ext
}
