package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/egoist/mygo/ui"
)

type nativeClock struct{}

func (nativeClock) Now() time.Time { return time.Unix(1_700_000_000, 0) }

func TestNativeTextClientAdaptsRangesAndSelection(t *testing.T) {
	document, err := controller.NewTextDocument("a🦀b", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	client := &NativeTextClient{Controller: document}
	text, actual := client.TextForRange(ui.TextInputRange{Start: 2, End: 2})
	if text != "🦀" || actual != (ui.TextInputRange{Start: 1, End: 3}) {
		t.Fatalf("text=%q actual=%#v", text, actual)
	}
	if err := document.SelectProjected(3, 1); err != nil {
		t.Fatal(err)
	}
	selection := client.Selection()
	if selection.Range != (ui.TextInputRange{Start: 1, End: 3}) || !selection.Reversed {
		t.Fatalf("selection=%#v", selection)
	}
}

func TestNativeTextClientTracksMarkedRange(t *testing.T) {
	document, err := controller.NewTextDocument("ab", nativeClock{})
	if err != nil {
		t.Fatal(err)
	}
	client := &NativeTextClient{Controller: document}
	document.SetMarkedText(
		&controller.UTF16Range{Start: 1, End: 1},
		"中",
		controller.UTF16Range{Start: 1, End: 1},
	)
	marked, ok := client.MarkedRange()
	if !ok || marked != (ui.TextInputRange{Start: 1, End: 2}) {
		t.Fatalf("marked=%#v ok=%v", marked, ok)
	}
}
