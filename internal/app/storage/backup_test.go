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

func TestZeroByteDatabaseRecoversFromBackup(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	saver := store.Autosave()
	saver.Page(1, Summary{Title: "important note"})
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if !Holds(root) {
		t.Fatal("Holds = false for valid notebook")
	}

	// Simulate crash/sync truncation leaving a 0-byte database file.
	if err := os.WriteFile(filepath.Join(root, databaseName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if !Holds(root) {
		t.Fatal("Holds = false when verified backup exists for 0-byte db")
	}
	reopened := openStore(t, root)
	defer reopened.Close()
	if !reopened.Recovery.Repaired() || reopened.Recovery.Restored == "" {
		t.Fatalf("expected backup recovery for 0-byte db, got %+v", reopened.Recovery)
	}
	loaded, err := reopened.Load()
	if err != nil || len(loaded.Pages) != 1 || loaded.Pages[0].Title != "important note" {
		t.Fatalf("restored pages = %+v, err = %v", loaded.Pages, err)
	}
}

func TestListBackupsSkipsNewerSchemaAndRestoreBackupWorks(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	saver := store.Autosave()
	saver.Page(1, Summary{Title: "v1 state"})
	if err := saver.Close(); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}

	list, err := ListBackups(root)
	if err != nil || len(list) == 0 {
		t.Fatalf("ListBackups = %+v, %v", list, err)
	}
	// Corrupt live DB content logically (delete pages) and verify RestoreBackup brings back v1 state.
	live := openStore(t, root)
	if _, err := live.db.Exec(`DELETE FROM pages`); err != nil {
		t.Fatal(err)
	}
	if err := live.Close(); err != nil {
		t.Fatal(err)
	}

	if err := RestoreBackup(root, list[0].Path); err != nil {
		t.Fatalf("RestoreBackup: %v", err)
	}
	restored := openStore(t, root)
	defer restored.Close()
	loaded, err := restored.Load()
	if err != nil || len(loaded.Pages) != 1 || loaded.Pages[0].Title != "v1 state" {
		t.Fatalf("after RestoreBackup pages = %+v, err = %v", loaded.Pages, err)
	}
}

func TestEmptyZeroByteDirWithoutBackupsNotTreatedAsNotebook(t *testing.T) {
	root := t.TempDir()
	if err := os.WriteFile(filepath.Join(root, databaseName), nil, 0o644); err != nil {
		t.Fatal(err)
	}
	if Holds(root) {
		t.Fatal("Holds = true for empty 0-byte file with no backups")
	}
}

func TestListBackupsSkipsNewerSchemaVersion(t *testing.T) {
	root := t.TempDir()
	openStore(t, root).Close()
	store := openStore(t, root)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	list, err := ListBackups(root)
	if err != nil || len(list) == 0 {
		t.Fatalf("ListBackups = %+v, %v", list, err)
	}
	// Copy backup to an older timestamp and bump its PRAGMA user_version past len(migrations).
	newerPath := filepath.Join(root, backupDir, backupPrefix+time.Now().Add(-time.Hour).Format(backupLayout)+backupSuffix)
	if err := copyDurably(list[0].Path, newerPath); err != nil {
		t.Fatal(err)
	}
	db, err := openDatabase(newerPath)
	if err != nil {
		t.Fatal(err)
	}
	_, execErr := db.Exec(`PRAGMA user_version = 999`)
	_ = db.Close()
	if execErr != nil {
		t.Fatal(execErr)
	}

	filtered, err := ListBackups(root)
	if err != nil {
		t.Fatal(err)
	}
	for _, b := range filtered {
		if b.Path == newerPath {
			t.Fatalf("ListBackups returned backup with newer schema version %q", newerPath)
		}
	}
}
