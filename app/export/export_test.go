package export

import (
	"errors"
	"go/parser"
	"go/token"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"
	"time"

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

func paths(p Plan) []string {
	var out []string
	for _, f := range p.Files {
		out = append(out, f.Path)
	}
	return out
}

// A sheet is its Markdown in a file named by its title; its pictures are
// copied beside it, a picture used twice once, and its figures link them,
// to the assets folder it gets where one is there already.
func TestPlanSheetLinksItsPicturesWhereTheyGo(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "assets"), 0o755); err != nil {
		t.Fatal(err)
	}
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
		return Picture{Path: src, Name: "bay at dawn.png", Width: 4, Height: 3}, media == "ab12"
	}
	plan, err := PlanSheet(dir, sheet, pictures)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := paths(plan), []string{"Tides high low.md", "assets 2", "assets 2/bay at dawn.png"}; !slices.Equal(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	if plan.Files[1].Wanted != "assets" || !plan.Files[1].Assets || plan.Files[1].Pictures != 1 || plan.Files[2].Bytes != 3 {
		t.Fatalf("assets = %+v, picture = %+v", plan.Files[1], plan.Files[2])
	}
	want := "# Tides\n\n![Bay](<assets 2/bay at dawn.png>)\n\n![Again](<assets 2/bay at dawn.png>)\n\n![Lost](gone)\n"
	if text := plan.Text(plan.Files[0]); text != want {
		t.Fatalf("text = %q, want %q", text, want)
	}
	top, err := plan.Write(nil)
	if err != nil {
		t.Fatal(err)
	}
	if top != filepath.Join(dir, "Tides high low.md") || read(t, top) != want {
		t.Fatalf("wrote %s: %q", top, read(t, top))
	}
	if read(t, filepath.Join(dir, "assets 2", "bay at dawn.png")) != "png" {
		t.Fatal("the picture was not copied")
	}
}

// A group is a folder of its sheets, its groups folders within; a name
// taken at the destination or in the plan is numbered, and says so.
func TestPlanFolderMirrorsGroupsAndNumbersWhatIsTaken(t *testing.T) {
	dir := t.TempDir()
	if err := os.Mkdir(filepath.Join(dir, "Work"), 0o755); err != nil {
		t.Fatal(err)
	}
	folder := Folder{Name: "Work", Sheets: []Sheet{
		{Title: "Plan", Read: page(t, heading("Plan"))},
		{Title: "Plan", Read: page(t, heading("Plan"))},
		{Title: "...", Read: nil},
	}, Folders: []Folder{{Name: "Specs", Sheets: []Sheet{{Title: "API", Read: page(t, heading("API"))}}}}}
	plan, err := PlanFolder(dir, folder, nil)
	if err != nil {
		t.Fatal(err)
	}
	want := []string{"Work 2", "Work 2/Plan.md", "Work 2/Plan 2.md", "Work 2/_.md", "Work 2/Specs", "Work 2/Specs/API.md"}
	if got := paths(plan); !slices.Equal(got, want) {
		t.Fatalf("files = %v, want %v", got, want)
	}
	top := plan.Files[0]
	if top.Wanted != "Work" || top.Sheets != 4 || top.Folders != 1 || plan.Files[2].Wanted != "Plan.md" {
		t.Fatalf("folder = %+v, second plan = %+v", top, plan.Files[2])
	}
	if sheets, pictures, groups, _ := plan.Totals(); sheets != 4 || pictures != 0 || groups != 2 {
		t.Fatalf("totals = %d sheets, %d pictures, %d groups", sheets, pictures, groups)
	}
	var steps []int
	written, err := plan.Write(func(n int) { steps = append(steps, n) })
	if err != nil {
		t.Fatal(err)
	}
	if written != filepath.Join(dir, "Work 2") || len(steps) != len(plan.Files) {
		t.Fatalf("wrote %s in %v", written, steps)
	}
	if read(t, filepath.Join(dir, "Work 2", "Specs", "API.md")) != "# API\n" {
		t.Fatal("the group's group was not written")
	}
	// Written again, a plan whose names were taken since overwrites
	// nothing.
	if _, err := plan.Write(nil); !errors.Is(err, ErrTaken) {
		t.Fatalf("writing over what is there = %v, want ErrTaken", err)
	}
}

// Front matter heads a sheet's file with its title, times and tags, as
// YAML reads them back, only where the plan says.
func TestFrontMatterSaysTheSheet(t *testing.T) {
	at := time.Date(2026, 11, 20, 9, 14, 0, 0, time.UTC)
	sheet := Sheet{Title: "Plan: Q4", Created: at, Updated: at.Add(time.Hour), Tags: []string{"work", "two words", "yes"},
		Read: page(t, heading("Plan"))}
	plan, err := PlanSheet(t.TempDir(), sheet, nil)
	if err != nil {
		t.Fatal(err)
	}
	if text := plan.Text(plan.Files[0]); text != "# Plan\n" {
		t.Fatalf("without front matter = %q", text)
	}
	plan.FrontMatter = true
	want := "---\ntitle: \"Plan: Q4\"\ncreated: 2026-11-20T09:14:00Z\nupdated: 2026-11-20T10:14:00Z\n" +
		"tags:\n  - work\n  - two words\n  - \"yes\"\n---\n\n# Plan\n"
	if text := plan.Text(plan.Files[0]); text != want {
		t.Fatalf("front matter = %q, want %q", text, want)
	}
	if plan.Size(plan.Files[0]) != int64(len(want)) {
		t.Fatal("the size is not the text's")
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
