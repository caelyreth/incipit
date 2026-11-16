package components

import (
	"testing"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestTextFieldAutoFocusAndClearPreserveTextInput(t *testing.T) {
	value := "hello"
	tester := ui.NewTester(func(c *ui.Context) {
		TextField{
			Key:       "test-field",
			Label:     "Test field",
			Value:     &value,
			ClearIcon: IconClose.SVG(),
			AutoFocus: true,
		}.View(c, theme.DefaultSettings().Semantic())
	}, 320, 80)
	tester.Frame()

	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("text field did not receive initial text-input focus")
	}
	if err := tester.Click("Clear"); err != nil {
		t.Fatal(err)
	}
	tester.Frame()
	if value != "" {
		t.Fatalf("clear value = %q", value)
	}
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("clear removed text-input focus")
	}
	tester.Key(ui.Cmd, ui.KeyZ)
	if value != "hello" {
		t.Fatalf("undo did not restore the cleared value: %q", value)
	}
	tester.Key(ui.Cmd|ui.Shift, ui.KeyZ)
	if value != "" {
		t.Fatalf("redo did not clear the value: %q", value)
	}
}
