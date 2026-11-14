package window

import (
	"os"
	"path/filepath"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/internal/query"
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

func titles(app *AppState, text string) []string {
	settleSearch(app)
	var out []string
	for _, item := range app.Search(text).Sheets {
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

// foundLine is the line the query matches in the first sheet found, once
// it is read.
func foundLine(t *testing.T, app *AppState, text string) (string, []query.Span) {
	t.Helper()
	settleSearch(app)
	for deadline := time.Now().Add(5 * time.Second); time.Now().Before(deadline); time.Sleep(5 * time.Millisecond) {
		found := app.Search(text)
		if len(found.Sheets) == 0 {
			t.Fatalf("%q found nothing", text)
		}
		if found.Sheets[0].Line != "" {
			return found.Sheets[0].Line, found.Sheets[0].LineMatches
		}
	}
	t.Fatalf("the line of %q was never read", text)
	return "", nil
}

func TestASheetFoundShowsTheLineItMatches(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	typeIntoActivePage(t, app, "Trip plans")
	el := app.pageElement()
	el.Editor.Do(vocab.Break{}, time.Now())
	typeIntoActivePage(t, app, "Buy the train tickets and pack the budget sheet")
	line, spans := foundLine(t, app, "budget")
	if line != "Buy the train tickets and pack the budget sheet" || len(spans) != 1 || line[spans[0].From:spans[0].To] != "budget" {
		t.Fatalf("line %q %v", line, spans)
	}
	if found := app.Search("budget"); len(found.Sheets[0].TitleMatches) != 0 {
		t.Fatalf("the title holds no match: %v", found.Sheets[0].TitleMatches)
	}
}

func TestAFoundSheetOpensWithItsMatchSelected(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	typeIntoActivePage(t, app, "alpha beta budget gamma")
	name := sheetName(activeSession(t, app).page)
	settleSearch(app)
	app.OpenFound(name, "budget", false)
	sel := app.pageElement().Editor.Selection()
	if sel.Kind != doc.SelectText || sel.Anchor.Offset != 11 || sel.Head.Offset != 17 || sel.Anchor.Node != sel.Head.Node {
		t.Fatalf("selection %+v", sel)
	}
}

func TestCompletionTakesTheNotebooksTagsAndGroups(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	typeIntoActivePage(t, app, "notes #travel")
	if got := app.SearchNames("tag"); len(got) != 1 || got[0] != "travel" {
		t.Fatalf("tags %v", got)
	}
	if got := app.SearchNames("in"); len(got) == 0 || got[0] != "Inbox" {
		t.Fatalf("groups %v", got)
	}
}
