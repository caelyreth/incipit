package manifest

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
)

const repoModule = "github.com/caelyreth/neverland/"

// TestD14ImportBoundary checks the D14 dependency contract across the repo:
//   - internal/editor/doc imports nothing internal.
//   - Nothing under internal/editor imports internal/app or internal/ui.
//   - Format converters (internal/app/markdown, internal/app/html) import only
//     doc, vocab, and profile (never engine, policy, policies, storage, or ui).
//   - internal/app/clipboard imports only doc, vocab, profile, store and
//     the two readers it chooses between (markdown, html).
func TestD14ImportBoundary(t *testing.T) {
	root := filepath.Join("..", "..", "..")
	checks := []struct {
		dir        string
		allowed    []string
		forbidApp  bool
		forbidUI   bool
		exactAllow bool
	}{
		{dir: "internal/editor/doc", allowed: nil, exactAllow: true},
		{dir: "internal/editor", forbidApp: true, forbidUI: true},
		{dir: "internal/app/markdown", allowed: []string{"internal/editor/doc", "internal/editor/profile", "internal/editor/vocab"}, exactAllow: true},
		{dir: "internal/app/html", allowed: []string{"internal/editor/doc", "internal/editor/profile", "internal/editor/vocab"}, exactAllow: true},
		{dir: "internal/app/clipboard", allowed: []string{"internal/app/html", "internal/app/markdown", "internal/editor/doc", "internal/editor/profile", "internal/editor/store", "internal/editor/vocab"}, exactAllow: true},
	}

	for _, c := range checks {
		abs := filepath.Join(root, c.dir)
		if _, err := os.Stat(abs); os.IsNotExist(err) {
			continue
		}
		err := filepath.WalkDir(abs, func(path string, d os.DirEntry, err error) error {
			if err != nil || d.IsDir() || !strings.HasSuffix(path, ".go") || strings.HasSuffix(path, "_test.go") {
				return err
			}
			f, err := parser.ParseFile(token.NewFileSet(), path, nil, parser.ImportsOnly)
			if err != nil {
				return err
			}
			rel, _ := filepath.Rel(root, path)
			for _, imp := range f.Imports {
				p := strings.Trim(imp.Path.Value, `"`)
				dep, ok := strings.CutPrefix(p, repoModule)
				if !ok {
					continue
				}
				if c.forbidApp && strings.HasPrefix(dep, "internal/app/") {
					t.Errorf("%s imports %s (D14 forbids internal/editor -> internal/app)", filepath.ToSlash(rel), dep)
				}
				if c.forbidUI && strings.HasPrefix(dep, "internal/ui/") {
					t.Errorf("%s imports %s (D14 forbids internal/editor -> internal/ui)", filepath.ToSlash(rel), dep)
				}
				if c.exactAllow && !slices.Contains(c.allowed, dep) {
					t.Errorf("%s imports %s (D14 allows only %v)", filepath.ToSlash(rel), dep, c.allowed)
				}
			}
			return nil
		})
		if err != nil {
			t.Fatal(err)
		}
	}
}
