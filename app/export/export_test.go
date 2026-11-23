package export

import (
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/editor/vocab"
)

func page(t *testing.T, blocks ...*doc.Node) func() (*doc.Snapshot, error) {
	t.Helper()
	bare, err := doc.NewSnapshot(profile.Schema(), doc.NewInterner(), doc.NewNodeWithID(1, profile.Page, doc.Attrs{}, "", doc.BlockSlot()), 1)
	if err != nil {
		t.Fatal(err)
	}
	tx := bare.Begin(doc.Origin{Class: doc.OriginCommand})
	if err := tx.Apply(doc.Insert{Parent: 1, Nodes: blocks}); err != nil {
		t.Fatal(err)
	}
	snap, err := tx.Commit()
	if err != nil {
		t.Fatal(err)
	}
	return func() (*doc.Snapshot, error) { return snap, nil }
}

func heading(text string) *doc.Node {
	return vocab.NewBlock(profile.Schema(), profile.Heading, doc.Attrs{}, doc.TextItem(text, nil))
}

func read(t *testing.T, path string) string {
	t.Helper()
	data, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	return string(data)
}

// A sheet is its Markdown in a file named by its title; its pictures are
// copied beside it, a picture used twice once, and its figures link them.
func TestWriteSheetCopiesItsPicturesBesideIt(t *testing.T) {
	dir := t.TempDir()
	src := filepath.Join(t.TempDir(), "ab12.png")
	if err := os.WriteFile(src, []byte("png"), 0o644); err != nil {
		t.Fatal(err)
	}
	s := profile.Schema()
	sheet := Sheet{Title: "Tides: high/low", Read: page(t,
		heading("Tides"),
		profile.NewFigure(s, profile.Picture{Media: "ab12", Width: 4, Height: 3}, "Bay"),
		profile.NewFigure(s, profile.Picture{Media: "ab12", Width: 4, Height: 3}, "Again"),
		profile.NewFigure(s, profile.Picture{Media: "gone", Width: 4, Height: 3}, "Lost"),
	)}
	pictures := func(media string) (Picture, bool) {
		return Picture{Path: src, Name: "bay at dawn.png"}, media == "ab12"
	}
	got, err := WriteSheet(dir, sheet, pictures)
	if err != nil {
		t.Fatal(err)
	}
	if want := filepath.Join(dir, "Tides high low.md"); got.Path != want || got.Pictures != 1 {
		t.Fatalf("wrote %+v, want %s with one picture", got, want)
	}
	want := "# Tides\n\n![Bay](<assets/bay at dawn.png>)\n\n![Again](<assets/bay at dawn.png>)\n\n![Lost](gone)\n"
	if text := read(t, got.Path); text != want {
		t.Fatalf("text = %q, want %q", text, want)
	}
	if read(t, filepath.Join(dir, "assets", "bay at dawn.png")) != "png" {
		t.Fatal("the picture was not copied")
	}
}

// A group is a folder of its sheets, its groups folders within; nothing at
// the destination is overwritten, a name taken there numbered.
func TestWriteFolderMirrorsGroupsAndOverwritesNothing(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Work"), 0o755); err != nil {
		t.Fatal(err)
	}
	folder := Folder{Name: "Work", Sheets: []Sheet{
		{Title: "Plan", Read: page(t, heading("Plan"))},
		{Title: "Plan", Read: page(t, heading("Plan"))},
		{Title: "...", Read: nil},
	}, Folders: []Folder{{Name: "Specs", Sheets: []Sheet{{Title: "API", Read: page(t, heading("API"))}}}}}
	got, err := WriteFolder(dir, folder, nil)
	if err != nil {
		t.Fatal(err)
	}
	if got.Path != filepath.Join(dir, "Work 2") || got.Sheets != 4 {
		t.Fatalf("wrote %+v", got)
	}
	var files []string
	filepath.WalkDir(got.Path, func(path string, d os.DirEntry, _ error) error {
		if !d.IsDir() {
			rel, _ := filepath.Rel(got.Path, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(files)
	if want := []string{"Plan 2.md", "Plan.md", "Specs/API.md", "_.md"}; !slices.Equal(files, want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
}

// Export is a sink of the notebook's structure: it reads documents and
// writes files, and depends on nothing that holds the notebook or shows
// it, so what it writes is only what the host hands it.
func TestExportDependsOnlyOnStructure(t *testing.T) {
	allowed := []string{"app/markdown", "editor/doc"}
	for _, dir := range []string{".", "../markdown"} {
		files, _ := filepath.Glob(filepath.Join(dir, "*.go"))
		for _, file := range files {
			if strings.HasSuffix(file, "_test.go") {
				continue
			}
			f, err := parser.ParseFile(token.NewFileSet(), file, nil, parser.ImportsOnly)
			if err != nil {
				t.Fatal(err)
			}
			for _, imp := range f.Imports {
				p, ok := strings.CutPrefix(strings.Trim(imp.Path.Value, `"`), "github.com/caelyreth/neverland/")
				if !ok {
					continue
				}
				if dir == "../markdown" {
					allowed = []string{"editor/doc", "editor/profile", "editor/vocab"}
				}
				if !slices.Contains(allowed, p) {
					t.Errorf("%s imports %s: export and markdown read structure only", file, p)
				}
			}
		}
	}
}
