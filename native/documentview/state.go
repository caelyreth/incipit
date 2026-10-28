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
	Composing         bool
	ContentTop        float32
}
