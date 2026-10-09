// pattern: Functional Core
package gotestprofile

import (
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// The first profile is a pure formatter exercise, not an arbitrary Go project.
// Prevent candidate test entry points from replacing the trusted test runner.
func validateFixtureSourcePolicy(path string, content []byte) bool {
	if !strings.HasSuffix(path, ".go") {
		return true
	}
	f, err := parser.ParseFile(token.NewFileSet(), path, content, parser.ParseComments)
	if err != nil || f.Name.Name != "formatter" {
		return false
	}
	for _, im := range f.Imports {
		p, e := strconv.Unquote(im.Path.Value)
		if e != nil || im.Name != nil {
			return false
		}
		switch p {
		case "math", "strconv", "strings":
		case "testing":
			if !strings.HasSuffix(path, "_test.go") {
				return false
			}
		default:
			return false
		}
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			if d.Name.Name == "init" || d.Name.Name == "TestMain" {
				return false
			}
		case *ast.GenDecl:
			if d.Tok == token.VAR {
				return false
			}
		}
	}
	valid := true
	ast.Inspect(f, func(n ast.Node) bool {
		switch v := n.(type) {
		case *ast.GoStmt:
			valid = false
		case *ast.CallExpr:
			if id, ok := v.Fun.(*ast.Ident); ok && (id.Name == "print" || id.Name == "println") {
				valid = false
			}
		case *ast.SelectorExpr:
			if id, ok := v.X.(*ast.Ident); ok && id.Name == "testing" && v.Sel.Name != "T" && v.Sel.Name != "Short" {
				valid = false
			}
		case *ast.Comment:
			if strings.HasPrefix(strings.TrimSpace(strings.TrimPrefix(v.Text, "//")), "go:") {
				valid = false
			}
		}
		return valid
	})
	return valid
}
