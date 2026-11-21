package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A sheet opened from the field flies out of what was pressed for it, its
// row or card or the + that made it, into its column on the desk, and its
// last sheet closed, back into its row (ux-audit F1). The desk itself
// only ever comes up out of its lane and goes back into it; what flies
// is one plain sheet, its ground and its title, never the page: it costs
// a frame a rectangle and a line of text, the same whichever way and
// whatever the desk holds. How far it has flown is how far the window is
// on its way to the desk (viewAt), so it lands as the desk does.

// flightState is a sheet's way between the field and the desk.
type flightState struct {
	// from is where it flies from or back to as the field rests, sheet
	// the sheet it carries ("" for the one coming in front), title its
	// title flying back, and back whether it is flying back.
	from        stageRect
	sheet       string
	title       string
	back, armed bool
	// origins are where the things a sheet comes from stood as the field
	// last rested, by origin key; slots where each column stood on the
	// desk the frame before.
	origins map[string]stageRect
	slots   map[string]stageRect
	// last is the sheet in front while the window looked at the desk,
	// and wasOn whether it did the frame before.
	last  Page
	wasOn bool
}

// sheetOrigin, createOrigin and actionOrigin name what a sheet comes
// from: its row or card, a group's area that makes one, an action.
func sheetOrigin(id string) string  { return "sheet:" + id }
func createOrigin(id string) string { return "create:" + id }
func actionOrigin(id string) string { return "action:" + id }

// noteOrigin notes where el, a thing a sheet comes from, stands in the
// window as the field rests.
func (s *State) noteOrigin(key string, el ui.Element) {
	if s.view > 0.001 {
		return
	}
	b := el.Bounds()
	if b.W <= 0 || b.H <= 0 {
		return
	}
	if s.flight.origins == nil {
		s.flight.origins = map[string]stageRect{}
	}
	s.flight.origins[key] = stageRect{X: b.X, Y: b.Y, W: b.W, H: b.H}
}

// forgetOrigins lets go of where things stood, as the field at rest is
// built anew: what is not built again is not in view.
func (s *State) forgetOrigins() {
	if s.view <= 0.001 {
		clear(s.flight.origins)
	}
}

// riseFrom has the sheet the desk next comes up with fly out of what key
// names, where it was last seen at rest.
func (s *State) riseFrom(key string) {
	from, ok := s.flight.origins[key]
	s.flight.from, s.flight.sheet, s.flight.back, s.flight.armed = from, "", false, ok
}

// noteSlot notes where a column, col, stands in the window.
func (s *State) noteSlot(id string, col ui.Element) {
	if b := col.Bounds(); b.W > 0 && b.H > 0 {
		if s.flight.slots == nil {
			s.flight.slots = map[string]stageRect{}
		}
		s.flight.slots[id] = stageRect{X: b.X, Y: b.Y, W: b.W, H: b.H}
	}
}

// followFlight keeps the flight as the window moves between the field and
// the desk, on it or not, at as far on its way: its last sheet closed,
// that sheet flies back into its row from where its column stood; and
// arrived either way, nothing flies.
func (s *State) followFlight(on bool, at float32) {
	f := &s.flight
	if on && s.ActivePage != "" {
		f.last = s.openPage(s.ActivePage)
	}
	if f.wasOn && !on {
		f.armed = false
		if from, ok := f.origins[sheetOrigin(f.last.ID)]; ok && len(s.Open) == 0 {
			f.from, f.sheet, f.title, f.back, f.armed = from, f.last.ID, f.last.Title, true, true
		}
	}
	f.wasOn = on
	if on && at >= 0.999 || !on && at <= 0.001 {
		f.armed = false
		clear(f.slots)
	}
}

// flying is the flight this frame in a window w by h, the field moved by
// field, at on the way: where the sheet stands, how far it shows and its
// title, if it flies.
func (s *State) flying(field stageLook, at, w, h float32) (stageRect, float32, string, bool) {
	f := s.flight
	if !f.armed {
		return stageRect{}, 0, "", false
	}
	sheet, title := f.sheet, f.title
	if sheet == "" {
		sheet = s.ActivePage
		title = s.openPage(sheet).Title
	}
	to, ok := f.slots[sheet]
	if !ok {
		return stageRect{}, 0, "", false
	}
	from := f.from
	from.X += field.Offset.X
	if from.X >= w || from.Y >= h || from.X+from.W <= 0 || from.Y+from.H <= 0 {
		return stageRect{}, 0, "", false
	}
	// It comes out of what was pressed and goes back into it fading, and
	// landing on the desk gives way to its column; going back it takes
	// its column's place at once.
	shown := min(1, 8*at)
	if !f.back {
		shown *= min(1, 6*(1-at))
	}
	return stageLerpRect(from, to, at), shown, title, shown > 0.001
}

// flightView draws the sheet flying, over the field and the desk.
func (s *State) flightView(c *ui.Context, semantic theme.Theme, field stageLook, at, w, h float32) {
	r, shown, title, ok := s.flying(field, at, w, h)
	if !ok {
		return
	}
	k := s.Settings.UIFontSize / theme.DefaultUIFontSize
	ui.Column(c).Key("flight").Absolute().Left(r.X).Top(r.Y).Size(r.W, r.H).IgnoreInput(true).AccessHidden(true).
		Background(semantic.Surface.Page).Border(1, semantic.Border.Variant).Radius(10 * k).Clip().
		Opacity(shown).PaddingX(12 * k).PaddingY(8 * k).Children(func() {
		components.CenteredLabel(c, title, theme.TextSmall, semantic.Text.Muted)
	})
}
