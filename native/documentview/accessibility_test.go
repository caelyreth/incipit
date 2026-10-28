package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestDocumentElementIsLabeledFocusableAndReceivesTextInput(t *testing.T) {
	document, err := controller.NewTextDocument("hello", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	input := NewBridge(document, nil, func() time.Time {
		return time.Unix(1_700_000_000, 0)
	})
	settings := theme.DefaultSettings()
	element := NewElement(
		input,
		settings.Semantic(),
		theme.NewProseStyle(settings.ProseFontSize),
		time.Unix(1_700_000_000, 0),
	)
	element.Label = "Writing document"
	tester := ui.NewTester(func(c *ui.Context) {
		element.View(c)
	}, 600, 400)
	tester.Frame()
	if !tester.Focused("Writing document") {
		t.Fatalf("document did not take initial focus: %#v", tester.Texts())
	}
	if active, _ := tester.TextInputActive(); !active {
		t.Fatal("document did not register a native text input client")
	}
	tester.Type("!")
	if document.Text() != "!hello" {
		t.Fatalf("typed text = %q", document.Text())
	}
}
