package editor

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

const module = "github.com/caelyreth/incipit/"

// allowed is §2.1's dependency table: what each editor module may import
// from the editor. Policies depend on vocab and policy, never on each
// other; only the kind-aware policies name the profile's kinds. The
// framework and the engine read kinds through vocab.Profile. The renderer
// and the app are outside and use the engine API.
var allowed = map[string][]string{
	"internal/editor/doc":     {},
	"internal/editor/vocab":   {"internal/editor/doc", "internal/text/wordseg"},
	"internal/editor/profile": {"internal/editor/doc", "internal/editor/vocab"},
	"internal/editor/policy":  {"internal/editor/doc", "internal/editor/vocab"},
	"internal/editor/present": {"internal/editor/doc", "internal/editor/vocab", "internal/editor/policy"},
	"internal/editor/store":   {"internal/editor/doc", "internal/editor/vocab"},
	"internal/editor/engine":  {"internal/editor/doc", "internal/editor/vocab", "internal/editor/policy", "internal/editor/present"},
	"policies":                {"internal/editor/doc", "internal/editor/vocab", "internal/editor/policy", "internal/text/query"},
	"profiled":                {"internal/editor/doc", "internal/editor/vocab", "internal/editor/policy", "internal/editor/profile", "internal/text/query"},
}

// profiled are the policies whose behaviour is about the profile's kinds.
var profiled = map[string]bool{
	"blocks": true, "find": true, "figures": true, "formats": true, "formulas": true,
	"lists": true, "markdown": true, "notation": true, "spelling": true, "tables": true, "tags": true, "words": true,
}

func TestImportsFollowTheModuleTable(t *testing.T) {
	err := filepath.WalkDir(".", func(path string, d os.DirEntry, err error) error {
		if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
			return err
		}
		dir := filepath.ToSlash(filepath.Dir(path))
		if dir == "." {
			return nil
		}
		pkg := "internal/editor/" + dir
		rule := pkg
		if name, ok := strings.CutPrefix(pkg, "internal/editor/policies/"); ok {
			rule = "policies"
			if profiled[name] {
				rule = "profiled"
			}
		}
		ok, known := allowed[rule]
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
			if strings.HasPrefix(dep, "internal/app/") || strings.HasPrefix(dep, "internal/ui/") {
				t.Errorf("%s imports %s: nothing under internal/editor may import internal/app or internal/ui", path, dep)
			}
			if !known {
				continue
			}
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
