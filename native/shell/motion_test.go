package shell

import (
	"testing"

	"github.com/egoist/mygo/ui"
)

// TestTransitionsChooseWhatTheyAnimate holds each transition the shell
// builds to naming what it animates. MyGo animates an element's place,
// size and colors when none is named, so one that only fades would trail
// behind its parent's every move, as the title bar's lines once did as a
// pane opened.
func TestTransitionsChooseWhatTheyAnimate(t *testing.T) {
	state := New()
	for name, transition := range map[string]ui.ElementTransition{
		"list":     listTransition(true, false),
		"list row": state.sheets().listRowTransition(true),
		"fade":     fadeTransition(true),
		"swap":     swapTransition(true, 8),
		"pop":      popTransition(true),
		"unfold":   unfoldTransition(true),
		"row":      rowTransition(true),
	} {
		if !transition.Position && !transition.Size && !transition.Colors {
			t.Errorf("the %s transition names nothing, so it animates where its element is", name)
		}
	}
}
