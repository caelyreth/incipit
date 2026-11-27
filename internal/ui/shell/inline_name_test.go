package shell

import (
	"testing"

	"github.com/caelyreth/incipit/internal/ui/components"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

func TestInlineNameSelectsUnicodeAndEndsOnSubmitEscapeOrBlur(t *testing.T) {
	for _, tt := range []struct {
		name   string
		text   string
		finish func(*ui.Tester) error
		want   nameEditAction
	}{
		{"submit", "新", func(tester *ui.Tester) error { tester.Key(0, ui.KeyEnter); return nil }, nameCommit},
		{"escape", "新", func(tester *ui.Tester) error { tester.Key(0, ui.KeyEscape); return nil }, nameCancel},
		{"blur", "新", func(tester *ui.Tester) error { return tester.Click("Elsewhere") }, nameCommit},
		{"empty", "", func(tester *ui.Tester) error { tester.Key(0, ui.KeyEnter); return nil }, nameCommit},
	} {
		t.Run(tt.name, func(t *testing.T) {
			name := "章节 🚀"
			action := nameContinue
			tester := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Children(func() {
					if action == nameContinue {
						input := ui.TextInputBase(c.Key("name"), &name).Label("Name")
						action = inlineName(input)
					}
					components.Button{Key: "elsewhere", Label: "Elsewhere"}.View(c, theme.DefaultSettings().Semantic())
				})
			}, 300, 100)
			tester.Frame()
			tester.Frame()
			if action != nameContinue {
				t.Fatalf("initial action = %v", action)
			}
			if tt.text == "" {
				tester.Key(0, ui.KeyBackspace)
			} else {
				tester.Type(tt.text)
			}
			tester.Frame()
			if name != tt.text {
				t.Fatalf("initial Unicode name was not fully selected: %q", name)
			}
			if err := tt.finish(tester); err != nil {
				t.Fatal(err)
			}
			tester.Frame()
			if action != tt.want {
				t.Fatalf("action = %v, want %v", action, tt.want)
			}
		})
	}
}
