package components

import (
	"testing"

	"github.com/caelyreth/neverland/native/theme"
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
