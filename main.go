// crud-gen generates CRUD repository code from a Go struct definition.
//
// CLI usage:
//
//	crud-gen --model=User --input=./models/user.go --output=./repositories/user_repo.go --package=repo
//	crud-gen --model=User --input=./models/user.go --package=repo --with-tests
//
// HTTP server mode:
//
//	crud-gen --serve --port=8080
//	curl -s -X POST http://localhost:8080/generate \
//	  -H 'Content-Type: application/json' \
//	  -d '{"model":"User","input":"./models/user.go","output":"./repo/user.go","package":"repo"}'
package main

import (
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	"crud-gen/internal/generator"
	"crud-gen/internal/parser"
)

func main() {
	// ── Flags ─────────────────────────────────────────────────────────────────
	fModel := flag.String("model", "", "struct name to generate CRUD for (e.g. User)")
	fInput := flag.String("input", "", "path to Go file containing the struct")
	fOutput := flag.String("output", "", "output file path (default: stdout)")
	fPkg := flag.String("package", "", "package name for generated code")
	fIDField := flag.String("id-field", "ID", "name of the primary-key field")
	fWithTests  := flag.Bool("with-tests", false, "also generate _test.go")
	fDialect    := flag.String("dialect", "postgres", "SQL dialect: postgres")
	fRepoTmpl   := flag.String("repo-template", "", "path to custom repository template file")
	fTestTmpl   := flag.String("test-template", "", "path to custom test template file")
	fServe      := flag.Bool("serve", false, "start HTTP server instead of CLI generation")
	fPort       := flag.Int("port", 8080, "HTTP server port (used with --serve)")
	flag.Parse()

	if *fServe {
		runServer(*fPort)
		return
	}

	if err := validateDialect(*fDialect); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		flag.Usage()
		os.Exit(1)
	}

	if err := validateCLIFlags(*fModel, *fInput, *fPkg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		flag.Usage()
		os.Exit(1)
	}

	cfg := &runConfig{
		ModelName:    *fModel,
		InputPath:    *fInput,
		OutputPath:   *fOutput,
		Package:      *fPkg,
		IDField:      *fIDField,
		WithTests:    *fWithTests,
		Dialect:      *fDialect,
		RepoTemplate: *fRepoTmpl,
		TestTemplate: *fTestTmpl,
	}
	if err := run(cfg); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

// ── Core generation logic ─────────────────────────────────────────────────────

type runConfig struct {
	ModelName    string `json:"model"`
	InputPath    string `json:"input"`
	OutputPath   string `json:"output"`
	Package      string `json:"package"`
	IDField      string `json:"id_field"`
	WithTests    bool   `json:"with_tests"`
	Dialect      string `json:"dialect"`
	RepoTemplate string `json:"repo_template"`
	TestTemplate string `json:"test_template"`
}

// run is the single entry point shared by CLI and HTTP handler.
func run(cfg *runConfig) error {
	if cfg.IDField == "" {
		cfg.IDField = "ID"
	}

	// ── 1. Parse AST ──────────────────────────────────────────────────────────
	// parser.ParseFile walks *ast.StructType nodes, extracts field names/types.
	// See internal/parser/parser.go for the AST traversal details.
	model, err := parser.ParseFile(cfg.InputPath, cfg.ModelName, cfg.IDField)
	if err != nil {
		return fmt.Errorf("parse: %w", err)
	}

	// ── 2. Resolve import path for cross-package generation ───────────────────
	modelRef, modelImport, err := resolveModelImport(cfg.InputPath, cfg.Package, model.Package, model.Name)
	if err != nil {
		// Non-fatal: fall back to unqualified name (same-package assumption)
		log.Printf("warn: cannot resolve module import: %v — assuming same package", err)
		modelRef = model.Name
		modelImport = ""
	}

	// ── 3. Pre-compute SQL fragments, build template data ────────────────────
	genCfg := &generator.Config{
		Model:            model,
		Package:          cfg.Package,
		OutputPath:       cfg.OutputPath,
		WithTests:        cfg.WithTests,
		ModelRef:         modelRef,
		ModelImport:      modelImport,
		Dialect:          cfg.Dialect,
		RepoTemplatePath: cfg.RepoTemplate,
		TestTemplatePath: cfg.TestTemplate,
	}
	data := generator.BuildTemplateData(genCfg)

	// ── 4. Render + gofmt + write ─────────────────────────────────────────────
	return generator.Generate(data)
}

// resolveModelImport figures out ModelRef and ModelImport.
//
// If the target package equals the model's own package → no import needed.
// Otherwise → auto-detect module from go.mod and build the import path.
func resolveModelImport(inputFile, targetPkg, modelPkg, modelName string) (ref, imp string, err error) {
	if targetPkg == modelPkg {
		// Same package — no import, bare type name
		return modelName, "", nil
	}

	// Walk up from input file's directory to find go.mod
	absInput, err := filepath.Abs(inputFile)
	if err != nil {
		return "", "", err
	}
	moduleName, moduleDir, err := detectModule(filepath.Dir(absInput))
	if err != nil {
		return "", "", err
	}

	// Compute relative path from module root to model directory
	rel, err := filepath.Rel(moduleDir, filepath.Dir(absInput))
	if err != nil {
		return "", "", err
	}
	importPath := moduleName + "/" + filepath.ToSlash(rel)

	// Qualifier is the package name; ref is "pkg.TypeName"
	return modelPkg + "." + modelName, importPath, nil
}

// detectModule walks directory ancestors looking for go.mod.
// Returns (moduleName, moduleDir, error).
func detectModule(startDir string) (string, string, error) {
	dir := filepath.Clean(startDir)
	for {
		gomodPath := filepath.Join(dir, "go.mod")
		data, err := os.ReadFile(gomodPath)
		if err == nil {
			for _, line := range strings.Split(string(data), "\n") {
				line = strings.TrimSpace(line)
				if strings.HasPrefix(line, "module ") {
					mod := strings.TrimSpace(strings.TrimPrefix(line, "module"))
					return mod, dir, nil
				}
			}
			return "", "", errors.New("go.mod found but no module directive")
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break // reached filesystem root
		}
		dir = parent
	}
	return "", "", errors.New("go.mod not found")
}

// ── HTTP server ───────────────────────────────────────────────────────────────

func runServer(port int) {
	mux := http.NewServeMux()
	mux.HandleFunc("/generate", generateHandler)
	mux.HandleFunc("/health", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write([]byte("ok"))
	})

	addr := fmt.Sprintf(":%d", port)
	log.Printf("crud-gen HTTP server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server: %v", err)
	}
}

func generateHandler(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}

	var cfg runConfig
	dec := json.NewDecoder(r.Body)
	dec.DisallowUnknownFields()
	if err := dec.Decode(&cfg); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateCLIFlags(cfg.ModelName, cfg.InputPath, cfg.Package); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := validateDialect(cfg.Dialect); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if cfg.OutputPath == "" {
		http.Error(w, "output path required for HTTP mode", http.StatusBadRequest)
		return
	}

	if err := validateHTTPPaths(cfg.InputPath, cfg.OutputPath, cfg.RepoTemplate, cfg.TestTemplate); err != nil {
		http.Error(w, "bad request: "+err.Error(), http.StatusBadRequest)
		return
	}

	if err := run(&cfg); err != nil {
		http.Error(w, "generation failed: "+err.Error(), http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "application/json")
	_ = json.NewEncoder(w).Encode(map[string]string{
		"status": "ok",
		"output": cfg.OutputPath,
	})
}

func validateDialect(d string) error {
	switch d {
	case "", "postgres":
		return nil
	}
	return fmt.Errorf("unknown dialect %q: must be postgres", d)
}

// validateHTTPPaths rejects absolute paths and path traversal attempts.
// In HTTP mode the server must not read/write arbitrary filesystem locations.
func validateHTTPPaths(paths ...string) error {
	for _, p := range paths {
		if p == "" {
			continue
		}
		cleaned := filepath.Clean(p)
		if filepath.IsAbs(cleaned) || strings.HasPrefix(cleaned, "..") {
			return fmt.Errorf("path not allowed: %s", p)
		}
	}
	return nil
}

func validateCLIFlags(model, input, pkg string) error {
	var missing []string
	if model == "" {
		missing = append(missing, "--model")
	}
	if input == "" {
		missing = append(missing, "--input")
	}
	if pkg == "" {
		missing = append(missing, "--package")
	}
	if len(missing) > 0 {
		return fmt.Errorf("required flags missing: %s", strings.Join(missing, ", "))
	}
	return nil
}
