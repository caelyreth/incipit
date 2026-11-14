package window

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/vocab"
)

// typeIntoActivePage types text into the page in front and lets the frame
// that follows sum it up and save it.
func typeIntoActivePage(t *testing.T, app *AppState, text string) {
	t.Helper()
	el := app.pageElement()
	if el == nil {
		t.Fatal("the page did not open")
	}
	if !el.Editor.Do(vocab.Type{Text: text}, time.Now()) {
		t.Fatalf("could not type %q", text)
	}
	app.summarize()
	app.save()
}

func titles(app *AppState, query string) []string {
	settleSearch(app)
	var out []string
	for _, item := range app.Search(query) {
		out = append(out, item.Title)
	}
	return out
}

func TestWhatIsTypedIsFoundAsItIsTyped(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	typeIntoActivePage(t, app, "The quarterly budget")
	if got := titles(app, "budget"); len(got) != 1 {
		t.Fatalf("budget found %v", got)
	}
	if got := titles(app, "quart"); len(got) != 1 {
		t.Fatalf("the word being typed found %v", got)
	}
	typeIntoActivePage(t, app, " and the review")
	if got := titles(app, "review budget"); len(got) != 1 {
		t.Fatalf("both words found %v", got)
	}
	if got := titles(app, "missing"); len(got) != 0 {
		t.Fatalf("missing found %v", got)
	}
}

func TestTheTrashKeepsItsSheetsOutOfSearch(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	typeIntoActivePage(t, app, "budget")
	app.TrashSheet(sheetName(activeSession(t, app).page))
	if got := titles(app, "budget"); len(got) != 0 {
		t.Fatalf("a trashed sheet was found: %v", got)
	}
	if got := titles(app, "$trash budget"); len(got) != 1 {
		t.Fatalf("$trash found %v", got)
	}
}

func TestTheIndexOutlivesAQuitAndIsMadeAgainWhenLost(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	typeIntoActivePage(t, app, "a 关系数据库 for the budget")
	settleSearch(app)
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	app = openNotebookApp(t, root)
	if got := titles(app, "数据库 budget"); len(got) != 1 {
		t.Fatalf("after a quit: %v", got)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}

	// An index lost or unreadable is made again from the store's pages,
	// none of them open.
	if err := os.RemoveAll(filepath.Join(root, indexFolder)); err != nil {
		t.Fatal(err)
	}
	app = openNotebookApp(t, root)
	for _, s := range app.sessions {
		if s.element != nil {
			t.Fatal("a page is open: the store's text would go unread")
		}
	}
	if got := titles(app, "数据库 budget"); len(got) != 1 {
		t.Fatalf("rebuilt from the store: %v", got)
	}
}
