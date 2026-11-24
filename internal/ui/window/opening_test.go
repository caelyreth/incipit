package window

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// TestASheetThatTookTheAppDownIsLeftClosed opens a sheet, then finds its
// mark at the next launch, as a crash before any frame was drawn with it
// would leave it: the launch leaves the sheet closed, forgets where it
// stood, and says so. A frame drawn, or a whole quit, lets the mark go.
func TestASheetThatTookTheAppDownIsLeftClosed(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	nameActivePage(t, app, "Tides")
	if app.pageElement() == nil {
		t.Fatal("no editor")
	}
	mark := filepath.Join(root, openingName)
	left, err := os.ReadFile(mark)
	if err != nil {
		t.Fatal("opening left no mark:", err)
	}
	app.save()
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(mark); !os.IsNotExist(err) {
		t.Fatal("a whole quit kept the mark")
	}
	// The crash: the mark as the opening left it.
	if err := os.WriteFile(mark, left, 0o644); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	if opened := app.Workspace.Opened(); len(opened) != 0 {
		t.Fatalf("reopened %v", opened)
	}
	if _, err := os.Stat(mark); !os.IsNotExist(err) {
		t.Fatal("the mark outlived the launch that read it")
	}
	var told string
	for _, change := range app.Notices.take() {
		if change.notice.ID == "opening.crashed" {
			told = change.notice.Description
		}
	}
	if !strings.Contains(told, "Tides") {
		t.Fatalf("told %q", told)
	}
	app.newSheet()
	app.pageElement()
	app.openingDrawn()
	if _, err := os.Stat(mark); !os.IsNotExist(err) {
		t.Fatal("a frame drawn kept the mark")
	}
}
