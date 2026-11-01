// Package documentview paints Neverland's custom writing surface from
// UI-neutral engine values.
package documentview

import (
	"github.com/caelyreth/neverland/engine/surface"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/textlayout"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type State struct {
	Flow              *surface.Flow
	Frame             *surface.Frame
	Layout            *textlayout.Layout
	Motion            *motion.Motion
	Theme             theme.Theme
	Prose             theme.ProseStyle
	ScrollY           float32
	Focused           bool
	Animate           bool
	Placeholder       string
	PlaceholderLayout *ui.TextLayout
	PointerActive     bool
	PointerSelecting  bool
	Composing         bool
	// TopInset and BottomInset are space within the scrolled page above
	// its first line and below its last, so text scrolls through them to
	// the viewport's edges rather than to a clip short of them.
	TopInset    float32
	BottomInset float32
	// Margin is the room beside the column, where areas show, and
	// UIFontSize the size keys are named in.
	Margin     float32
	UIFontSize float32
	// Guides keeps the anchored key hints' motion across frames; without
	// it guides still paint, settled.
	Guides *Guides
	// Pictures gives the figures their pictures, and FigureMotion keeps
	// their fades; without either figures show their places, settled.
	Pictures     *Pictures
	FigureMotion *FigureMotion

	// DrawnCaret is set by Paint: the caret geometry it settled on this
	// frame, local to the content like the layout's.
	DrawnCaret ui.Rect
}
