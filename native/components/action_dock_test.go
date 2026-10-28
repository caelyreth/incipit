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
