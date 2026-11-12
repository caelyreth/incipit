package editor

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/caelyreth/neverland/"

// allowed is §2.1's dependency table: what each editor module may import
// from the editor. Policies depend on vocab and policy, never on each
// other; only the kind-aware policies name the profile's kinds. The
// framework and the engine read kinds through vocab.Profile. The renderer
// and the app are outside and use the engine API.
var allowed = map[string][]string{
	"editor/doc":     {},
	"editor/vocab":   {"editor/doc", "internal/wordseg"},
	"editor/profile": {"editor/doc", "editor/vocab"},
	"editor/policy":  {"editor/doc", "editor/vocab"},
	"editor/present": {"editor/doc", "editor/vocab", "editor/policy"},
	"editor/store":   {"editor/doc", "editor/vocab"},
	"editor/engine":  {"editor/doc", "editor/vocab", "editor/policy", "editor/present"},
	"policies":       {"editor/doc", "editor/vocab", "editor/policy"},
	"profiled":       {"editor/doc", "editor/vocab", "editor/policy", "editor/profile"},
}

// profiled are the policies whose behaviour is about the profile's kinds.
var profiled = map[string]bool{"blocks": true, "figures": true, "formats": true, "formulas": true, "lists": true, "markdown": true, "notation": true, "tables": true}

func TestImportsFollowTheModuleTable(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		pkg := "editor/" + filepath.ToSlash(filepath.Dir(path))
		rule := pkg
		if name, ok := strings.CutPrefix(pkg, "editor/policies/"); ok {
			rule = "policies"
			if profiled[name] {
				rule = "profiled"
			}
		}
		ok, known := allowed[rule]
		if !known {
			return nil
		}
		f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
		if err != nil {
			return err
		}
		for _, imp := range f.Imports {
			p := strings.Trim(imp.Path.Value, `"`)
			if !strings.HasPrefix(p, module) {
				continue
			}
			dep := strings.TrimPrefix(p, module)
			allowedDep := false
			for _, a := range ok {
				allowedDep = allowedDep || dep == a
			}
			if !allowedDep {
				t.Errorf("%s imports %s, which §2.1 forbids", path, dep)
			}
		}
		return nil
	})
	if err != nil {
		t.Fatal(err)
	}
}
