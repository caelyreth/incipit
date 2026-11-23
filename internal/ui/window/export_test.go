package window

import (
	"os"
	"path/filepath"
	"slices"
	"testing"

	"github.com/caelyreth/neverland/internal/app/export"
	"github.com/caelyreth/neverland/internal/ui/shell"
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
	plan, err := export.PlanFolder(dir, job.folder, nil)
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
