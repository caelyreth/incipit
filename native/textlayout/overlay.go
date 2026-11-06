package textlayout

import (
	"maps"
	"slices"

	"github.com/caelyreth/neverland/engine/model"
	"github.com/caelyreth/neverland/engine/surface"
)

// Overlay is how a view shows a flow apart from what the flow is: how far
// each formula's card and each caption's line have opened, the width a
// figure takes while it is resized, the room tables and the page open as
// rows, columns and blocks come and go. The flow is the editor's and stays
// as it says; an overlay is the view's, and its zero value shows the flow
// as it is.
type Overlay struct {
	// Formulas are the formulas shown being edited, by where each starts.
	Formulas map[FormulaKey]FormulaShow
	// Captions are how far each figure's caption line has opened, from 0,
	// none, to 1, a line's; a figure not here takes its line as far as
	// it has a caption.
	Captions map[model.NodeID]float32
	// Spans are the share of the column a figure takes, while a view sets
	// it otherwise than the flow, as a resize under way does.
	Spans map[model.NodeID]float32
	// Tables are the room each table opens, by the table's node.
	Tables map[model.NodeID]TableRoom
	// Rooms are the room the page opens before its lines.
	Rooms []Room
}

// FormulaKey names a formula by its block and where it starts, which
// typing in its source does not move.
type FormulaKey struct {
	Node  model.NodeID
	Start model.UTF16Offset
}

// KeyOf is the key of f.
func KeyOf(f surface.Formula) FormulaKey { return FormulaKey{Node: f.Node, Start: f.Range.Start} }

// FormulaShow is how a formula being edited shows: Open while its source
// is edited on its card, Card how far that card has opened, from 0 to 1,
// and, Stale, what it last set soundly in place of what it now reads,
// which cannot be set.
type FormulaShow struct {
	Open  bool
	Card  float32
	Stale bool
	Last  string
}

// Formula is a flow's formula as the page shows it: Set is the TeX it
// sets, what its source reads unless it is stale.
type Formula struct {
	surface.Formula
	FormulaShow
	Set string
}

// formula is f as the overlay shows it.
func (o *Overlay) formula(f surface.Formula) Formula {
	shown := Formula{Formula: f, Set: f.Source}
	if o == nil {
		return shown
	}
	shown.FormulaShow = o.Formulas[KeyOf(f)]
	shown.Card = max(0, min(1, shown.Card))
	if shown.Stale {
		shown.Set = shown.Last
	}
	return shown
}

// caption is the share of its line a figure's caption takes.
func (o *Overlay) caption(figure surface.Figure) float32 {
	if o != nil {
		if open, ok := o.Captions[figure.Node]; ok {
			return max(0, min(1, open))
		}
	}
	if figure.Captioned {
		return 1
	}
	return 0
}

// span is the share of the column a figure takes, zero for its own size.
func (o *Overlay) span(figure surface.Figure) float32 {
	if o != nil {
		if span, ok := o.Spans[figure.Node]; ok {
			return max(0, min(1, span))
		}
	}
	return figure.Span
}

// table is the room table opens.
func (o *Overlay) table(table surface.Table) TableRoom {
	if o == nil {
		return TableRoom{}
	}
	return o.Tables[table.Node()]
}

// roomAt is the room the page opens before the line at at, -1 for after
// the last.
func (o *Overlay) roomAt(at model.UTF16Offset) float32 {
	room := float32(0)
	if o == nil {
		return room
	}
	for _, candidate := range o.Rooms {
		if candidate.At == at {
			room += candidate.Height
		}
	}
	return room
}

// Equal reports whether two overlays show a flow alike, so a layout of one
// is a layout of the other.
func (o *Overlay) Equal(other *Overlay) bool {
	var none Overlay
	if o == nil {
		o = &none
	}
	if other == nil {
		other = &none
	}
	return maps.Equal(o.Formulas, other.Formulas) && maps.Equal(o.Captions, other.Captions) &&
		maps.Equal(o.Spans, other.Spans) && slices.Equal(o.Rooms, other.Rooms) &&
		maps.EqualFunc(o.Tables, other.Tables, TableRoom.equal)
}

// clone is a copy of the overlay sharing nothing with it.
func (o *Overlay) clone() *Overlay {
	if o == nil {
		return nil
	}
	next := &Overlay{
		Formulas: maps.Clone(o.Formulas),
		Captions: maps.Clone(o.Captions),
		Spans:    maps.Clone(o.Spans),
		Rooms:    slices.Clone(o.Rooms),
	}
	if o.Tables != nil {
		next.Tables = make(map[model.NodeID]TableRoom, len(o.Tables))
		for node, room := range o.Tables {
			next.Tables[node] = room.clone()
		}
	}
	return next
}

// Room is space a view opens before the line at At, as a drag parts the
// page where what it carries would land; At of -1 opens it after the last
// line.
type Room struct {
	At     model.UTF16Offset
	Height float32
}

// TableRoom is room a view opens in and below a table, in DIPs, and how
// far each row and column has come in. Its zero value opens none.
type TableRoom struct {
	// Below is room under the table, for a new row's area.
	Below float32
	// RowRoom and ColumnRoom open before row Row and column Column, for
	// an area making one there.
	Row, Column         int
	RowRoom, ColumnRoom float32
	// RowShown and ColumnShown are each row's and column's share of its
	// size, coming in from 0; all 1 when nil.
	RowShown, ColumnShown []float32
	// Scroll is how far a table wider than the room it shows in is
	// scrolled across, and Pull how far an edge's pull draws it on past
	// that, elastic, as a scroll goes past its end.
	Scroll, Pull float32
	// RowsApart and ColumnsApart are room a drag opens before each row
	// and column, the last after them all, where what it carries would
	// land.
	RowsApart, ColumnsApart []float32
}

func (r TableRoom) equal(other TableRoom) bool {
	return r.Below == other.Below && r.Row == other.Row && r.Column == other.Column &&
		r.RowRoom == other.RowRoom && r.ColumnRoom == other.ColumnRoom &&
		r.Scroll == other.Scroll && r.Pull == other.Pull &&
		slices.Equal(r.RowShown, other.RowShown) && slices.Equal(r.ColumnShown, other.ColumnShown) &&
		slices.Equal(r.RowsApart, other.RowsApart) && slices.Equal(r.ColumnsApart, other.ColumnsApart)
}

func (r TableRoom) clone() TableRoom {
	r.RowShown = slices.Clone(r.RowShown)
	r.ColumnShown = slices.Clone(r.ColumnShown)
	r.RowsApart = slices.Clone(r.RowsApart)
	r.ColumnsApart = slices.Clone(r.ColumnsApart)
	return r
}

// apart is the room at index of rooms, none past them.
func apart(rooms []float32, index int) float32 {
	if index < len(rooms) {
		return max(0, rooms[index])
	}
	return 0
}

// RowApart and ColumnApart are the room a drag opens before row and
// column.
func (r TableRoom) RowApart(row int) float32       { return apart(r.RowsApart, row) }
func (r TableRoom) ColumnApart(column int) float32 { return apart(r.ColumnsApart, column) }

// RowShare and ColumnShare are row's and column's share of their size.
func (r TableRoom) RowShare(row int) float32 {
	if row < len(r.RowShown) {
		return max(0, min(1, r.RowShown[row]))
	}
	return 1
}

func (r TableRoom) ColumnShare(column int) float32 {
	if column < len(r.ColumnShown) {
		return max(0, min(1, r.ColumnShown[column]))
	}
	return 1
}
