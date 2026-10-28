package documentview

import (
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestElementPaintsControllerSurfaceAndRegistersTextInput(t *testing.T) {
	document, err := controller.NewTextDocument("hello\nworld", nativeClock{})
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
	tester := ui.NewTester(func(c *ui.Context) {
		element.View(c)
	}, 600, 400)
	tester.Frame()
	if tester.Image() == nil {
		t.Fatal("document element produced no image")
	}
	if element.current == nil || element.current.Flow().Text() != "hello\nworld" {
		t.Fatalf("layout = %#v", element.current)
	}
}
