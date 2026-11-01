package storage

import (
	"errors"
	"io"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func TestPreparedPublicationPreservesDestinationOnFailure(t *testing.T) {
	for _, test := range []struct {
		name   string
		failAt string
	}{
		{name: "permissions", failAt: "chmod"},
		{name: "write", failAt: "write"},
		{name: "sync", failAt: "sync"},
		{name: "close", failAt: "close"},
		{name: "verification", failAt: "verify"},
		{name: "rename", failAt: "rename"},
	} {
		t.Run(test.name, func(t *testing.T) {
			root := t.TempDir()
			target := filepath.Join(root, "page.txt")
			if err := os.WriteFile(target, []byte("old destination"), 0o600); err != nil {
				t.Fatal(err)
			}
			sentinel := errors.New(test.name + " failed")
			ops := defaultPublicationOps()
			write := func(target io.Writer) error {
				_, err := io.WriteString(target, "new contents")
				return err
			}
			var verify func(string) error
			switch test.failAt {
			case "chmod":
				ops.chmod = func(string, os.FileMode) error { return sentinel }
			case "write":
				write = func(io.Writer) error { return sentinel }
			case "sync":
				ops.syncOpen = func(*os.File) error { return sentinel }
			case "close":
				ops.close = func(file *os.File) error {
					if err := file.Close(); err != nil {
						return err
					}
					return sentinel
				}
			case "verify":
				verify = func(string) error { return sentinel }
			case "rename":
				ops.rename = func(string, string) error { return sentinel }
			}
			err := writePreparedFileWith(target, write, verify, ops)
			if !errors.Is(err, sentinel) {
				t.Fatalf("error = %v, want %v", err, sentinel)
			}
			contents, err := os.ReadFile(target)
			if err != nil || string(contents) != "old destination" {
				t.Fatalf("destination = %q, %v", contents, err)
			}
			entries, err := os.ReadDir(root)
			if err != nil {
				t.Fatal(err)
			}
			if len(entries) != 1 || entries[0].Name() != filepath.Base(target) {
				t.Fatalf("staging files remain: %v", entries)
			}
		})
	}
}

func TestNewPreparedFileDoesNotBroadenTemporaryPermissions(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission modes are not portable to Windows")
	}
	target := filepath.Join(t.TempDir(), "new-file")
	if err := writePreparedFile(target, func(w io.Writer) error {
		_, err := io.WriteString(w, "private")
		return err
	}, nil); err != nil {
		t.Fatal(err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if mode := info.Mode().Perm(); mode&0o077 != 0 {
		t.Fatalf("new file mode = %o; temporary file permissions were broadened", mode)
	}
}

func TestPreparedPublicationPreservesExistingMode(t *testing.T) {
	if runtime.GOOS == "windows" {
		t.Skip("Unix permission modes are not portable to Windows")
	}
	root := t.TempDir()
	target := filepath.Join(root, "page.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	err := writePreparedFile(target, func(target io.Writer) error {
		_, err := io.WriteString(target, "new")
		return err
	}, nil)
	if err != nil {
		t.Fatal(err)
	}
	contents, err := os.ReadFile(target)
	if err != nil || string(contents) != "new" {
		t.Fatalf("contents = %q, %v", contents, err)
	}
	info, err := os.Stat(target)
	if err != nil {
		t.Fatal(err)
	}
	if got := info.Mode().Perm(); got != 0o600 {
		t.Fatalf("mode = %o, want 600", got)
	}
}

func TestPreparedPublicationReportsCleanupFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "page.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	verifyErr := errors.New("verification failed")
	cleanupErr := errors.New("cleanup failed")
	ops := defaultPublicationOps()
	ops.remove = func(string) error { return cleanupErr }
	err := writePreparedFileWith(target, func(target io.Writer) error {
		_, err := io.WriteString(target, "new")
		return err
	}, func(string) error {
		return verifyErr
	}, ops)
	if !errors.Is(err, verifyErr) || !errors.Is(err, cleanupErr) {
		t.Fatalf("error = %v", err)
	}
	contents, readErr := os.ReadFile(target)
	if readErr != nil || string(contents) != "old" {
		t.Fatalf("destination = %q, %v", contents, readErr)
	}
	entries, readErr := os.ReadDir(root)
	if readErr != nil {
		t.Fatal(readErr)
	}
	for _, entry := range entries {
		if entry.Name() != filepath.Base(target) {
			if err := os.Remove(filepath.Join(root, entry.Name())); err != nil {
				t.Fatal(err)
			}
		}
	}
}

func TestDirectorySyncFailureOccursAfterAtomicReplacement(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "page.txt")
	if err := os.WriteFile(target, []byte("old"), 0o600); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("directory sync failed")
	ops := defaultPublicationOps()
	ops.syncDir = func(string) error { return sentinel }
	err := writePreparedFileWith(target, func(target io.Writer) error {
		_, err := io.WriteString(target, "new")
		return err
	}, nil, ops)
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	contents, readErr := os.ReadFile(target)
	if readErr != nil || string(contents) != "new" {
		t.Fatalf("destination = %q, %v", contents, readErr)
	}
}

func TestExternallyPreparedFileKeepsOldDestinationUntilVerified(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "notebook.db")
	if err := os.WriteFile(target, []byte("old database"), 0o600); err != nil {
		t.Fatal(err)
	}
	temp := filepath.Join(root, "notebook.db.partial")
	if err := os.WriteFile(temp, []byte("new database"), 0o644); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("database verification failed")
	err := completePreparedFile(target, temp, func(string) error { return sentinel })
	if !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	contents, readErr := os.ReadFile(target)
	if readErr != nil || string(contents) != "old database" {
		t.Fatalf("destination = %q, %v", contents, readErr)
	}
	if _, statErr := os.Stat(temp); !errors.Is(statErr, os.ErrNotExist) {
		t.Fatalf("failed staged database remains: %v", statErr)
	}
}

func TestRescueReplacementDoesNotTruncateExistingFileOnFailure(t *testing.T) {
	root := t.TempDir()
	target := filepath.Join(root, "page-1.txt")
	if err := os.WriteFile(target, []byte("old rescue text"), 0o600); err != nil {
		t.Fatal(err)
	}
	sentinel := errors.New("rename failed")
	ops := defaultPublicationOps()
	ops.rename = func(string, string) error { return sentinel }
	if err := writeDurablyWith(target, "new rescue text", ops); !errors.Is(err, sentinel) {
		t.Fatalf("error = %v", err)
	}
	contents, err := os.ReadFile(target)
	if err != nil || string(contents) != "old rescue text" {
		t.Fatalf("destination = %q, %v", contents, err)
	}
}

func TestCloseReportsBothSaveAndRescueFailure(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	saver := store.Autosave(nil)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(filepath.Join(root, recoveryDir), []byte("not a directory"), 0o600); err != nil {
		t.Fatal(err)
	}
	saver.Page(1, "", page(t, "unsaved").Snapshot())
	err := saver.Close()
	if err == nil || !strings.Contains(err.Error(), "database is closed") || !strings.Contains(err.Error(), "rescue") {
		t.Fatalf("close error = %v; want both save and rescue failure", err)
	}
}

func TestCloseKeepsUnsavedPageInRescueFile(t *testing.T) {
	root := t.TempDir()
	store := openStore(t, root)
	saver := store.Autosave(nil)
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	saver.Page(1, "", page(t, "unsaved words").Snapshot())
	if err := saver.Close(); err == nil || !strings.Contains(err.Error(), "database is closed") || strings.Contains(err.Error(), "rescue") {
		t.Fatalf("close error = %v, want save failure", err)
	}
	files, err := filepath.Glob(filepath.Join(root, recoveryDir, "unsaved-*", "page-1.txt"))
	if err != nil || len(files) != 1 {
		t.Fatalf("rescue files = %v, error = %v", files, err)
	}
	contents, err := os.ReadFile(files[0])
	if err != nil || !strings.Contains(string(contents), "unsaved words") {
		t.Fatalf("rescue contents = %q, error = %v", contents, err)
	}
}
