package window

import (
	"errors"
	"os"
	"path/filepath"
	"sync"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/engine/editor"
	"github.com/caelyreth/neverland/engine/kernel"
	"github.com/egoist/mygo/ui"
)

// What fails anywhere in the window shows as a toast with what failed
// and why, and nowhere else.
func TestAFailureShowsAsAToast(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	app.fail("Couldn't move the sheet", errors.New("the group is in the trash"))
	tester.Frame()
	if !tester.HasText("Couldn't move the sheet") || !tester.HasText("The group is in the trash") {
		t.Fatalf("the failure did not show: %v", tester.Texts())
	}
	if title, _ := tester.Find("Couldn't move the sheet"); title.X < 1260/2 || title.Y < 850/2 {
		t.Fatalf("the toast is at %+v, not at the bottom right", title)
	}
}

// fakeSaver fails, or not, as a test says.
type fakeSaver struct {
	mu             sync.Mutex
	err, backupErr error
	flushed        chan struct{}
}

func (f *fakeSaver) Err() error       { f.mu.Lock(); defer f.mu.Unlock(); return f.err }
func (f *fakeSaver) BackupErr() error { f.mu.Lock(); defer f.mu.Unlock(); return f.backupErr }
func (f *fakeSaver) Flush() error {
	defer func() { f.flushed <- struct{}{} }()
	return f.Err()
}

func (f *fakeSaver) fail(err, backupErr error) {
	f.mu.Lock()
	f.err, f.backupErr = err, backupErr
	f.mu.Unlock()
}

// Saving failing shows once, until it is saved again, which says so; Retry
// Now writes at once, and a write failing as before shows again.
func TestSavingFailingShowsUntilItIsSavedAgain(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	writer := &fakeSaver{flushed: make(chan struct{}, 1)}
	var told saving
	tell := func() {
		app.tellSaving(&told, writer)
		tester.Frame()
	}
	writer.fail(errors.New("disk full"), nil)
	tell()
	if !tester.HasText("Changes aren't saved yet") || !tester.HasText("Disk full") {
		t.Fatalf("saving failing did not show: %v", tester.Texts())
	}
	// It stays, however long the saver goes on failing.
	tester.Advance(time.Minute)
	tell()
	if !tester.HasText("Changes aren't saved yet") {
		t.Fatal("the failure went while saving still failed")
	}
	if err := tester.Click("Retry Now"); err != nil {
		t.Fatal(err)
	}
	<-writer.flushed
	tester.Frame()
	if !tester.HasText("Changes aren't saved yet") {
		t.Fatalf("a retry failing as before did not show again: %v", tester.Texts())
	}
	writer.fail(nil, nil)
	tell()
	if tester.HasText("Changes aren't saved yet") || !tester.HasText("All changes saved") {
		t.Fatalf("saving again did not say so: %v", tester.Texts())
	}
	// The time stops while the pointer rests on the toasts, as after the
	// click.
	tester.Move(10, 10)
	tester.Advance(10 * time.Second)
	if tester.HasText("All changes saved") {
		t.Fatal("the news of saving stayed")
	}
	tell()
	if tester.HasText("All changes saved") {
		t.Fatal("saving well was told again")
	}
}

// A backup failing warns once, and goes when backups work again.
func TestABackupFailingWarnsOnce(t *testing.T) {
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	writer := &fakeSaver{}
	var told saving
	writer.fail(nil, errors.New("backups folder is read-only"))
	app.tellSaving(&told, writer)
	tester.Frame()
	if !tester.HasText("Couldn't back up the notebook") {
		t.Fatalf("the backup failing did not show: %v", tester.Texts())
	}
	tester.Advance(10 * time.Second)
	app.tellSaving(&told, writer)
	tester.Frame()
	if tester.HasText("Couldn't back up the notebook") {
		t.Fatal("the same backup failure warned again")
	}
}

// A notebook opened from its backup says so until it is closed, and shows
// where the damaged one is kept.
func TestARepairedNotebookSaysSo(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	typeText(t, app, "kept in the backup")
	app.save()
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	garbage := make([]byte, 8192)
	for index := range garbage {
		garbage[index] = byte(index * 31)
	}
	if err := os.WriteFile(filepath.Join(root, "notebook.db"), garbage, 0o644); err != nil {
		t.Fatal(err)
	}
	app = openNotebookApp(t, root)
	defer app.Close()
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	tester.Advance(time.Minute)
	if !tester.HasText("The notebook was restored from its latest backup") || !tester.HasText("Show in Finder") {
		t.Fatalf("the repair was not told: %v", tester.Texts())
	}
}

// An edit that fails is told once; one the page declines is no fault.
func TestAFailedEditIsToldAndADeclinedOneIsNot(t *testing.T) {
	if !controller.Declined(editor.ErrCompositionActive) || !controller.Declined(controller.ErrPolicyDeclined) {
		t.Fatal("a declined edit counts as a fault")
	}
	if controller.Declined(errors.New("the document is damaged")) {
		t.Fatal("a fault counts as declined")
	}
	app, err := NewAppState()
	if err != nil {
		t.Fatal(err)
	}
	tester := ui.NewTester(app.View, 1260, 850)
	tester.Frame()
	document := app.Session.Document()
	if err := document.MoveVertical(0, false); err == nil {
		t.Fatal("a move without a direction was made")
	}
	tester.Frame()
	if !tester.HasText("Couldn't make that change") || document.LastError() != nil {
		t.Fatalf("the failed edit was not told once: %v", tester.Texts())
	}
	tester.Advance(10 * time.Second)
	reversed := controller.UTF16Range{Start: 5, End: 1}
	if _, err := document.Replace(&reversed, "x", kernel.EventOriginTyping); err == nil {
		t.Fatal("a reversed range was taken")
	}
	tester.Frame()
	if tester.HasText("Couldn't make that change") {
		t.Fatal("a declined edit was told")
	}
}
