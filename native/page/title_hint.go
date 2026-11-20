package page

import (
	"time"

	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/egoist/mygo/ui"
)

// titleHint is what an empty page shows on its first line, which names
// the sheet: written there, the hint's place is the title's. It shows
// whenever the page holds nothing, wherever the caret is, and goes with
// the first letter.
const titleHint = "Title"

// paintTitleHint draws an empty page's hint, in its first line's own
// style, muted, where its caret stands.
func (el *Element) paintTitleHint(p *ui.Painter, pl *pageLayout, ox, oy float32) {
	u, s, ok := titleHinted(pl)
	if !ok {
		return
	}
	sp := span(titleHint, el.Editor.theme.Resolve([]vocab.Group{u.pres.Group}), el.Editor.metrics.Body)
	sp.Color = el.Theme.Text.Muted.Alpha(0.65 * u.fade)
	c := s.caretRect(0)
	h := sp.Size * 1.2
	p.RichText(ox+u.x+s.x+c.X, oy+u.y+s.y+c.Y+(c.H-h)/2, 0, sp)
}

// titleHinted is the line an empty page's hint stands on: its one unit,
// text with nothing written, not even a composition.
func titleHinted(pl *pageLayout) (*unitLayout, *surfaceLayout, bool) {
	if pl == nil || len(pl.units) != 1 {
		return nil, nil, false
	}
	u := pl.units[0]
	if u.kind != nil || len(u.surfaces) != 1 {
		return nil, nil, false
	}
	s := u.surfaces[0]
	if s.text != "" || s.hasMarked || !s.shown {
		return nil, nil, false
	}
	return u, s, true
}

// SelectTitle selects the page's first line, which names the sheet, for
// writing over, and asks for the keys.
func (el *Element) SelectTitle() {
	at := time.Now()
	el.Editor.Do(vocab.Move{Unit: vocab.ToPageEdge, Dir: vocab.Backward}, at)
	el.Editor.Do(vocab.Move{Unit: vocab.ToSlotEdge, Dir: vocab.Forward, Extend: true}, at)
	el.Focus()
}
