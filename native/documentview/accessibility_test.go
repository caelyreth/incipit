package documentview

import (
	"reflect"
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

func TestDocumentAccessibilitySelectionUsesRuneOffsets(t *testing.T) {
	document, err := controller.NewTextDocument("A🦀ca", nativeClock{})
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
	eventTime := time.Unix(1_700_000_001, 0)
	if !element.handleInput(ui.InputEvent{
		Kind: ui.InputSelect, Start: 1, End: 3, Time: eventTime,
	}) {
		t.Fatal("the document rejected an accessibility selection")
	}
	selection := document.Selection()
	if selection.Range.Start != 1 || selection.Range.End != 4 {
		t.Fatalf("UTF-16 selection = %#v, want 1..4", selection.Range)
	}
}

// MyGo v0.7.1's rich Access hook cannot be implemented by code outside its
// module because the callback names an internal platform type. Keep this test
// until MyGo publishes a public accessibility description type; then replace
// the basic document node fields with that richer description.
func TestMyGoRichAccessibilityHookIsInternal(t *testing.T) {
	method, ok := reflect.TypeOf(ui.Element{}).MethodByName("Access")
	if !ok || method.Type.NumIn() != 2 || method.Type.In(1).Kind() != reflect.Func {
		t.Fatalf("unexpected MyGo Element.Access signature: %v", method.Type)
	}
	callback := method.Type.In(1)
	if callback.NumIn() != 1 || callback.In(0).Kind() != reflect.Pointer {
		t.Fatalf("unexpected MyGo accessibility callback: %v", callback)
	}
	if path := callback.In(0).Elem().PkgPath(); path != "github.com/egoist/mygo/internal/platform" {
		t.Fatalf("MyGo accessibility node package = %q; a public hook is now possible", path)
	}
}
