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

func TestTabDrawsKeyboardFocusAtItself(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	for _, selected := range []bool{false, true} {
		t.Run(map[bool]string{false: "unselected", true: "selected"}[selected], func(t *testing.T) {
			tester := ui.NewTester(func(c *ui.Context) {
				c.SetTheme(theme.DefaultSettings().MyGoTheme())
				Tab{
					Key: "tab", Title: "Chapter one", Selected: selected,
					Icon: testDocumentIcon, CloseIcon: testCloseIcon,
				}.View(c, semantic)
			}, 360, 80)
			tester.Frame()
			tab, ok := tester.Find("Chapter one")
			if !ok {
				t.Fatal("tab is missing")
			}
			before := tester.Image()
			tester.Key(0, ui.KeyTab)
			if !tester.Focused("Chapter one") {
				t.Fatal("Tab did not focus the tab")
			}
			after := tester.Image()
			changed := 0
			for y := int(tab.Y); y < int(tab.Y+tab.H); y++ {
				for x := int(tab.X); x < int(tab.X+tab.W); x++ {
					if before.RGBAAt(x, y) != after.RGBAAt(x, y) {
						changed++
					}
				}
			}
			if changed == 0 {
				t.Fatal("the focused tab changed no pixels")
			}
		})
	}
}
