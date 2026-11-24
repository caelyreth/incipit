package shell

import (
	"github.com/caelyreth/neverland/internal/ui/components"
	"github.com/caelyreth/neverland/internal/ui/theme"
	"github.com/caelyreth/neverland/locales"
	"github.com/egoist/mygo/ui"
)

// Ambient cards are a few quiet cards the field shows when nothing is
// asked (field-and-desk.md §4.4.1): how much was written today, a line
// for the day. They answer only an empty query, stand after everything
// else, are never more than ambientMost, and each says itself whether it
// shows, turned off or with nothing to say.

// ambient is one kind of ambient card.
type ambient interface {
	// id names the card among the field's.
	id() string
	// shows reports whether the card stands in the field now.
	shows(s *State) bool
	// card is the card, as it stands now.
	card(s *State) cardKind
}

// ambients are the ambient cards, in the order they are offered.
var ambients = []ambient{firstRun{}, writtenToday{}}

// ambientMost is how many ambient cards stand at once.
const ambientMost = 2

type ambientSource struct{}

func (ambientSource) answer(s *State, text string, f *found) {
	if text != "" || s.field.words != "" {
		return
	}
	shown := 0
	for _, a := range ambients {
		if shown == ambientMost {
			return
		}
		if a.shows(s) {
			f.add("ambient:"+a.id(), tierAmbient, a.card(s))
			shown++
		}
	}
}

// note is an ambient card's face: a head saying what it is, then a few
// lines, none of which is pressed.
type note struct {
	icon  components.IconName
	title string
	lines []noteLine
}

// noteLine is one line of a note, and the key it tells of, if any. A
// line with run is a button that runs it.
type noteLine struct {
	text string
	keys []string
	run  func()
}

func (n note) shape(_ *State, _ *ui.Context, m blockMetrics) shape {
	rows := make([]row, len(n.lines))
	for i := range n.lines {
		rows[i] = row{line: &n.lines[i], h: m.groupRow}
	}
	return stack(rows, m)
}

func (note) rows() []string { return nil }

func (note) ground() bool { return true }

// A note with a button is drawn anew each frame, for its hover.
func (n note) drawsLive(*State) bool {
	for _, l := range n.lines {
		if l.run != nil {
			return true
		}
	}
	return false
}

func (n note) view(_ *State, c *ui.Context, semantic theme.Theme, box ui.Element, _ stageFrame, sh shape, _ bool, m blockMetrics, _ bool) {
	box.Gap(m.headGap).Children(func() {
		cardHead(c, semantic, n.icon, n.title, m)
		cardRows(c, sh, m, func(r row) {
			if r.line == nil {
				moreLabel(c, semantic, sh.more, r.h, m)
				return
			}
			ink := semantic.Text.Muted
			line := ui.Row(c)
			if run := r.line.run; run != nil {
				btn := ui.ButtonBase(c.Key("note-" + r.line.text)).Label(r.line.text).Cursor(ui.CursorDefault).FocusRing(false)
				if btn.Clicked() {
					run()
				}
				ink = semantic.Text.Default
				if btn.Hovered() {
					btn.Background(semantic.GhostElement.Hover).Radius(6 * m.k)
				}
				line = btn
			}
			line.Height(r.h).WidthPercent(100).PaddingX(rowPad(m.scale)).Gap(8 * m.k).
				AlignItems(ui.Center).Children(func() {
				components.CenteredLabel(c, r.line.text, theme.TextSmall, ink).Grow(1).MinWidth(0)
				if len(r.line.keys) > 0 {
					components.Keycaps(c, components.KeycapStyle(semantic, m.scale, true), r.line.keys, "").Shrink(0)
				}
			})
		})
	})
}

// firstRun says how to begin, while nothing in the notebook is written:
// gone with its first word, back only were every word let go.
type firstRun struct{}

func (firstRun) id() string { return "first-run" }

func (firstRun) shows(s *State) bool {
	for _, place := range []Place{{Kind: PlaceAll}, {Kind: PlaceTrash}} {
		for _, sh := range s.Library.Sheets(place) {
			if sh.Words > 0 {
				return false
			}
		}
	}
	return true
}

func (firstRun) card(s *State) cardKind {
	return note{icon: components.IconPencilSpark, title: locales.ANewNotebook(), lines: []noteLine{
		{text: locales.TypeToFind()},
		{text: locales.WriteANewSheet(), keys: s.commandKeys("page.new")},
		{text: locales.DragToFile()},
	}}
}

// commandKeys are the caps of the key command id runs on, if it has one.
func (s *State) commandKeys(id string) []string {
	if s.Commands == nil {
		return nil
	}
	for _, cmd := range s.Commands() {
		if cmd.ID == id {
			return cmd.Keys
		}
	}
	return nil
}

// writtenToday tells how many words were written today, while it has
// any to tell.
type writtenToday struct{}

func (writtenToday) id() string { return "written-today" }

func (writtenToday) shows(s *State) bool { return s.WrittenToday != nil && s.WrittenToday() > 0 }

func (writtenToday) card(s *State) cardKind {
	return note{icon: components.IconPen, title: locales.WrittenToday(), lines: []noteLine{{text: words(s.WrittenToday())}}}
}

// nothingFound answers a search that finds nothing: a quiet card under
// the pill saying so and what was asked, with Clear Search to press
// (field-and-desk.md §4.7.7). It stands after every source, alone.
type nothingFound struct{}

func (nothingFound) answer(s *State, _ string, f *found) {
	if s.field.words == "" || len(f.cards) > 0 {
		return
	}
	f.add("nothing-found", tierAmbient, note{icon: components.IconSearch, title: locales.NothingFound(), lines: []noteLine{
		{text: locales.NothingHolds(s.field.words)},
		{text: locales.ClearSearch(), keys: s.commandKeys(VerbHome), run: s.goHome},
	}})
}
