package page

import (
	"slices"
	"strings"
	"time"

	"github.com/caelyreth/neverland/editor/doc"
	"github.com/caelyreth/neverland/editor/present"
	"github.com/caelyreth/neverland/editor/profile"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A tag being written opens a picker under it: the notebook's tags its
// name begins or holds, and, where none is it, one that makes it. The
// arrows choose, Return or Tab picks (profile.PickTag), Escape closes it
// until the tag is written in again. It opens only as the tag is typed or
// deleted in, so moving through text past tags never opens it, and it
// closes as the caret leaves the tag. The tag is what the page presents
// in the tag group, so the picker reads what the policy reads.

// tagRowsShown is how many tags the picker offers at most.
const tagRowsShown = 6

// tagKey is a tag where it stands: its slot and its start.
type tagKey struct {
	slot doc.SlotRef
	from int
}

// tagHere is the tag the caret is in: where it stands and its name.
type tagHere struct {
	key  tagKey
	name string
}

// tagRow is one row the picker offers: a tag of the notebook's, or the
// tag being written, made.
type tagRow struct {
	name   string
	create bool
}

// tagPicker is the picker's state: the tag it is open for, its rows as
// last shown and the one chosen.
type tagPicker struct {
	at     *tagHere
	rows   []tagRow
	chosen int
	// steered is whether the keys or the pointer chose the row: until
	// they do, the choice stays on the best match as the name is written,
	// and once they do, it stays on the row chosen while it is offered.
	steered bool
	// placed is whether the side the picker opened on is settled, above
	// whether that side is over the tag: it stays there while the picker
	// is open, so it never jumps across the tag as rows come and go.
	placed, above bool
	// pointer is where the pointer was over the picker last frame, in
	// the page, pointed whether it was over it: only the pointer's moves
	// choose, so rows passing under a resting pointer leave the choice to
	// the keys.
	pointer ui.Point
	pointed bool
}

func (tp *tagPicker) open() bool { return tp.at != nil && len(tp.rows) > 0 }

// tagAt is the tag holding the caret at c, from the start of its name to
// its end, as the page presents it.
func (pl *pageLayout) tagAt(c doc.Caret) (tagHere, bool) {
	pc, ok := pl.lookup(c)
	if !ok || pc.surface == nil || pc.surface.index >= len(pc.unit.pres.Surfaces) {
		return tagHere{}, false
	}
	off := c.Pos.Offset
	type run struct {
		from, to int
		text     string
	}
	var runs []run
	for _, it := range pc.unit.pres.Surfaces[pc.surface.index].Flow.Items {
		if it.Kind != present.TextRun || !slices.Contains(it.Groups, profile.GroupTag) {
			continue
		}
		if n := len(runs); n > 0 && runs[n-1].to == it.From {
			runs[n-1].to, runs[n-1].text = it.To, runs[n-1].text+it.Text
		} else {
			runs = append(runs, run{it.From, it.To, it.Text})
		}
	}
	for _, r := range runs {
		if r.from < off && off <= r.to {
			name := strings.TrimPrefix(r.text, string(profile.TagSpelling))
			return tagHere{key: tagKey{slot: pc.surface.slot, from: r.from}, name: name}, true
		}
	}
	return tagHere{}, false
}

// tagRows are what the picker offers for the name being written, from the
// notebook's tags, most written first: the tag it already is, then those
// it begins, then those it is in, and, where none is it, the tag made.
func tagRows(tags []string, name string) []tagRow {
	key := profile.TagKey(name)
	var same []tagRow
	var begun, held []tagRow
	for _, tag := range tags {
		switch k := profile.TagKey(tag); {
		case k == key:
			same = append(same, tagRow{name: tag})
		case strings.HasPrefix(k, key):
			begun = append(begun, tagRow{name: tag})
		case strings.Contains(k, key):
			held = append(held, tagRow{name: tag})
		}
	}
	rows := append(append(same[:min(len(same), 1)], begun...), held...)
	rows = rows[:min(len(rows), tagRowsShown)]
	if len(same) == 0 {
		rows = append(rows, tagRow{name: name, create: true})
	}
	return rows
}

// syncTags opens, keeps or closes the picker for the caret as the page now
// stands: it opens for the tag the last input typed or deleted in, stays
// while the caret is in that tag, and closes as the caret leaves it.
func (el *Element) syncTags() {
	ed := el.Editor
	tp := &el.tags
	typed := ed.typed
	ed.typed = false
	sel := ed.Selection()
	here, ok := tagHere{}, false
	if ed.layout != nil && sel.Kind == doc.SelectText && sel.IsCollapsed() {
		here, ok = ed.layout.tagAt(sel.Head)
	}
	switch {
	case !ok:
		tp.close()
		return
	case typed:
		if tp.at == nil || tp.at.key != here.key {
			*tp = tagPicker{}
		}
	case tp.at == nil || tp.at.key != here.key:
		tp.close()
		return
	}
	tp.at = &here
	var tags []string
	if ed.Tags != nil {
		tags = ed.Tags(here.name)
	}
	rows := tagRows(tags, here.name)
	chosen := 0
	if tp.steered && tp.chosen < len(tp.rows) {
		was := tp.rows[tp.chosen]
		if i := slices.IndexFunc(rows, was.same); i >= 0 {
			chosen = i
		} else {
			tp.steered = false
		}
	}
	tp.rows, tp.chosen = rows, chosen
}

// close closes the picker.
func (tp *tagPicker) close() { *tp = tagPicker{} }

// same reports whether row is r as the name being written changes: the
// same tag, or the tag made.
func (r tagRow) same(row tagRow) bool {
	return r.create == row.create && (r.create || profile.TagKey(r.name) == profile.TagKey(row.name))
}

// tagKeys answers the picker's keys while it is open.
func (el *Element) tagKeys(ev ui.InputEvent, services ui.Services, at time.Time) bool {
	tp := &el.tags
	if !tp.open() || ev.Mods&^ui.Shift != 0 {
		return false
	}
	switch ev.Key {
	case ui.KeyDown:
		tp.chosen, tp.steered = (tp.chosen+1)%len(tp.rows), true
	case ui.KeyUp:
		tp.chosen, tp.steered = (tp.chosen+len(tp.rows)-1)%len(tp.rows), true
	case ui.KeyEnter, ui.KeyTab:
		el.pickTag(tp.chosen, services, at)
	case ui.KeyEscape:
		tp.close()
	default:
		return false
	}
	return true
}

// pickTag names the tag being written as row i offers.
func (el *Element) pickTag(i int, services ui.Services, at time.Time) {
	tp := &el.tags
	if i < 0 || i >= len(tp.rows) {
		return
	}
	name := tp.rows[i].name
	tp.close()
	el.run(profile.PickTag{Name: name}, services, at)
}

// tagPickerView draws the picker under the tag it is open for, or over it
// where the page has no room below, its first tag's text beginning where
// the tag's does. It is the page's own: outlined, not cast in shadow, and
// each tag in it is shown as the page and the sheet list show one. It
// comes in from the tag and fades out where it stood; its height springs
// as rows come and go, rows open and fold away in place as the others slide,
// and the chosen row's fill slides to it.
func (el *Element) tagPickerView(c *ui.Context, services ui.Services) {
	tp := &el.tags
	pl := el.Editor.layout
	if !tp.open() || pl == nil {
		return
	}
	start, ok := pl.caretBox(doc.Caret{Pos: doc.At(tp.at.key.slot, tp.at.key.from)}, el.Editor.metrics)
	if !ok {
		return
	}
	semantic := el.Theme
	scale := components.UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	pad := theme.Space04.DIPs(scale)
	inset := theme.Space06.DIPs(scale)
	chipPad := theme.Space04.DIPs(scale)
	gap := theme.Space04.DIPs(scale)
	const border = 1
	row := metrics.Height
	width := 240 * scale / theme.DefaultUIFontSize
	height := float32(len(tp.rows))*row + 2*(pad+border)
	r := el.fromPage(start)
	if !tp.placed {
		// The side is chosen for the most rows the picker can show.
		most := float32(tagRowsShown+1)*row + 2*(pad+border)
		tp.above = r.Y+r.H+gap+most > el.viewport.H && r.Y-gap-most >= 0
		tp.placed = true
	}
	spring := components.Spring(components.MotionNormal)
	rise := -theme.Space04.DIPs(scale)
	panel := ui.Column(c).Key("tag-picker").Absolute().
		Left(max(0, min(r.X-border-pad-inset-chipPad, el.viewport.W-width))).
		Width(width).Height(height).Padding(pad, pad, pad, pad).Clip().
		Role(ui.RoleList).Label("Tags")
	if tp.above {
		// Anchored by its foot, so it grows up, away from the tag.
		panel.Bottom(el.viewport.H - (r.Y - gap))
		rise = -rise
	} else {
		panel.Top(r.Y + r.H + gap)
	}
	components.OutlinedSurface(panel, semantic, scale)
	panel.Transition(ui.ElementTransition{
		Animation: &spring, Height: true,
		Enter: &ui.Motion{Y: rise, Opacity: 0}, Exit: &ui.Motion{Y: rise / 2, Opacity: 0},
		ExitDuration: components.MotionQuick,
	})
	// Only the pointer's moves choose the row under it.
	px, py, over := panel.PointerPosition()
	if p := (ui.Point{X: px, Y: py}); over {
		if tp.pointed && p != tp.pointer {
			if i := int((py - border - pad) / row); i >= 0 && i < len(tp.rows) {
				tp.chosen, tp.steered = i, true
			}
		}
		tp.pointer, tp.pointed = p, true
	} else {
		tp.pointed = false
	}
	slide := ui.ElementTransition{Animation: &spring, Position: true}
	moving := ui.ElementTransition{
		Animation: &spring, Position: true,
		Enter: &ui.Motion{Collapse: true, Opacity: 0}, Exit: &ui.Motion{Collapse: true, Opacity: 0},
	}
	panel.Children(func() {
		// The chosen row's fill, under the rows, sliding as the choice
		// moves.
		ui.Box(c).Key("tag-chosen").Absolute().Left(pad).Right(pad).
			Top(pad + float32(tp.chosen)*row).Height(row).
			Radius(metrics.PanelRadius - pad).Background(semantic.GhostElement.Hover).ZIndex(-1).
			Transition(slide)
		for i, tr := range tp.rows {
			key, label := "tag-"+profile.TagKey(tr.name), "#"+tr.name
			if tr.create {
				key, label = "tag-create", "Create #"+tr.name
			}
			button := ui.ButtonBase(c.Key(key)).Height(row).WidthPercent(100).Shrink(0).
				PaddingX(inset).AlignItems(ui.Center).Justify(ui.SpaceBetween).
				Gap(theme.Space06.DIPs(scale)).FocusRing(false).Selected(i == tp.chosen).
				Background(ui.Transparent).Label(label).Transition(moving)
			button.Children(func() {
				components.TagChip(c, "#"+tr.name, theme.TextSmall, chipPad, width, semantic.Tag.Background, semantic.Tag.Foreground)
				if tr.create {
					ui.Row(c).Gap(theme.Space04.DIPs(scale)).AlignItems(ui.Center).Shrink(0).Children(func() {
						ui.Icon(c, components.IconPlus.SVG()).FontSize(theme.TextXSmall.Size(scale)).
							TextColor(semantic.Text.Muted)
						components.CenteredLabel(c, "Create", theme.TextXSmall, semantic.Text.Muted)
					})
				}
			})
			if button.Clicked() {
				el.pickTag(i, services, c.Now())
				el.Focus()
				services.Invalidate()
			}
		}
	})
}
