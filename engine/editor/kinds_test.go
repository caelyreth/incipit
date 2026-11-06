package editor

import (
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

// TestKindNamesOnlyInDescriptors keeps what a kind is in its descriptor
// file: no other file of the editor compares a node's or a mark's kind with
// a kind's name or switches on it, so a new kind is described in one place.
func TestKindNamesOnlyInDescriptors(t *testing.T) {
	descriptorNames(t, "kinds.go", len(kindTable))
	descriptorNames(t, "marks.go", len(markTable))
}

// descriptorNames checks that the kinds file names, as typed constants, are
// all described and compared nowhere else.
func descriptorNames(t *testing.T, descriptors string, described int) {
	t.Helper()
	names := make(map[string]bool)
	set := token.NewFileSet()
	parsed, err := parser.ParseFile(set, descriptors, nil, 0)
	if err != nil {
		t.Fatal(err)
	}
	for _, decl := range parsed.Decls {
		if gen, ok := decl.(*ast.GenDecl); ok && gen.Tok == token.CONST {
			for _, spec := range gen.Specs {
				value := spec.(*ast.ValueSpec)
				if kind, ok := value.Type.(*ast.SelectorExpr); ok && (kind.Sel.Name == "NodeTypeID" || kind.Sel.Name == "MarkKind") {
					for _, name := range value.Names {
						names[name.Name] = true
					}
				}
			}
		}
	}
	if len(names) != described {
		t.Fatalf("%s names %d kinds, describes %d", descriptors, len(names), described)
	}
	entries, err := os.ReadDir(".")
	if err != nil {
		t.Fatal(err)
	}
	for _, entry := range entries {
		file := entry.Name()
		if !strings.HasSuffix(file, ".go") || strings.HasSuffix(file, "_test.go") || file == descriptors {
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
		if desc.Group != nil && !desc.Prefix {
			t.Fatalf("%s: grouped blocks are prefixed", desc.Kind)
		}
		if desc.Prefix && desc.Marker == 0 {
			t.Fatalf("%s: a prefix has a marker the caret meets", desc.Kind)
		}
		if desc.Raise != RaiseNothing && desc.Glyphs == "" {
			t.Fatalf("%s raises by a glyph it has none of", desc.Kind)
		}
		if (desc.Box != nil) != desc.Checks {
			t.Fatalf("%s: a checked state is held in a box", desc.Kind)
		}
	}
}

// Each kind reads its own notation back as the shape it was written from,
// so notation and parsing are one grammar.
func TestEveryKindReadsItsOwnNotation(t *testing.T) {
	for _, desc := range kindTable {
		if desc.notation == nil || desc.Atomic {
			continue
		}
		for _, shape := range []blockShape{
			{kind: desc.Kind, level: 1},
			{kind: desc.Kind, level: 3, depth: 2, checked: true, info: "go"},
		} {
			if !desc.Leveled {
				shape.level = 0
			}
			if desc.Group == nil {
				shape.depth = 0
			}
			if !desc.Checks {
				shape.checked = false
			}
			if !desc.Info {
				shape.info = ""
			}
			open, _ := blockNotation(shape, 1)
			got, ok := parseOpener(open)
			if !ok || got != shape {
				t.Errorf("%s: %q reads back as %+v (%v), not %+v", desc.Kind, open, got, ok, shape)
			}
		}
	}
}
