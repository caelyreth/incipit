package window

import (
	"path/filepath"
	"runtime"
	"testing"

	appsettings "github.com/caelyreth/neverland/internal/app/settings"
	"github.com/caelyreth/neverland/internal/app/storage"
	"github.com/egoist/mygo"
)

// TestNotebookRootFirstLaunch offers the default folder, not chosen, until
// a folder is recorded, and takes NEVERLAND_HOME as chosen.
func TestNotebookRootFirstLaunch(t *testing.T) {
	userData := t.TempDir()
	docs := t.TempDir()
	t.Setenv(storage.HomeVariable, "")
	t.Setenv("XDG_DATA_HOME", docs)
	mygo.App.SetPath(mygo.PathDocuments, docs)
	t.Cleanup(func() { mygo.App.SetPath(mygo.PathDocuments, "") })

	root, chosen, err := notebookRoot(userData)
	if err != nil || chosen || root != filepath.Join(docs, storage.FolderName(runtime.GOOS)) {
		t.Fatalf("first launch: %q, chosen %v, %v; want the default, not chosen", root, chosen, err)
	}
	elsewhere := filepath.Join(t.TempDir(), "Notes")
	if err := storage.ChooseRoot(userData, elsewhere); err != nil {
		t.Fatal(err)
	}
	if root, chosen, err := notebookRoot(userData); err != nil || !chosen || root != elsewhere {
		t.Fatalf("after choosing: %q, chosen %v, %v; want %q", root, chosen, err, elsewhere)
	}
	home := t.TempDir()
	t.Setenv(storage.HomeVariable, home)
	if root, chosen, err := notebookRoot(userData); err != nil || !chosen || root != home {
		t.Fatalf("with %s: %q, chosen %v, %v; want %q", storage.HomeVariable, root, chosen, err, home)
	}
}

// TestWalkthroughChoicesPersist keeps what the walkthrough chose in the
// notebook's settings record, as the walkthrough's end applies it. (A
// theme's change moves the system's chrome, which waits on a running app:
// policy choices take the same path to the record.)
func TestWalkthroughChoicesPersist(t *testing.T) {
	root, userData := t.TempDir(), t.TempDir()
	app, err := openApp(root, userData)
	if err != nil {
		t.Fatal(err)
	}
	for _, change := range []appsettings.Change{
		appsettings.SetOption("notation.reveal", "hidden"),
		appsettings.SetOption("pairs.autoClose", false),
	} {
		app.Settings.Apply(change)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	again, err := openApp(root, userData)
	if err != nil {
		t.Fatal(err)
	}
	defer again.Close()
	if got := again.Product.Policy["notation.reveal"]; got != "hidden" {
		t.Errorf("notation.reveal reopened as %v, want hidden", got)
	}
	if got := again.Product.Policy["pairs.autoClose"]; got != false {
		t.Errorf("pairs.autoClose reopened as %v, want false", got)
	}
}
