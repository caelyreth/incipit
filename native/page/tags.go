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
		tp.at = nil
	case typed:
		if tp.at == nil || tp.at.key != here.key {
			tp.chosen = 0
		}
		tp.at = &here
	case tp.at != nil && tp.at.key == here.key:
		tp.at = &here
	default:
		tp.at = nil
	}
	if tp.at == nil {
		tp.rows = nil
		return
	}
	var tags []string
	if ed.Tags != nil {
		tags = ed.Tags(here.name)
	}
	tp.rows = tagRows(tags, here.name)
	tp.chosen = max(0, min(tp.chosen, len(tp.rows)-1))
}

// tagKeys answers the picker's keys while it is open.
func (el *Element) tagKeys(ev ui.InputEvent, services ui.Services, at time.Time) bool {
	tp := &el.tags
	if !tp.open() || ev.Mods&^ui.Shift != 0 {
		return false
	}
	switch ev.Key {
	case ui.KeyDown:
		tp.chosen = (tp.chosen + 1) % len(tp.rows)
	case ui.KeyUp:
		tp.chosen = (tp.chosen + len(tp.rows) - 1) % len(tp.rows)
	case ui.KeyEnter, ui.KeyTab:
		el.pickTag(tp.chosen, services, at)
	case ui.KeyEscape:
		tp.at, tp.rows = nil, nil
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
	tp.at, tp.rows = nil, nil
	el.run(profile.PickTag{Name: name}, services, at)
}

// tagPickerView draws the picker under the tag it is open for, or over it
// where the page has no room below.
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
	unit := scale / theme.DefaultUIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	pad := theme.Space04.DIPs(scale)
	width := 240 * unit
	height := float32(len(tp.rows))*metrics.Height + 2*pad
	r := el.fromPage(start)
	gap := theme.Space04.DIPs(scale)
	x := max(0, min(r.X, el.viewport.W-width))
	y := r.Y + r.H + gap
	if y+height > el.viewport.H && r.Y-gap-height >= 0 {
		y = r.Y - gap - height
	}
	panel := ui.Column(c).Key("tag-picker").Absolute().Left(x).Top(y).Width(width).
		Padding(pad, pad, pad, pad).Role(ui.RoleList).Label("Tags")
	components.ElevatedSurface(panel, semantic, scale, components.PopoverElevation)
	panel.Children(func() {
		for i, row := range tp.rows {
			key := "tag-" + row.name
			if row.create {
				key = "tag-create"
			}
			button := ui.ButtonBase(c.Key(key)).Height(metrics.Height).WidthPercent(100).
				PaddingX(theme.Space08.DIPs(scale)).Radius(metrics.Radius).AlignItems(ui.Center).
				Gap(theme.Space06.DIPs(scale)).FocusRing(false).Selected(i == tp.chosen).
				Label("#" + row.name)
			if button.Hovered() {
				tp.chosen = i
			}
			fill := ui.Transparent
			if i == tp.chosen {
				fill = semantic.GhostElement.Hover
			}
			button.Background(fill)
			button.Children(func() {
				if row.create {
					components.Label(c, "Create", theme.TextSmall, semantic.Text.Muted).SingleLine().Shrink(0)
				}
				components.Label(c, "#"+row.name, theme.TextSmall, semantic.Text.Default).
					SingleLine().MinWidth(0)
			})
			if button.Clicked() {
				el.pickTag(i, services, c.Now())
				el.Focus()
				services.Invalidate()
			}
		}
	})
}
