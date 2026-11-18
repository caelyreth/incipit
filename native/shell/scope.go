package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A place the query is in may stand first in the field as a card of its
// own (field-and-desk.md §4.4.2): what the place is, and what is done to
// it whole, nearest the pill, before what it holds. The trash is the
// first: what it holds, counted, and the button that empties it.

// trashCard is the trash's own card.
type trashCard struct {
	groups, sheets int
}

func (trashCard) shape(_ *State, _ *ui.Context, m blockMetrics) shape {
	return shape{size: stage.Size{W: m.col + 2*m.pad, H: m.head + m.groupRow + 3*m.pad}}
}

func (trashCard) rows() []string { return nil }

func (trashCard) ground() bool { return true }

// drawsLive: its button turns as it is armed.
func (trashCard) drawsLive(*State) bool { return true }

func (t trashCard) view(s *State, c *ui.Context, semantic theme.Theme, box ui.Element, _ stage.Frame, _ shape, _ bool, m blockMetrics, animate bool) {
	box.Gap(m.pad).Children(func() {
		cardHead(c, semantic, components.IconTrash, "Trash", m)
		ui.Row(c).Key("body").Height(m.groupRow).PaddingX(rowPad(m.scale)).Gap(8 * m.k).
			AlignItems(ui.Center).Children(func() {
			components.CenteredLabel(c, t.count(), theme.TextSmall, semantic.Text.Muted).Grow(1).MinWidth(0)
			if t.groups+t.sheets > 0 {
				s.confirmButton(c, semantic, emptyTrashKey, "Empty Trash…", "Empty Trash", animate, func() { s.Library.EmptyTrash() })
			}
		})
	})
}

// count says what the trash holds.
func (t trashCard) count() string {
	switch {
	case t.groups+t.sheets == 0:
		return "Nothing in the trash"
	case t.groups == 0:
		return thousands(t.sheets) + " " + nounForm(t.sheets, "sheet", "sheets")
	case t.sheets == 0:
		return thousands(t.groups) + " " + nounForm(t.groups, "group", "groups")
	}
	return thousands(t.sheets) + " " + nounForm(t.sheets, "sheet", "sheets") + " · " + thousands(t.groups) + " " + nounForm(t.groups, "group", "groups")
}
