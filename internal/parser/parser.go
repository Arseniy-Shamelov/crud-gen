// Package parser extracts struct metadata from Go source files via AST.
package parser

import (
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"reflect"
	"strings"
)

// Field represents a single struct field.
type Field struct {
	Name     string
	Type     string // canonical Go type string
	IsPtr    bool   // true if type is a pointer (e.g. *string)
	IsTime   bool   // true if type is time.Time or *time.Time
	Optional bool   // true if pointer field
	Tag      string // raw struct tag value
	Excluded bool   // true when json:"-" tag present
}

// Model holds parsed struct metadata.
type Model struct {
	Name    string
	Package string
	Fields  []*Field
	IDField string // name of the primary key field
}

// ParseFile opens a Go source file and extracts the named struct.
//
// AST walk strategy:
//  1. parser.ParseFile → *ast.File (full syntax tree)
//  2. Walk ast.Decls looking for *ast.GenDecl with tok == TYPE
//  3. Inside each GenDecl find *ast.TypeSpec whose Name matches structName
//  4. The TypeSpec.Type must be *ast.StructType — iterate its Fields.List
//  5. Each ast.Field may declare multiple names; handle both named and embedded
func ParseFile(path, structName, idField string) (*Model, error) {
	fset := token.NewFileSet()
	// parser.ParseComments keeps doc comments; not required but harmless
	f, err := parser.ParseFile(fset, path, nil, parser.ParseComments)
	if err != nil {
		return nil, fmt.Errorf("parse %s: %w", path, err)
	}

	model := &Model{
		Name:    structName,
		Package: f.Name.Name,
		IDField: idField,
	}

	found := false
	// ast.Inspect does a depth-first walk; we stop descending once we find the struct
	ast.Inspect(f, func(n ast.Node) bool {
		if found {
			return false
		}
		ts, ok := n.(*ast.TypeSpec)
		if !ok || ts.Name.Name != structName {
			return true
		}
		st, ok := ts.Type.(*ast.StructType)
		if !ok {
			return true
		}
		found = true
		model.Fields = extractFields(st)
		return false
	})

	if !found {
		return nil, fmt.Errorf("struct %q not found in %s", structName, path)
	}
	return model, nil
}

// extractFields converts ast.FieldList → []*Field.
func extractFields(st *ast.StructType) []*Field {
	var fields []*Field
	for _, f := range st.Fields.List {
		// Embedded fields have no Names — skip them for CRUD generation
		if len(f.Names) == 0 {
			continue
		}

		rawTag := ""
		if f.Tag != nil {
			// f.Tag.Value includes surrounding backticks; strip them
			rawTag = strings.Trim(f.Tag.Value, "`")
		}

		// Honour json:"-" — field must not appear in generated code
		excluded := false
		if rawTag != "" {
			tag := reflect.StructTag(rawTag)
			if jv := tag.Get("json"); jv == "-" {
				excluded = true
			}
		}

		typStr, isPtr, isTime := resolveType(f.Type)

		for _, ident := range f.Names {
			fields = append(fields, &Field{
				Name:     ident.Name,
				Type:     typStr,
				IsPtr:    isPtr,
				IsTime:   isTime,
				Optional: isPtr,
				Tag:      rawTag,
				Excluded: excluded,
			})
		}
	}
	return fields
}

// resolveType converts an ast.Expr to a canonical type string.
//
// Handles:
//   - *ast.Ident          → simple type (string, int, bool …)
//   - *ast.StarExpr       → pointer (*string, *int …)
//   - *ast.SelectorExpr   → qualified type (time.Time, sql.NullString …)
//   - *ast.ArrayType      → slice ([]byte …)
//   - *ast.MapType        → map[K]V
func resolveType(expr ast.Expr) (typStr string, isPtr bool, isTime bool) {
	switch t := expr.(type) {

	case *ast.Ident:
		return t.Name, false, false

	case *ast.StarExpr:
		inner, _, innerIsTime := resolveType(t.X)
		return "*" + inner, true, innerIsTime

	case *ast.SelectorExpr:
		// X.Sel — e.g. time.Time → pkg="time", sel="Time"
		pkg, ok := t.X.(*ast.Ident)
		if !ok {
			return "interface{}", false, false
		}
		full := pkg.Name + "." + t.Sel.Name
		isT := pkg.Name == "time" && t.Sel.Name == "Time"
		return full, false, isT

	case *ast.ArrayType:
		inner, _, _ := resolveType(t.Elt)
		return "[]" + inner, false, false

	case *ast.MapType:
		k, _, _ := resolveType(t.Key)
		v, _, _ := resolveType(t.Value)
		return "map[" + k + "]" + v, false, false

	default:
		return "interface{}", false, false
	}
}
