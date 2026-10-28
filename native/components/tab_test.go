package components

import (
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

var testDocumentIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M6 3h8l4 4v14H6z"/></svg>`))
var testCloseIcon = ui.MustParseSVG([]byte(`<svg xmlns="http://www.w3.org/2000/svg" viewBox="0 0 24 24"><path d="M6 6l12 12M18 6 6 18"/></svg>`))

func TestTabSelectAndCloseStayIndependent(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	selected, closed := 0, 0
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		Tab{
			Key: "tab", Title: "Chapter one", Selected: true,
			Icon: testDocumentIcon, CloseIcon: testCloseIcon,
			OnSelect: func() { selected++ },
			OnClose:  func() { closed++ },
		}.View(c, semantic)
	}, 360, 80)
	tester.Frame()
	tab, ok := tester.Find("Chapter one")
	if !ok || tab.H != theme.DefaultLayout.Tab {
		t.Fatalf("tab rect = %#v, %v", tab, ok)
	}
	if err := tester.Click("Chapter one"); err != nil {
		t.Fatal(err)
	}
	if selected != 1 || closed != 0 {
		t.Fatalf("select = %d, close = %d", selected, closed)
	}
	if err := tester.Click("Close Chapter one"); err != nil {
		t.Fatal(err)
	}
	if closed != 1 || selected != 1 {
		t.Fatalf("after close select = %d, close = %d", selected, closed)
	}
}
