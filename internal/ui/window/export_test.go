package window

import (
	"bytes"
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/caelyreth/neverland/internal/app/export"
	"github.com/caelyreth/neverland/internal/editor/doc"
	"github.com/caelyreth/neverland/internal/editor/profile"
	"github.com/caelyreth/neverland/internal/ui/shell"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo"
	"github.com/egoist/mygo/ui"
)

// A group exports as a folder of its sheets, its groups folders within,
// each sheet read as it stands, open or not.
func TestExportingAGroupWritesItsSheetsAndGroups(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	work := app.NewGroup("", shell.Last)
	app.RenameGroup(work, "Work")
	specs := app.NewGroup(work, shell.Last)
	app.RenameGroup(specs, "Specs")
	app.NewSheet(work, shell.Last)
	nameActivePage(t, app, "Plan")
	app.NewSheet(specs, shell.Last)
	nameActivePage(t, app, "")

	job, ok := app.exportJob("group:" + work)
	if !ok || job.name != "Work" || job.sheets != 2 {
		t.Fatalf("job = %+v, %v", job, ok)
	}
	if plan := job.folder.Sheets[0]; plan.Title != "Plan" || plan.Created.IsZero() || plan.Updated.IsZero() {
		t.Fatalf("the sheet goes without its times: %+v", plan)
	}
	dir := t.TempDir()
	plan, err := export.PlanFolder(dir, job.folder, nil, nil)
	if err != nil {
		t.Fatal(err)
	}
	top, err := plan.Write(nil)
	if err != nil {
		t.Fatal(err)
	}
	var files []string
	filepath.WalkDir(top, func(path string, d os.DirEntry, _ error) error {
		if !d.IsDir() {
			rel, _ := filepath.Rel(dir, path)
			files = append(files, filepath.ToSlash(rel))
		}
		return nil
	})
	slices.Sort(files)
	if want := []string{"Work/Plan.md", "Work/Specs/Untitled.md"}; !slices.Equal(files, want) {
		t.Fatalf("files = %v, want %v", files, want)
	}
	if _, ok := app.exportJob("group:404"); ok {
		t.Fatal("a group the library does not hold exports")
	}
}

// As PDF, an export typesets each sheet as its page reads, even one that
// holds nothing.
func TestAnExportTypesetsSheetsAsPDF(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	app.exportPDF = true
	typeset := app.exportTypeset()
	if typeset == nil {
		t.Fatal("no typesetter for PDF")
	}
	snap, err := profile.EmptyPage(1, 1, doc.NewInterner())
	if err != nil {
		t.Fatal(err)
	}
	for _, s := range []*doc.Snapshot{snap, nil} {
		out, pages, err := typeset(s, "Sheet")
		if err != nil || pages != 1 || !bytes.HasPrefix(out, []byte("%PDF-")) {
			t.Fatalf("%d pages, %d bytes: %v", pages, len(out), err)
		}
	}
	app.exportPDF = false
	if app.exportTypeset() != nil {
		t.Fatal("Markdown is typeset")
	}
}

func TestExportSummaryReflectsPDFModeAndDefaultDirFallback(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	job := &exportJob{name: "Notes", sheets: 2}
	app.exportPDF = false
	if got := app.exportSummary(job); got != locales.ExportMarkdown()+" · "+locales.ExportReading(2) {
		t.Fatalf("markdown summary = %q", got)
	}
	app.exportPDF = true
	if got := app.exportSummary(job); got != locales.Pdf()+" · "+locales.ExportReading(2) {
		t.Fatalf("pdf summary = %q", got)
	}

	docs := t.TempDir()
	mygo.App.SetPath(mygo.PathDesktop, filepath.Join(t.TempDir(), "no-desktop"))
	mygo.App.SetPath(mygo.PathDocuments, docs)
	t.Cleanup(func() {
		mygo.App.SetPath(mygo.PathDesktop, "")
		mygo.App.SetPath(mygo.PathDocuments, "")
	})
	if got := defaultExportDir(); got != docs {
		t.Fatalf("defaultExportDir = %q, want fallback %q", got, docs)
	}
}

func TestExportViewBoundsWithLongFolderName(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	longDir := filepath.Join(t.TempDir(), "VeryLongUnbrokenExportDestinationFolderName")
	longFolder := "VeryLongUnbrokenSubfolderNameForPreviewBoundsVerification"
	app.exporting = &exportJob{
		name:  "Work",
		group: true,
		dir:   longDir,
		shown: longFolder,
		plan: &export.Plan{
			Files: []export.File{
				{Path: longFolder, Kind: export.KindFolder, Sheets: 1},
			},
		},
	}

	tester := ui.NewTester(app.exportView, 900, 620)
	tester.Frame()

	place, ok := tester.Access(locales.ExportPlace())
	if !ok || place.Bounds.W <= 0 || place.Bounds.X < 24 || place.Bounds.X+place.Bounds.W > 344 {
		t.Fatalf("export-place button %+v (ok=%v) escaped files panel", place.Bounds, ok)
	}
	if !tester.HasText(locales.ExportFiles()) {
		t.Fatalf("missing %q in export texts: %v", locales.ExportFiles(), tester.Texts())
	}
	metaRect, ok := tester.Find(locales.SheetsCount(1))
	if !ok || metaRect.X < 356 || metaRect.X+metaRect.W > 900-24 {
		t.Fatalf("preview folder meta %+v (ok=%v) escaped preview panel", metaRect, ok)
	}
}
