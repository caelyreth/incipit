package settings

import (
	"strings"
	"unicode/utf8"

	"github.com/caelyreth/neverland/app/keymap"
	appsettings "github.com/caelyreth/neverland/app/settings"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Every key the settings show is live and records in place, wherever it is:
// a key named in a sentence ("Press ⇥ to leave a pair") and a key at the end
// of a shortcut row are the same control. Pressing one opens the recorder
// under the row or sentence that holds it, and a taken key is resolved there,
// so recording behaves the same on the Editor and Keymap pages.

// unassigned is how a sentence shows a key reference with no key.
const unassigned = "no key"

// keyText is how a command's key reads in text: its first binding.
func (s *State) keyText(command keymap.CommandID) string {
	shortcut, ok := s.Data.Shortcut(command)
	if !ok || len(shortcut.Bindings) == 0 {
		return unassigned
	}
	return keymap.Spell(shortcut.Bindings[0], keymap.PlatformCurrent)
}

// spell writes text's key references as their keys, for search and for
// assistive technology.
func (s *State) spell(text string) string {
	return appsettings.Spell(text, s.keyText)
}

// spellWords also writes each key the way the keymap search reads it.
func (s *State) spellWords(text string) string {
	return appsettings.Spell(text, func(command keymap.CommandID) string {
		shortcut, ok := s.Data.Shortcut(command)
		words := s.keyText(command)
		if ok && len(shortcut.Bindings) > 0 {
			words += " " + keymap.Words(shortcut.Bindings[0], keymap.PlatformCurrent)
		}
		return words
	})
}

// phrase draws settings text whose key references are live keys. host names
// the row or sentence that holds them, where their recorder opens.
//
// Each key is an inline object the sentence lays out like a word, drawn as
// keycaps, one per key, as the Keymap draws them. A sentence that holds keys
// takes a little more leading, so the caps can be pressed comfortably while
// its words stay the size of any other description.
func (s *State) phrase(
	c *ui.Context,
	semantic theme.Theme,
	host string,
	text string,
	size theme.TextSize,
	color ui.Color,
) ui.Element {
	if !strings.ContainsRune(text, '{') {
		return components.Label(c, text, size, color)
	}
	scale := s.Theme.UIFontSize
	geometry := chipGeometryFor(scale)
	keys := make(map[int]func(*ui.Painter, ui.Rect), 2)
	return ui.RichText(c).Label(s.spell(text)).
		Font(theme.UIFontFamily).FontSize(size.Size(scale)).
		FixedLineHeight(max(size.LineHeight(scale), geometry.line)).TextColor(color).
		Children(func() {
			at := 0
			for _, fragment := range appsettings.Phrase(text) {
				if fragment.Command == "" {
					ui.Text(c, fragment.Text)
					at += utf8.RuneCountInString(fragment.Text)
					continue
				}
				keys[at] = s.keyChip(c, semantic, host, fragment.Command, size.Size(scale), geometry)
				at++
			}
		}).
		DrawObjects(func(p *ui.Painter, at int, r ui.Rect) {
			if draw, ok := keys[at]; ok {
				draw(p, r)
			}
		})
}

// chipGeometry sizes keycaps in a sentence: the Keymap's caps, a margin of
// target around them, and the line that holds them.
type chipGeometry struct {
	cap, gap, margin, line, radius, glyph float32
}

func chipGeometryFor(scale float32) chipGeometry {
	unit := scale / theme.DefaultUIFontSize
	return chipGeometry{
		cap:    22 * unit,
		gap:    2 * unit,
		margin: 3 * unit,
		line:   30 * unit,
		radius: 5 * unit,
		glyph:  theme.TextXSmall.Size(scale),
	}
}

// keyCap is one cap of a key: its glyphs, their width, and its place and
// width within the key.
type keyCap struct {
	glyphs []ui.Glyph
	ink    float32
	x, w   float32
}

// layoutCaps sets labels as caps, left to right: a square cap for one glyph,
// a wider one for a word.
func layoutCaps(labels []string, font ui.Font, geometry chipGeometry) ([]keyCap, float32) {
	caps := make([]keyCap, 0, len(labels))
	x := float32(0)
	for index, label := range labels {
		glyphs := ui.Shape(label, font)
		ink := float32(0)
		if n := len(glyphs); n > 0 {
			ink = glyphs[n-1].X + glyphs[n-1].Advance
		}
		w := max(geometry.cap, ink+0.6*geometry.cap)
		if index > 0 {
			x += geometry.gap
		}
		caps = append(caps, keyCap{glyphs: glyphs, ink: ink, x: x, w: w})
		x += w
	}
	return caps, x
}

// keyChip is a key inside a sentence. Its target is the whole object, the
// caps and a margin around them. When the key changes, the caps reshape to
// the new key, each easing to its new width and place while the sentence
// makes room, and the old glyphs are gone before the new ones arrive, so
// two keys never show at once. The faces warm on hover and take the accent
// while recording.
func (s *State) keyChip(
	c *ui.Context,
	semantic theme.Theme,
	host string,
	command keymap.CommandID,
	fontSize float32,
	geometry chipGeometry,
) func(*ui.Painter, ui.Rect) {
	shortcut, known := s.Data.Shortcut(command)
	editable := known
	animate := s.motion(c)
	labels, unbound := []string{unassigned}, true
	if known && len(shortcut.Bindings) > 0 {
		labels, unbound = keymap.Keycaps(shortcut.Bindings[0], keymap.PlatformCurrent), false
	}
	key := strings.Join(labels, "\x00")

	chip := ui.RichText(c).Key("key-" + host + "-" + string(command)).
		Role(ui.RoleButton).Label(shortcutLabel(shortcut, command))
	if editable {
		chip.Focusable().FocusRing(false).Cursor(ui.CursorPointer)
	}
	st := *ui.Local(chip, "chip", func() *chipState { return &chipState{} })
	font := ui.Font{Family: theme.UIFontFamily, Size: geometry.glyph, Features: "tnum"}
	swap := float32(1)
	if animate {
		swap = components.Follow(chip, "swap", 1, components.MotionSettle)
	}
	if st.key != key {
		caps, width := layoutCaps(labels, font, geometry)
		if st.key != "" && animate {
			// Leave from what shows now, even mid-change, and ask the
			// spring for the change: a value set aside without asking
			// would wait for some other rebuild to move.
			st.leaving, st.leavingW = st.at(swap)
			chip.Jump("swap", 0)
			swap = components.Follow(chip, "swap", 1, components.MotionSettle)
		}
		st.key, st.shown, st.shownW = key, caps, width
	}
	if swap >= 1 {
		st.leaving = nil
	}
	ease := components.Smoothstep(swap)
	width := st.shownW
	if st.leaving != nil {
		width = st.leavingW + (st.shownW-st.leavingW)*ease
	}

	recording := float32(0)
	if s.keyEditing == command && s.keyHost == host {
		recording = 1
	}
	if animate {
		recording = components.Follow(chip, "record", recording, components.MotionQuick)
	}
	hover := components.HoverAmount(chip, editable && chip.Hovered(), animate)
	ring := s.keyRing(chip, triggerKey(host, command), s.keyEditing == command && s.keyHost == host, animate)

	// The object stands on the baseline as the caps center on the words'
	// lowercase letters, above the baseline by InlineCapMiddle.
	height := geometry.cap + 2*geometry.margin
	middle := components.InlineCapMiddle(fontSize)
	chip.Children(func() {
		ui.RichText(c, ui.Span{
			Text: "\uFFFC", ObjectWidth: width + 2*geometry.margin,
			ObjectHeight: height/2 + middle, ObjectDescent: height/2 - middle,
		})
	})
	if editable && chip.Clicked() {
		s.startRecording(command, host)
	}
	s.returnFocus(chip, triggerKey(host, command))

	ink := semantic.Text.Default
	if unbound {
		ink = semantic.Text.Muted
	}
	ink = ink.Mix(semantic.Accent.Foreground, recording)
	face := semantic.Surface.Inset.Mix(semantic.GhostElement.Hover, hover).
		Mix(semantic.Accent.Muted, recording)
	edge := semantic.Border.Variant.Mix(semantic.Border.Focused, ring)
	shown, leaving := st.shown, st.leaving
	out, in := swapGlyphs(swap)
	return func(p *ui.Painter, r ui.Rect) {
		x0 := r.X + geometry.margin
		top := r.Y + (r.H-geometry.cap)/2
		// Glyphs center optically on the cap via GlyphCenterBaseline.
		baseline := components.GlyphCenterBaseline(top+geometry.cap/2, geometry.glyph)
		for index := range max(len(shown), len(leaving)) {
			now, was := capAt(shown, index), capAt(leaving, index)
			hasNow, hasWas := index < len(shown), index < len(leaving)
			x, w := now.x, now.w
			if leaving != nil {
				x, w = morph(leaving, shown, index, ease)
			}
			if w < 0.5 {
				continue
			}
			cap := ui.Rect{X: x0 + x, Y: top, W: w, H: geometry.cap}
			// A cap coming or going is only as solid as it is wide.
			solid := min(1, 2*w/geometry.cap)
			p.Fill(cap, face.Alpha(solid), geometry.radius)
			if unbound && leaving == nil {
				p.StrokeDashed(cap, edge, geometry.radius, 1)
			} else {
				p.Stroke(cap, edge.Alpha(solid), geometry.radius, 1)
			}
			p.Clip(cap, geometry.radius, func() {
				if leaving == nil {
					p.Glyphs(now.glyphs, cap.X+(cap.W-now.ink)/2, baseline, ink)
					return
				}
				if hasWas && out > 0 {
					p.Glyphs(was.glyphs, cap.X+(cap.W-was.ink)/2, baseline, ink.Alpha(out))
				}
				if hasNow && in > 0 {
					p.Glyphs(now.glyphs, cap.X+(cap.W-now.ink)/2, baseline, ink.Alpha(in))
				}
			})
		}
	}
}

// morph is where cap index stands at ease between two keys. A cap one key
// lacks grows out of, or shrinks into, the other key's end, which is where
// the key's box ends too, so a cap never stands outside it.
func morph(from, to []keyCap, index int, ease float32) (x, w float32) {
	was, now := capAt(from, index), capAt(to, index)
	if index >= len(from) {
		was = keyCap{x: capsEnd(from)}
	}
	if index >= len(to) {
		now = keyCap{x: capsEnd(to)}
	}
	return was.x + (now.x-was.x)*ease, was.w + (now.w-was.w)*ease
}

func capsEnd(caps []keyCap) float32 {
	if len(caps) == 0 {
		return 0
	}
	last := caps[len(caps)-1]
	return last.x + last.w
}

// swapGlyphs is how much of the old and new glyphs show at swap: the old
// are out by a third of the way and the new come in only after, so two keys
// never show in one cap.
func swapGlyphs(swap float32) (out, in float32) {
	return 1 - components.Smoothstep(swap*3), components.Smoothstep((swap - 0.35) / 0.65)
}

func capAt(caps []keyCap, index int) keyCap {
	if index < len(caps) {
		return caps[index]
	}
	return keyCap{}
}

// chipState is the key a chip shows and, while it changes, the caps it is
// leaving, as they showed when the change began.
type chipState struct {
	key              string
	shown, leaving   []keyCap
	shownW, leavingW float32
}

// at is what shows at swap: the caps settled, or, mid-change, the caps of
// whichever key is in view at the places they have eased to.
func (st *chipState) at(swap float32) ([]keyCap, float32) {
	if st.leaving == nil || swap >= 1 {
		return st.shown, st.shownW
	}
	ease := components.Smoothstep(swap)
	source := st.shown
	if swap < 0.35 {
		source = st.leaving
	}
	caps := make([]keyCap, len(source))
	for index := range source {
		caps[index] = source[index]
		caps[index].x, caps[index].w = morph(st.leaving, st.shown, index, ease)
	}
	return caps, st.leavingW + (st.shownW-st.leavingW)*ease
}
