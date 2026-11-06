package editor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestKindNamesOnlyInDescriptors keeps what a kind is in kinds.go: no other
// file of the editor compares a node's kind with a kind's name or switches
// on it, so a new kind is described in one place.
func TestKindNamesOnlyInDescriptors(t *testing.T) {
	names := make(map[string]bool, len(kindTable))
	set := token.NewFileSet()
	parsed, err := parser.ParseFile(set, "kinds.go", nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range parsed.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.CONST {
			for _, spec := range gen.Specs {
				for _, name := range spec.(*ast.ValueSpec).Names {
					names[name.Name] = true
				}
			}
		}
	}
	if len(names) != len(kindTable) {
		t.Fatalf("kinds.go names %d kinds, describes %d", len(names), len(kindTable))
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		file := entry.Name()
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") || file == "kinds.go" {
			continue
		}
		parsed, err := parser.ParseFile(set, file, nil, 0)
		if err != nil {
			t.Fatal(err)
		}
		named := func(expr ast.Expr) bool {
			ident, ok := expr.(*ast.Ident)
			return ok && names[ident.Name]
		}
		ast.Inspect(parsed, func(node ast.Node) bool {
			switch node := node.(type) {
			case *ast.BinaryExpr:
				if (node.Op == token.EQL || node.Op == token.NEQ) && (named(node.X) || named(node.Y)) {
					t.Errorf("%s: kind compared by name; ask its descriptor", set.Position(node.Pos()))
				}
			case *ast.CaseClause:
				for _, expr := range node.List {
					if named(expr) {
						t.Errorf("%s: kind switched on by name; ask its descriptor", set.Position(expr.Pos()))
					}
				}
			}
			return true
		})
	}
}

func TestEveryKindIsDescribedOnce(t *testing.T) {
	seen := make(map[string]bool)
	for _, desc := range kindTable {
		if seen[string(desc.Kind)] {
			t.Fatalf("%s described twice", desc.Kind)
		}
		seen[string(desc.Kind)] = true
		if desc.Literal && !desc.KeepsLines {
			t.Fatalf("%s is literal but breaks at lines", desc.Kind)
		}
		if (desc.Group != "") != desc.Prefix && desc.Kind != HeadingNode {
			t.Fatalf("%s: grouped blocks are prefixed", desc.Kind)
		}
	}
}
