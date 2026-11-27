package page

import (
	"math"

	"github.com/caelyreth/incipit/internal/editor/engine"
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/egoist/mygo/ui"
)

// printView is the view a printed page reads its engine through.
const printView uint64 = 1

// Print lays e's page out as it prints, width DIPs wide in the colors and
// prose given, and returns what it draws, as vectors (ui.Record): still
// and unfocused, so no caret, hint or guide shows, nothing moving, every
// picture read as it is drawn.
func Print(e *engine.Engine, colors theme.Theme, prose theme.ProseStyle, pictures *Pictures, width float32) *ui.Drawing {
	ed := NewEditor(e, printView)
	defer ed.Close()
	el := NewElement(ed)
	el.focusPending = false
	el.Theme, el.Prose = colors, prose
	if pictures != nil {
		pictures.Sync = true
		el.Pictures = pictures
	}
	// Its height is known once it is laid out, which takes a text system:
	// a first recording only lays it out.
	var height float32
	ui.Record(func(c *ui.Context) {
		ui.Box(c).Width(width).Height(1).Draw(func(*ui.Painter, ui.Rect) {
			_, height = el.Still(width)
		})
	}, int(math.Ceil(float64(width))), 1)
	h := int(math.Ceil(float64(height))) + 1
	return ui.Record(func(c *ui.Context) {
		ui.Box(c).Width(width).Height(float32(h)).Draw(func(p *ui.Painter, r ui.Rect) {
			el.Still(width)
			el.paint(p, r, false)
		})
	}, int(math.Ceil(float64(width))), h)
}
