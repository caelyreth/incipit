package shell

import (
	"fmt"
	"math"

	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A block is a group as the field shows it: its head, then what it holds
// in full, row by row, as the sheet list showed it, one column wide and no
// taller than its half of the field holds, the rest counted. A block is as
// tall as what it holds, laid out once and moved as a whole
// (stageFrame), its layer kept.

// shape is how a block stands: its rows, column by column, its size, and
// how many of its sheets it leaves out.
type shape struct {
	size stageSize
	cols [][]row
	more int
}

// row is one row of a card: a group inside it, a sheet, a command, a
// settings section, the area that makes a sheet there, or what it leaves
// out.
type row struct {
	group   *GroupItem
	entry   *entry
	command *Command
	setting *SettingsSection
	create  bool
	h       float32
}

// blockMetrics are the measures blocks are laid out to, at the type's
// size, for a field w by h.
type blockMetrics struct {
	scale, k       float32
	col, pad, head float32
	// headGap is the room between a card's head and its first row, and
	// rowGap between rows: no two things that light under the pointer
	// touch (field-and-desk.md §4.7.2).
	headGap float32
	// tall is how tall a block's rows may stand in its half of the field,
	// and alone how tall where its row is all the field holds, nothing
	// over the pill.
	rowGap, tall      float32
	alone             float32
	groupRow, control float32
	horizon           stageHorizon
}

// measureStep is the step, at the reference type size, the field's
// measures are taken in from the window's.
const measureStep = 8

// fieldGap is the room between columns in the field and between the field
// and the desk lane, at the reference type size.
const fieldGap = 14

// fieldMetrics are the normal field's column measures, fixed for the
// window's size and type scale regardless of what query is shown.
func (s *State) fieldMetrics(w, h float32) blockMetrics {
	return s.columnMetrics(w, h, 260, 340)
}

func (s *State) blockMetrics(w, h float32) blockMetrics {
	// Live settings cards are wider, for a setting's words and its control
	// to stand side by side; category cards share the library's measure.
	if s.settingsLive() {
		return s.columnMetrics(w, h, 400, 560)
	}
	return s.fieldMetrics(w, h)
}

func (s *State) columnMetrics(w, h, least, most float32) blockMetrics {
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	pad, gap := 6*k, fieldGap*k
	cols, col := stageColumns(w-48*k, least*k, most*k, gap)
	// What the window gives is taken in steps, so that resizing it lays
	// the cards out again a step at a time and between steps only moves
	// them, as they were drawn.
	step := measureStep * k
	if col < most*k {
		col = max(step, float32(math.Floor(float64(col/step)))*step)
	}
	band := 40*k + 2*gap
	// What a card takes past its rows: its head, the room under it, and
	// its padding.
	head, headGap := 36*k, 6*k
	frame := head + headGap + 2*pad
	return blockMetrics{
		scale: scale, k: k,
		col: col - 2*pad, pad: pad, head: head, headGap: headGap, rowGap: 4 * k,
		// A block is no taller than its half of the field holds, or, alone
		// in its row, than the window holds about the pill.
		tall:     max(160*k, float32(math.Floor(float64((h/2-band/2-gap-frame)/step)))*step),
		alone:    max(160*k, float32(math.Floor(float64((h-band-2*gap-frame-(48+24)*k)/step)))*step),
		groupRow: 2*theme.Space06.DIPs(scale) + theme.TextSmall.LineHeight(scale),
		control:  theme.DefaultControls.Scaled(scale).Height,
		horizon:  stageHorizon{Cols: cols, Col: col, Gap: gap, Above: band / 2, Below: band / 2},
	}
}

// shapes are the cards' shapes, in their order, each worked out again only
// when what they hold or the measures change.
func (s *State) shapes(c *ui.Context, f found, m blockMetrics) []shape {
	key := fmt.Sprintf("%s|%d|%s|%d|%d|%v", f.query, f.rev, f.open, f.held, s.field.measures, m)
	if s.field.shapesKey != key {
		s.field.shapes, s.field.shapesKey = map[string]shape{}, key
	}
	// A card shaped for one view is not shaped for the other, as a
	// group's card in the field is its divider in the list: each view
	// keeps its own, so turning between them shapes nothing again.
	view := ""
	if f.listed {
		view = "list:"
	}
	out := make([]shape, len(f.cards))
	for i, cd := range f.cards {
		sh, ok := s.field.shapes[view+cd.id]
		if !ok {
			sh = cd.kind.shape(s, c, m)
			s.field.shapes[view+cd.id] = sh
		}
		out[i] = sh
	}
	return out
}

func (b block) shape(s *State, c *ui.Context, m blockMetrics) shape {
	switch {
	case b.single:
		h := entryHeight(c, b.entries[0], m, 4)
		return shape{size: stageSize{W: m.col + 2*m.pad, H: h + 2*m.pad}, cols: [][]row{{{entry: &b.entries[0], h: h}}}}
	case b.create:
		return shape{size: stageSize{W: m.col + 2*m.pad, H: m.control + 2*m.pad}, cols: [][]row{{{create: true, h: m.control}}}}
	case b.divider || b.section != "":
		// Across the list, as wide as its columns.
		z := m.horizon
		return shape{size: stageSize{W: float32(z.Cols)*z.Col + float32(z.Cols-1)*z.Gap, H: m.head}}
	}
	rows := make([]row, 0, len(b.groups)+len(b.entries)+1)
	pinnedFirst(b.groups, b.entries, func(i int) {
		rows = append(rows, row{group: &b.groups[i], h: m.groupRow})
	}, func(i int) {
		rows = append(rows, row{entry: &b.entries[i], h: entryHeight(c, b.entries[i], m, 2)})
	})
	// Where sheets can be made, the area that makes one ends the block.
	if g := b.group; g != nil && !b.trashed && !g.Smart() {
		rows = append(rows, row{create: true, h: m.control})
	}
	return stack(rows, m)
}

func (b block) rows() []string {
	ids := make([]string, 0, len(b.groups)+len(b.entries)+1)
	// A group's card is its head's row first, as the keys go.
	if b.group != nil && !b.single && !b.create {
		ids = append(ids, "group:"+b.group.ID)
	}
	pinnedFirst(b.groups, b.entries, func(i int) {
		ids = append(ids, "group:"+b.groups[i].ID)
	}, func(i int) {
		ids = append(ids, "sheet:"+b.entries[i].sheet.ID)
	})
	return ids
}

func (b block) ground() bool { return !b.create && !b.divider && b.section == "" }

// A block is drawn anew while its group's name is being written.
func (b block) drawsLive(s *State) bool {
	return b.group != nil && s.field.renaming == b.group.ID && !b.create
}

// The parts of a sheet's row, as the sheet list set them.
func rowPad(scale float32) float32 { return theme.Space08.DIPs(scale) }
func rowGap(scale float32) float32 { return theme.Space02.DIPs(scale) }

// tagsHeight is the height of a row's line of tags.
func tagsHeight(scale float32) float32 {
	return theme.TextXSmall.LineHeight(scale) + 3*theme.Space04.DIPs(scale)
}

// entryHeight is how tall a sheet's row stands: its title and time, its
// opening, or where it matches, in up to lines lines, its tags, and its
// words.
func entryHeight(c *ui.Context, e entry, m blockMetrics, lines int) float32 {
	pad, gap := rowPad(m.scale), rowGap(m.scale)
	xs := theme.TextXSmall.LineHeight(m.scale)
	h := 2*pad + theme.TextSmall.LineHeight(m.scale) + gap + xs
	if body := entryBody(e); body != "" {
		h += gap + float32(bodyLines(c, body, m.col-2*pad, m.scale, lines))*xs
	}
	if len(e.sheet.Tags) > 0 {
		h += gap + tagsHeight(m.scale)
	}
	return h
}

func entryBody(e entry) string {
	if e.line != "" {
		return e.line
	}
	return e.sheet.Excerpt
}

// bodyLines is how many lines, up to most, body takes width wide.
func bodyLines(c *ui.Context, body string, width, scale float32, most int) int {
	span := ui.Span{Text: body, Font: theme.UIFontFamily, Size: theme.TextXSmall.Size(scale)}
	_, one := c.MeasureText(0, ui.Span{Text: "Ag", Font: span.Font, Size: span.Size})
	_, h := c.MeasureText(width, span)
	return max(1, min(most, int(h/one+0.5)))
}
