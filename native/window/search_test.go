package window

import (
	"os"
	"path/filepath"
	"slices"
	"testing"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/shell"
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

func TestASmartFolderListsWhatItsQueryFinds(t *testing.T) {
	root := t.TempDir()
	app := openNotebookApp(t, root)
	typeIntoActivePage(t, app, "The tide turns #sea")
	first := sheetName(activeSession(t, app).page)
	settleSearch(app)
	made, err := app.library.AddSmartGroupAt(0, "#sea", "#sea", len(app.library.Groups(0)))
	if err != nil {
		t.Fatal(err)
	}
	app.libraryChanged()
	id := groupName(made)
	group, ok := app.Group(id)
	if !ok || !group.Smart() || group.Name != "#sea" || group.Count != 1 {
		t.Fatalf("the folder is %+v", group)
	}
	place := shell.Place{Kind: shell.PlaceGroup, Group: id}
	if got := app.Sheets(place); len(got) != 1 || got[0].ID != first {
		t.Fatalf("the folder lists %+v", got)
	}
	// It follows what is written: a new sheet the query finds is in it.
	app.NewSheet("", 0)
	typeIntoActivePage(t, app, "Low #sea")
	settleSearch(app)
	if got := len(app.Sheets(place)); got != 2 {
		t.Fatalf("after a second sheet the folder counts %d", got)
	}
	// Nothing is filed in it, nor offered to be.
	app.MoveSheet(first, id, 0)
	if app.SheetGroup(first) == id {
		t.Fatal("a sheet was filed in the smart folder")
	}
	for _, target := range app.Targets() {
		if target.ID == id {
			t.Fatal("the smart folder is a place to move to")
		}
	}
	if names := app.SearchNames("in"); slices.Contains(names, "#sea") {
		t.Fatalf("$in offers the smart folder: %v", names)
	}
	if err := app.library.SetGroupQuery(made, "turns"); err != nil {
		t.Fatal(err)
	}
	app.libraryChanged()
	if got := app.Sheets(place); len(got) != 1 || got[0].ID != first {
		t.Fatalf("after its query changed the folder lists %+v", got)
	}
	if err := app.Close(); err != nil {
		t.Fatal(err)
	}
	app = openNotebookApp(t, root)
	if group, ok := app.Group(id); !ok || group.Query != "turns" {
		t.Fatalf("after a quit the folder is %+v", group)
	}
}

// The pill's keys offer the notebook's own values: the sites its sheets
// link to, and the words their titles are written in, the commonest words
// of English left out; and each value says how many sheets it would find.
func TestTheKeysOfferTheNotebooksValuesAndCountThem(t *testing.T) {
	app := openNotebookApp(t, t.TempDir())
	typeIntoActivePage(t, app, "See https://docs.github.com/x for more.")
	nameActivePage(t, app, "The quarterly budget review")
	settleSearch(app)
	if got := app.SearchNames("title"); !slices.Contains(got, "quarterly") && !slices.Contains(got, "Quarterly") ||
		slices.ContainsFunc(got, func(w string) bool { return query.Fold(w) == "the" }) {
		t.Fatalf("title words = %v", got)
	}
	if got := app.SearchNames("link"); !slices.Contains(got, "docs.github.com") || !slices.Contains(got, "github.com") {
		t.Fatalf("linked sites = %v", got)
	}
	counts := app.CountValues("", "link", []string{"github.com", "example.com"})
	if counts["github.com"] != 1 || counts["example.com"] != 0 {
		t.Fatalf("counts = %v", counts)
	}
	if again := app.CountValues("", "link", []string{"github.com", "example.com"}); again["github.com"] != 1 {
		t.Fatalf("counts kept = %v", again)
	}
}
