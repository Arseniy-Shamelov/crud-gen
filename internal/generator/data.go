package generator

import (
	"fmt"
	"strings"
	"unicode"

	"crud-gen/internal/parser"
)

// Dialect identifies the target SQL database.
type Dialect string

const (
	DialectPostgres Dialect = "postgres"
)

// Config holds code-generation parameters supplied by the caller (CLI or HTTP).
type Config struct {
	Model            *parser.Model
	Package          string // target package name (e.g. "repo")
	OutputPath       string // file to write; "" → stdout
	WithTests        bool
	ModelRef         string // "User" (same pkg) or "models.User" (cross-pkg)
	ModelImport      string // "" or full import path e.g. "mymod/models"
	Dialect          string // "postgres" (default)
	RepoTemplatePath string // "" → use built-in
	TestTemplatePath string // "" → use built-in
}

// TemplateData holds pre-computed strings passed to text/template.
// All SQL-building logic lives here in Go — not inside the template.
type TemplateData struct {
	Package     string
	ModelName   string
	ModelRef    string // "User" or "models.User"
	ModelImport string // "" or "mymod/models"
	WithTests   bool
	OutputPath  string

	IDField      string // Go field name, e.g. "ID"
	IDColumn     string // SQL column name, e.g. "id"
	TableName    string // snake_case plural, e.g. "user_profiles"
	ReceiverName string // lowercase first letter, e.g. "u" for User

	// SELECT — all non-excluded fields including ID
	SelectColumns string // "id, name, email"
	ScanArgs      string // "&m.ID, &m.Name, &m.Email"

	// INSERT — non-ID, non-excluded fields
	InsertColumns      string // "name, email"
	InsertPlaceholders string // "$1, $2" or "?, ?"
	InsertArgs         string // "m.Name, m.Email"

	// UPDATE SET — same field set as INSERT
	SetClause  string // "name = $1, email = $2" or "name = ?, email = ?"
	UpdateArgs string // "m.Name, m.Email, id"
	WhereParam int    // param index for WHERE id = $N (postgres); kept for custom template compat

	// Dialect-specific
	UseReturning bool   // true for postgres (RETURNING id vs LastInsertId)
	SingleParam  string // "$1" or "?" — single-arg WHERE clause placeholder
	UpdateWhere  string // "$N" or "?" — UPDATE WHERE id placeholder
	DriverImport string // driver import path for generated test file

	// Custom template paths
	RepoTemplatePath string
	TestTemplatePath string
}

// BuildTemplateData computes all SQL fragments from parsed model metadata.
func BuildTemplateData(cfg *Config) *TemplateData {
	allActive := nonExcluded(cfg.Model.Fields)
	insertFields := nonIDFields(allActive, cfg.Model.IDField)

	useReturning := cfg.Dialect == "" || cfg.Dialect == string(DialectPostgres)

	// SELECT: all active fields
	selCols := make([]string, len(allActive))
	scanArgs := make([]string, len(allActive))
	for i, f := range allActive {
		selCols[i] = toSnake(f.Name)
		scanArgs[i] = "&m." + f.Name
	}

	// INSERT / UPDATE: non-ID fields
	insCols := make([]string, len(insertFields))
	insPlaceholders := make([]string, len(insertFields))
	insArgs := make([]string, len(insertFields))
	setClauseParts := make([]string, len(insertFields))
	updArgs := make([]string, len(insertFields)+1)

	for i, f := range insertFields {
		insCols[i] = toSnake(f.Name)
		if useReturning {
			insPlaceholders[i] = fmt.Sprintf("$%d", i+1)
			setClauseParts[i] = fmt.Sprintf("%s = $%d", toSnake(f.Name), i+1)
		} else {
			insPlaceholders[i] = "?"
			setClauseParts[i] = fmt.Sprintf("%s = ?", toSnake(f.Name))
		}
		insArgs[i] = "m." + f.Name
		updArgs[i] = "m." + f.Name
	}
	// Last update arg is the WHERE id value
	updArgs[len(insertFields)] = "id"

	whereParam := len(insertFields) + 1
	singleParam := "$1"
	updateWhere := fmt.Sprintf("$%d", whereParam)
	driverImport := "github.com/lib/pq"

	return &TemplateData{
		Package:            cfg.Package,
		ModelName:          cfg.Model.Name,
		ModelRef:           cfg.ModelRef,
		ModelImport:        cfg.ModelImport,
		WithTests:          cfg.WithTests,
		OutputPath:         cfg.OutputPath,
		IDField:            cfg.Model.IDField,
		IDColumn:           toSnake(cfg.Model.IDField),
		TableName:          toSnake(cfg.Model.Name) + "s",
		ReceiverName:       strings.ToLower(cfg.Model.Name[:1]),
		SelectColumns:      strings.Join(selCols, ", "),
		ScanArgs:           strings.Join(scanArgs, ", "),
		InsertColumns:      strings.Join(insCols, ", "),
		InsertPlaceholders: strings.Join(insPlaceholders, ", "),
		InsertArgs:         strings.Join(insArgs, ", "),
		SetClause:          strings.Join(setClauseParts, ", "),
		UpdateArgs:         strings.Join(updArgs, ", "),
		WhereParam:         whereParam,
		UseReturning:       useReturning,
		SingleParam:        singleParam,
		UpdateWhere:        updateWhere,
		DriverImport:       driverImport,
		RepoTemplatePath:   cfg.RepoTemplatePath,
		TestTemplatePath:   cfg.TestTemplatePath,
	}
}

func nonExcluded(fields []*parser.Field) []*parser.Field {
	var out []*parser.Field
	for _, f := range fields {
		if !f.Excluded {
			out = append(out, f)
		}
	}
	return out
}

func nonIDFields(fields []*parser.Field, idField string) []*parser.Field {
	var out []*parser.Field
	for _, f := range fields {
		if f.Name != idField {
			out = append(out, f)
		}
	}
	return out
}

// toSnake converts CamelCase → snake_case.
//
// Rules:
//   - Insert '_' before an uppercase letter when the previous char is lowercase ("camelCase" boundary).
//   - Insert '_' before an uppercase letter when it starts a new word after an acronym
//     (e.g. the 'B' in "URLBase": prev='L' upper, next='a' lower → "url_base").
//   - Consecutive uppercase letters without a trailing lowercase are kept together ("ID" → "id").
func toSnake(s string) string {
	runes := []rune(s)
	var b strings.Builder
	for i, r := range runes {
		if i > 0 && unicode.IsUpper(r) {
			prev := runes[i-1]
			prevIsLower := unicode.IsLower(prev)
			nextIsLower := i+1 < len(runes) && unicode.IsLower(runes[i+1])
			if prevIsLower || (unicode.IsUpper(prev) && nextIsLower) {
				b.WriteByte('_')
			}
		}
		b.WriteRune(unicode.ToLower(r))
	}
	return b.String()
}
