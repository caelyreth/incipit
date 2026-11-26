package window

import (
	"path/filepath"
	"testing"

	"github.com/caelyreth/neverland/internal/app/storage"
	"github.com/egoist/mygo"
)

func TestNotebookBaseByPlatform(t *testing.T) {
	xdg := filepath.Join(t.TempDir(), "xdg-data")
	docs := filepath.Join(t.TempDir(), "docs")
	t.Setenv(storage.HomeVariable, "")
	t.Setenv("XDG_DATA_HOME", xdg)
	mygo.App.SetPath(mygo.PathDocuments, docs)
	t.Cleanup(func() { mygo.App.SetPath(mygo.PathDocuments, "") })

	base, err := notebookBase("linux")
	if err != nil || base != xdg {
		t.Fatalf("notebookBase(\"linux\") = %q, %v; want %q", base, err, xdg)
	}
	root, err := storage.Root(base)
	if err != nil || root != filepath.Join(xdg, storage.FolderName) {
		t.Fatalf("linux root = %q, %v; want %q", root, err, filepath.Join(xdg, storage.FolderName))
	}

	for _, goos := range []string{"darwin", "windows"} {
		base, err := notebookBase(goos)
		if err != nil || base != docs {
			t.Fatalf("notebookBase(%q) = %q, %v; want %q", goos, base, err, docs)
		}
	}
}
