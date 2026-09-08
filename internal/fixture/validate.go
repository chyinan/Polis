// pattern: Functional Core
package fixture

import (
	"encoding/json"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"strconv"
	"strings"
)

// This fixture accepts pure formatting code, not arbitrary programs that can
// terminate or replace the trusted testing lifecycle.
func ValidateCandidate(source string) error {
	f, e := parser.ParseFile(token.NewFileSet(), "formatter.go", source, parser.ParseComments)
	if e != nil {
		return e
	}
	if f.Name.Name != "formatter" {
		return fmt.Errorf("package must be formatter")
	}
	for _, c := range f.Comments {
		for _, line := range c.List {
			if strings.HasPrefix(line.Text, "//go:") || strings.HasPrefix(line.Text, "//line ") {
				return fmt.Errorf("compiler directives are not allowed")
			}
		}
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.GenDecl:
			if d.Tok != token.IMPORT {
				return fmt.Errorf("only imports and pure formatting functions are permitted")
			}
			for _, s := range d.Specs {
				im := s.(*ast.ImportSpec)
				path, e := strconv.Unquote(im.Path.Value)
				if e != nil {
					return e
				}
				if im.Name != nil || (path != "math" && path != "strconv") {
					return fmt.Errorf("only unaliased math/strconv imports are permitted")
				}
			}
		case *ast.FuncDecl:
			if d.Name.Name == "init" || d.Name.Name == "TestMain" || d.Recv != nil || d.Body == nil {
				return fmt.Errorf("test-lifecycle hooks and methods are forbidden")
			}
		default:
			return fmt.Errorf("unsupported declaration")
		}
	}
	return nil
}
func TestsActuallyPassed(raw []byte, phase string) bool {
	required := []string{"TestFrozenCompatibility"}
	if phase == "full" {
		required = append(required, "TestFrozenNeighborChange")
	}
	run, passed := map[string]bool{}, map[string]bool{}
	for _, line := range strings.Split(string(raw), "\n") {
		var e struct{ Action, Test string }
		if json.Unmarshal([]byte(line), &e) != nil {
			continue
		}
		if e.Action == "fail" || e.Action == "skip" {
			return false
		}
		if e.Action == "run" {
			run[e.Test] = true
		}
		if e.Action == "pass" {
			passed[e.Test] = true
		}
	}
	for _, name := range required {
		if !run[name] || !passed[name] {
			return false
		}
	}
	return true
}
