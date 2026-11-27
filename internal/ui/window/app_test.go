package window

import (
	"os"
	"path/filepath"
	"testing"

	"github.com/caelyreth/incipit/internal/app/storage"
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
	root, err := storage.Root(base, "linux")
	if want := filepath.Join(xdg, "incipit"); err != nil || root != want {
		t.Fatalf("linux root = %q, %v; want %q", root, err, want)
	}

	for _, goos := range []string{"darwin", "windows"} {
		base, err := notebookBase(goos)
		if err != nil || base != docs {
			t.Fatalf("notebookBase(%q) = %q, %v; want %q", goos, base, err, docs)
		}
		if root, err := storage.Root(base, goos); err != nil || root != filepath.Join(docs, "Incipit") {
			t.Fatalf("%s root = %q, %v; want %q", goos, root, err, filepath.Join(docs, "Incipit"))
		}
	}
}

// TestAppFoldersOnLinux names the app's data, cache and logs folders in
// lowercase on Linux, in the user's config and cache folders.
func TestAppFoldersOnLinux(t *testing.T) {
	t.Setenv("HOME", t.TempDir())
	t.Setenv("XDG_CONFIG_HOME", filepath.Join(t.TempDir(), "config"))
	t.Setenv("XDG_CACHE_HOME", filepath.Join(t.TempDir(), "cache"))
	config, _ := os.UserConfigDir()
	cache, _ := os.UserCacheDir()
	want := map[mygo.PathName]string{
		mygo.PathUserData: filepath.Join(config, "incipit"),
		mygo.PathLogs:     filepath.Join(config, "incipit", "logs"),
		mygo.PathCache:    filepath.Join(cache, "incipit"),
	}
	for name := range want {
		t.Cleanup(func() { mygo.App.SetPath(name, "") })
	}
	appFolders("linux")
	for name, path := range want {
		if got, _ := mygo.App.Path(name); got != path {
			t.Errorf("%s = %q, want %q", name, got, path)
		}
	}
}
