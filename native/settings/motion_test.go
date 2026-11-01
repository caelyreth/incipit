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
		reduced bool
		moving  bool
	}{
		{"active", true, false, true},
		{"inactive", false, false, false},
		{"reduced motion", true, true, false},
	} {
		t.Run(test.name, func(t *testing.T) {
			state := New()
			state.WindowActive = test.active
			show := false
			tester := ui.NewTester(func(c *ui.Context) {
				ui.Column(c).Children(func() {
					if show {
						state.tag(c, state.Theme.Semantic(), "Changed")
					}
				})
			}, 200, 100)
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
