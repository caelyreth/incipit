package components

import (
	"time"

	"github.com/egoist/mygo/ui"
)

// Fold opens and closes content in place. Its height follows the content's
// measured height on a spring, open or not, so what sits below slides rather
// than jumps: as the fold opens or closes, and as its content changes size
// while open. The content fades with it. Closing keeps drawing content until
// the fold is shut, so content must still be drawable then, from what it
// last showed.
type Fold struct {
	Key     string
	Open    bool
	Animate bool
	// Appear opens a fold that is first built open from shut, as a panel
	// that appears on demand does; without it a fold first built open
	// starts open, as the page it belongs to arrives.
	Appear bool
	// Settle is how long it takes to open or close; MotionPanel when zero.
	Settle time.Duration
}

type foldState struct {
	natural  float32
	measured bool
}

// View draws the fold and reports how open it is, from 0 to 1.
func (f Fold) View(c *ui.Context, content func()) float32 {
	fold := ui.Column(c).Key(f.Key).WidthPercent(100).Clip()
	st := *ui.Local(fold, "fold", func() *foldState { return &foldState{} })
	open := float32(0)
	if f.Open {
		open = 1
	}
	height, alpha := st.natural*open, open
	switch {
	case !st.measured && f.Open && !f.Appear:
		// Shown as it is while it is measured for the first time.
		alpha = 1
	case !st.measured:
		height, alpha = 0, 0
		if f.Animate {
			fold.Jump("height", 0)
			fold.Jump("alpha", 0)
		}
	case f.Animate:
		settle := f.Settle
		if settle == 0 {
			settle = MotionPanel
		}
		height = Follow(fold, "height", height, settle)
		alpha = Follow(fold, "alpha", alpha, settle)
	}
	if !f.Open && alpha <= 0.001 && st.measured {
		fold.Height(0)
		return 0
	}
	if st.measured || !f.Open || f.Appear {
		if height < st.natural-0.25 || height > st.natural+0.25 || !st.measured {
			fold.Height(max(0, height))
		}
	}
	fold.Opacity(alpha)
	fold.Children(func() {
		// The body keeps its own height inside a shorter fold, so it can
		// be measured on the way.
		body := ui.Column(c).WidthPercent(100).Shrink(0)
		if bounds := body.LayoutBounds(); bounds.H > 0 {
			if !st.measured && f.Animate {
				// From here the springs take over, from what shows: all of
				// it for a fold that starts open, nothing for one that
				// appears or starts shut.
				shown := float32(0)
				if f.Open && !f.Appear {
					shown = 1
				}
				fold.Jump("height", bounds.H*shown)
				fold.Jump("alpha", shown)
			}
			st.natural, st.measured = bounds.H, true
		}
		body.Children(content)
	})
	return alpha
}
