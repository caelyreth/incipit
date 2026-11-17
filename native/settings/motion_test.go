package settings

import (
	"testing"
	"time"

	"github.com/egoist/mygo/ui"
)

func TestSettingsTagsFollowEffectiveMotionPolicy(t *testing.T) {
	for _, test := range []struct {
		name    string
		active  bool
		hidden  bool
		reduced bool
		moving  bool
	}{
		{"active", true, false, false, true},
		{"inactive visible", false, false, false, true},
		{"hidden", true, true, false, false},
		{"reduced motion", true, false, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := New()
			show := false
			tester := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Children(func() {
					if show {
						state.tag(c, state.Theme.Semantic(), "Changed")
					}
				})
			}, 200, 100)
			if !test.active {
				tester.SetWindowActive(false)
			}
			if test.hidden {
				tester.SetWindowHidden(true)
			}
			if test.reduced {
				tester.SetPreferences(ui.Preferences{ReduceMotion: true})
			}
			tester.Frame()
			tester.Advance(time.Second)
			show = true
			tester.Frame()
			if got := tester.FramePending(); got != test.moving {
				t.Fatalf("tag animation pending = %v, want %v", got, test.moving)
			}
		})
	}
}
