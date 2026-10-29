package documentview

import (
	"strings"
	"testing"
	"time"

	"github.com/caelyreth/neverland/app/controller"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

func TestOffscreenCaretDoesNotScheduleBreathing(t *testing.T) {
	document, err := controller.NewTextDocument(strings.Repeat("line\n", 100), nativeClock{})
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
	element.Animate = true

	tester := ui.NewTester(func(c *ui.Context) {
		element.View(c)
	}, 600, 200)
	tester.Frame()
	element.SetScrollY(2000)
	tester.Frame()
	tester.Advance(300 * time.Millisecond)
	if tester.FramePending() {
		t.Fatal("offscreen caret kept requesting breathing frames")
	}
}
