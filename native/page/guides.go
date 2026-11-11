package page

import (
	"slices"
	"time"

	"github.com/caelyreth/neverland/app/keymap"
	"github.com/caelyreth/neverland/editor/vocab"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/motion"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

const (
	// guideGlide follows a moving destination at the caret's pace, so a
	// guide past a closing pair keeps up with typing inside it.
	guideGlide = float32(38)
	// guideFade is gentler: a guide should arrive as a suggestion, not a
	// flash, and leave without a blink.
	guideFade = float32(18)
)

// guides are the page's key hints as last painted, by command. Each
// paint's guides retarget them: a guide that moves glides to its new
// place, one that appears fades in where it lands, and one no longer
// offered fades out where it was. Places are in page coordinates, so
// scrolling moves none.
type guides map[vocab.CommandID]*guideTrack

type guideTrack struct {
	x, y, h, opacity motion.Spring
	keys             []string
	seen             bool
}

func (t *guideTrack) place(r ui.Rect, opacity float32, now time.Time) {
	t.x, t.y, t.h = motion.NewSpring(r.X, now), motion.NewSpring(r.Y, now), motion.NewSpring(r.H, now)
	t.opacity = motion.NewUnitSpring(opacity, now)
}

func (t *guideTrack) advance(r ui.Rect, opacity float32, now time.Time, animate bool) {
	if !animate {
		t.place(r, opacity, now)
		return
	}
	t.x.Step(r.X, now, guideGlide)
	t.y.Step(r.Y, now, guideGlide)
	t.h.Step(r.H, now, guideGlide)
	t.opacity.Step(opacity, now, guideFade)
}

func (t *guideTrack) moving() bool {
	return t.x.Moving() || t.y.Moving() || t.h.Moving() || t.opacity.Moving()
}

// stepGuides retargets the tracks from the engine's guides, each shown
// with its command's key, and reports whether any still moves. A command
// with no key shows no guide.
func (el *Element) stepGuides(pl *pageLayout, show bool, now time.Time, animate bool) bool {
	if el.guides == nil {
		el.guides = guides{}
	}
	for _, t := range el.guides {
		t.seen = false
	}
	keys := el.Keys
	if keys == nil {
		keys = keymap.Default(keymap.Declared(el.Editor.Engine.Program().Catalog()))
	}
	if show {
		for _, g := range el.Editor.deco.Guides {
			chord, ok := keys.ChordFor(keymap.CommandID(g.Command))
			if !ok {
				continue
			}
			r, ok := pl.caretBox(g.At, el.Editor.metrics)
			if !ok {
				continue
			}
			t, ok := el.guides[g.Command]
			if !ok || t.opacity.Target() < 1 && t.opacity.Value() <= 0.01 {
				// Nothing shows: appear at the new place rather than
				// slide in from where the last guide faded out.
				t = &guideTrack{}
				t.place(r, 0, now)
				el.guides[g.Command] = t
			}
			t.keys = keymap.Keycaps(chord, keymap.PlatformCurrent)
			t.seen = true
			t.advance(r, 1, now, animate)
		}
	}
	moving := false
	for id, t := range el.guides {
		if !t.seen {
			t.advance(ui.Rect{X: t.x.Target(), Y: t.y.Target(), H: t.h.Target()}, 0, now, animate)
			if !t.opacity.Moving() && t.opacity.Value() <= 0.001 {
				delete(el.guides, id)
				continue
			}
		}
		moving = moving || t.moving()
	}
	return animate && moving
}

// paintGuides draws each guide as a hairline where its command would put
// the caret, and its keys on the compact caps the Keymap draws, riding
// just above the line, clear of the glyphs that follow.
func (el *Element) paintGuides(p *ui.Painter, view ui.Rect, ox, oy float32) {
	if len(el.guides) == 0 {
		return
	}
	scale := el.UIFontSize
	if scale <= 0 {
		scale = theme.DefaultUIFontSize
	}
	style := components.KeycapStyle(el.Theme, scale, true)
	ink := el.Theme.Text.Muted
	style.Ink = ink
	ids := make([]vocab.CommandID, 0, len(el.guides))
	for id := range el.guides {
		ids = append(ids, id)
	}
	slices.Sort(ids)
	for _, id := range ids {
		t := el.guides[id]
		alpha := t.opacity.Value()
		if alpha <= 0.001 {
			continue
		}
		x, y, h := ox+t.x.Value(), oy+t.y.Value(), t.h.Value()
		if y+h < view.Y || y > view.Y+view.H {
			continue
		}
		p.Fill(ui.Rect{X: x, Y: y, W: 1, H: h}, ink.Alpha(0.6*alpha), 0)
		_, width := style.Widths(t.keys)
		left := max(min(x+style.Height*0.3, view.X+view.W-width-2), view.X)
		top := max(y-style.Height+2, view.Y)
		style.Paint(p, t.keys, left, top, alpha)
	}
}
