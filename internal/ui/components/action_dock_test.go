package components

import (
	"testing"

	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

func TestActionDockKeepsDocumentTextInputFocus(t *testing.T) {
	text := ""
	clicked := false
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		ui.Column(c).Fill().Padding(12).Children(func() {
			ui.TextInputBase(c.Key("document"), &text).AutoFocus().Label("Document")
			ActionDock{
				Key: "actions", Label: "Writing actions",
				Groups: []ActionGroup{{
					Label: "Formatting",
					Actions: []func(){
						func() {
							IconButton{
								Key: "bold", Label: "Bold", Icon: IconBold.SVG(),
								Style: SubtleButton, Size: MediumButton,
								Action: func() { clicked = true },
							}.View(c, theme.DefaultSettings().Semantic())
						},
					},
				}},
			}.View(c, theme.DefaultSettings().Semantic())
		})
	}, 500, 160)
	tester.Frame()
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("document input was not active")
	}
	if err := tester.Click("Bold"); err != nil {
		t.Fatal(err)
	}
	if !clicked {
		t.Fatal("dock action did not run")
	}
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("dock action removed document text-input focus")
	}
}

func TestActionDockIsOneKeyboardFocusGroup(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	view := func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		ui.Column(c).Fill().Padding(12).Children(func() {
			Button{Key: "before", Label: "Before"}.View(c, semantic)
			ActionDock{
				Key: "actions", Label: "Writing actions",
				Groups: []ActionGroup{{
					Label: "Formatting",
					Actions: []func(){
						func() {
							IconButton{Key: "bold", Label: "Bold", Icon: IconBold.SVG()}.
								View(c, semantic)
						},
						func() {
							IconButton{Key: "italic", Label: "Italic", Icon: IconItalic.SVG()}.
								View(c, semantic)
						},
					},
				}},
			}.View(c, semantic)
			Button{Key: "after", Label: "After"}.View(c, semantic)
		})
	}
	tester := ui.NewTester(view, 500, 220)
	tester.Frame()
	tester.Key(0, ui.KeyTab)
	if !tester.Focused("Before") {
		t.Fatal("Tab did not enter the controls before the dock")
	}
	tester.Key(0, ui.KeyTab)
	if !tester.Focused("Bold") {
		t.Fatal("the dock is not one Tab stop")
	}
	tester.Key(0, ui.KeyRight)
	if !tester.Focused("Italic") {
		t.Fatal("Right did not move within the dock")
	}
	tester.Key(0, ui.KeyTab)
	if !tester.Focused("After") {
		t.Fatal("Tab did not leave the dock")
	}
}

// A group coming into a dock grows the dock about it, its width going
// through those between, not at once: MyGo collapses the group alone.
func TestActionDockGrowsAsAGroupComes(t *testing.T) {
	semantic := theme.DefaultSettings().Semantic()
	more := false
	tester := ui.NewTester(func(c *ui.Context) {
		c.SetTheme(theme.DefaultSettings().MyGoTheme())
		button := func(key string) func() {
			return func() {
				IconButton{Key: key, Label: key, Icon: IconBold.SVG(), Style: SubtleButton, Size: MediumButton}.View(c, semantic)
			}
		}
		groups := []ActionGroup{{Key: "first", Label: "First", Actions: []func(){button("One"), button("Two")}}}
		if more {
			groups = append(groups, ActionGroup{Key: "more", Label: "More", Actions: []func(){button("Three")}})
		}
		ui.Row(c).Fill().Justify(ui.Center).AlignItems(ui.Center).Children(func() {
			ActionDock{Key: "actions", Label: "Writing actions", Groups: groups, Room: 400, Animate: true}.View(c, semantic)
		})
	}, 500, 160)
	tester.Frame()
	width := func() float32 {
		r, _ := tester.Find("Writing actions")
		return r.W
	}
	from := width()
	more = true
	tester.Frame()
	tester.Advance(MotionDock / 3)
	tester.Frame()
	between := width()
	tester.Advance(MotionDock * 3)
	tester.Frame()
	to := width()
	if !(from < between && between < to) {
		t.Fatalf("The dock should grow from %v to %v through the widths between, but stood at %v on the way.",
			from, to, between)
	}
}
