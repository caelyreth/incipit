package storage

import (
	"os"
	"path/filepath"
	"testing"
	"time"
)

// TestAWholeCloseKeepsItsLaunchsBackup pins the backup a launch took once
// its session closes whole, and pruning keeps it past every limit, so
// launches that crash can never rotate out the last state known to open.
func TestAWholeCloseKeepsItsLaunchsBackup(t *testing.T) {
	root := t.TempDir()
	openStore(t, root).Close()
	store := openStore(t, root)
	if store.launched == "" {
		t.Fatal("the launch took no backup")
	}
	launched := store.launched
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if got := (&Store{root: root}).pinned(); got != launched {
		t.Fatalf("pinned %q, want %q", got, launched)
	}

	// Many crashing launches later, the pinned backup stands.
	folder := filepath.Join(root, backupDir)
	newest := time.Now().Add(time.Hour)
	for index := range 3 * (recentBackups + dailyBackups) {
		taken := newest.Add(-time.Duration(index) * 6 * time.Hour)
		name := filepath.Join(folder, backupPrefix+taken.Format(backupLayout)+backupSuffix)
		if err := os.WriteFile(name, nil, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	old := backupPrefix + newest.Add(-400*24*time.Hour).Format(backupLayout) + backupSuffix
	if err := os.WriteFile(filepath.Join(folder, old), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(folder, pinnedName), []byte(old), 0o644); err != nil {
		t.Fatal(err)
	}
	if err := (&Store{root: root}).prune(); err != nil {
		t.Fatal(err)
	}
	if _, err := os.Stat(filepath.Join(folder, old)); err != nil {
		t.Fatal("pruning let the pinned backup go:", err)
	}
}
