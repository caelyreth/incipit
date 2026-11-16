package shell

import (
	"fmt"
	"strings"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// A block is a group as the field shows it: its head, then what it holds
// in full, row by row, as the sheet list showed it, one column wide and no
// taller than its half of the field holds, the rest counted. A block is as
// tall as what it holds, laid out once and moved as a whole
// (stage.Frame), its layer kept.

// shape is how a block stands: its rows, column by column, its size, and
// how many of its sheets it leaves out.
type shape struct {
	size stage.Size
	cols [][]row
	more int
}

// row is one row of a block: a group inside it, a sheet, a command, the
// area that makes a sheet there, or what it leaves out.
type row struct {
	group   *GroupItem
	entry   *entry
	command *Command
	create  bool
	h       float32
}

// blockMetrics are the measures blocks are laid out to, at the type's
// size, for a field w by h.
type blockMetrics struct {
	scale, k          float32
	col, pad, head    float32
	rowGap, tall      float32
	groupRow, control float32
	horizon           stage.Horizon
}

func (s *State) blockMetrics(w, h float32) blockMetrics {
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	pad, gap := 6*k, 14*k
	cols, col := stage.Columns(w-48*k, 260*k, 340*k, gap)
	band := 40*k + 2*gap
	return blockMetrics{
		scale: scale, k: k,
		col: col - 2*pad, pad: pad, head: 36 * k, rowGap: 2 * k,
		// A block is no taller than its half of the field holds.
		tall:     max(160*k, h/2-band/2-gap-36*k-3*pad),
		groupRow: 2*theme.Space06.DIPs(scale) + theme.TextSmall.LineHeight(scale),
		control:  theme.DefaultControls.Scaled(scale).Height,
		horizon:  stage.Horizon{Cols: cols, Col: col, Gap: gap, Band: band},
	}
}

// shapes are the blocks' shapes, worked out again only when what they
// hold or the measures change.
func (s *State) shapes(c *ui.Context, f found, m blockMetrics) []shape {
	key := fmt.Sprintf("%s|%d|%s|%v", f.query, f.rev, f.open, m)
	if s.field.shapesKey == key && len(s.field.shapes) == len(f.blocks) {
		return s.field.shapes
	}
	out := make([]shape, len(f.blocks))
	for i := range f.blocks {
		out[i] = s.shapeOf(c, &f.blocks[i], m)
	}
	s.field.shapes, s.field.shapesKey = out, key
	return out
}

func (s *State) shapeOf(c *ui.Context, b *block, m blockMetrics) shape {
	switch {
	case b.single:
		h := entryHeight(c, b.entries[0], m, 4)
		return shape{size: stage.Size{W: m.col + 2*m.pad, H: h + 2*m.pad}, cols: [][]row{{{entry: &b.entries[0], h: h}}}}
	case b.create:
		return shape{size: stage.Size{W: m.col + 2*m.pad, H: m.control + 2*m.pad}, cols: [][]row{{{create: true, h: m.control}}}}
	}
	rows := make([]row, 0, len(b.groups)+len(b.entries)+len(b.commands)+1)
	for i := range b.commands {
		rows = append(rows, row{command: &b.commands[i], h: m.groupRow})
	}
	for i := range b.groups {
		rows = append(rows, row{group: &b.groups[i], h: m.groupRow})
	}
	for i := range b.entries {
		rows = append(rows, row{entry: &b.entries[i], h: entryHeight(c, b.entries[i], m, 2)})
	}
	// Where sheets can be made, the area that makes one ends the block.
	if g := b.group; g != nil && !b.trashed && !g.Smart() {
		rows = append(rows, row{create: true, h: m.control})
	}
	// One column, as tall as its half holds; what is left out is counted,
	// and the block's head goes into it.
	var sh shape
	var col []row
	colH := float32(0)
	for i, r := range rows {
		if len(col) > 0 && colH+m.rowGap+r.h > m.tall {
			for _, rest := range rows[i:] {
				if !rest.create {
					sh.more++
				}
			}
			for len(col) > 0 && colH+m.rowGap+m.groupRow > m.tall {
				colH -= col[len(col)-1].h + m.rowGap
				if last := col[len(col)-1]; !last.create {
					sh.more++
				}
				col = col[:len(col)-1]
			}
			col = append(col, row{h: m.groupRow})
			colH += m.rowGap + m.groupRow
			break
		}
		if len(col) > 0 {
			colH += m.rowGap
		}
		col = append(col, r)
		colH += r.h
	}
	sh.size = stage.Size{W: m.col + 2*m.pad, H: m.head + 2*m.pad}
	if len(col) > 0 {
		sh.cols = [][]row{col}
		sh.size.H += colH + m.pad
	}
	return sh
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

// blockView draws the block b shaped sh as fr says, its canvas origin at
// at; live while the field still shows it, not on its way out.
func (s *State) blockView(c *ui.Context, semantic theme.Theme, fr stage.Frame, b block, sh shape, at stage.Point, live bool, m blockMetrics, animate bool) {
	scale := fr.Rect.W / max(fr.Size.W, 1)
	renaming := b.group != nil && s.field.renaming == b.group.ID && !b.create
	// Its frame stands as tall as the change has it, its contents laid out
	// at their own height within: a block growing or shrinking shows more
	// or less of itself, and is never stretched.
	frame := ui.Box(c.Key("block-"+fr.ID)).Absolute().Left(0).Top(0).Size(fr.Size.W, fr.Rect.H/max(scale, 0.01)).
		TransformOrigin(0, 0).Scale(scale, scale).Translate(at.X+fr.Rect.X, at.Y+fr.Rect.Y).
		Opacity(fr.Shown).Radius(12 * m.k).Clip().Role(ui.RoleGroup)
	if !b.create {
		frame.Background(semantic.Surface.Page).Border(1, semantic.Border.Variant)
	}
	frame.Children(func() { s.blockContent(c, semantic, fr, b, sh, renaming, live, m, animate) })
}

// blockContent is what a block holds, at its own size.
func (s *State) blockContent(c *ui.Context, semantic theme.Theme, fr stage.Frame, b block, sh shape, renaming, live bool, m blockMetrics, animate bool) {
	box := ui.Column(c.Key("content")).Size(fr.Size.W, fr.Size.H).Shrink(0).Padding(m.pad)
	if !renaming {
		box.Cache()
	}
	if !live {
		box.IgnoreInput(true)
	} else if g := b.group; g != nil && !b.trashed && !g.Smart() && !b.create {
		if dropped, ok := s.dropOn(c, semantic, box, takes{sheets: true, groups: !g.Inbox}, animate); ok {
			switch d := dropped.(type) {
			case sheetDrag:
				s.Library.MoveSheet(d.id, g.ID, Last)
			case groupDrag:
				if d.id != g.ID {
					s.Library.MoveGroup(d.id, g.ID, Last)
				}
			}
		}
	}
	box.Children(func() {
		if !b.single && !b.create {
			s.blockHead(c, semantic, b, fr, m, animate)
		}
		if len(sh.cols) == 0 {
			return
		}
		ui.Row(c).Key("cols").AlignItems(ui.Start).Children(func() {
			for i, col := range sh.cols {
				ui.Column(c).Key(fmt.Sprint(i)).Width(m.col).Gap(m.rowGap).Shrink(0).Children(func() {
					for _, r := range col {
						switch {
						case r.group != nil:
							s.groupRow(c, semantic, *r.group, fr, m, animate)
						case r.entry != nil:
							s.entryRow(c, semantic, *r.entry, b.trashed, r.h, b.single, m, animate)
						case r.command != nil:
							s.commandRow(c, semantic, *r.command, m, animate)
						case r.create:
							g := *b.group
							components.CreateArea{Key: "create", Label: "New sheet", Animate: animate, Action: func() {
								s.Library.NewSheet(g.ID, Last)
								s.ShowDesk()
							}}.View(c, semantic).Height(r.h)
						default:
							s.moreRow(c, semantic, b, sh.more, fr, m, animate)
						}
					}
				})
			}
		})
	})
}

func (b block) name() string {
	switch {
	case b.group != nil:
		return b.group.Name
	case len(b.commands) > 0:
		return b.commands[0].Group
	}
	return "Trash"
}

// blockHead is a block's head: its group's icon, name and count, pressed
// to go into it.
func (s *State) blockHead(c *ui.Context, semantic theme.Theme, b block, fr stage.Frame, m blockMetrics, animate bool) {
	ui.Row(c).Key("head").Height(m.head).WidthPercent(100).AlignItems(ui.Center).Gap(4 * m.k).Shrink(0).Children(func() {
		g := b.group
		icon, ink := components.IconTrash, semantic.Icon.Muted
		if len(b.commands) > 0 {
			icon = components.IconCommand
		}
		if g != nil {
			icon = groupIcon(*g)
			if tint := groupTint(semantic, *g); tint != nil {
				ink = *tint
			}
		}
		head := ui.ButtonBase(c.Key("name")).Grow(1).MinWidth(0).FillHeight().PaddingX(8 * m.k).Gap(8 * m.k).
			AlignItems(ui.Center).Radius(8 * m.k).FocusRing(false).KeepFocus().Label(b.name())
		hover := components.HoverAmount(head, head.Hovered(), animate)
		head.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
		if g != nil {
			head.ContextMenu(s.groupMenu(*g, b.trashed))
			if !b.trashed && !g.Inbox {
				head.Drag(groupDrag{id: g.ID})
			}
			if !b.trashed && !b.scoped && s.field.renaming != g.ID && head.Clicked() {
				s.into(*g, fr.Rect)
			}
		}
		head.Children(func() {
			ui.Icon(c, icon.SVG()).FontSize(15 * m.k).TextColor(ink).Shrink(0)
			if g != nil && s.field.renaming == g.ID {
				s.renameInput(c, semantic, *g)
				return
			}
			components.Label(c, b.name(), theme.TextDefault, semantic.Text.Heading).Grow(1).MinWidth(0).SingleLine()
			if g != nil {
				components.Label(c, thousands(g.Count), theme.TextXSmall, semantic.Text.Muted).
					SingleLine().Shrink(0).FontFeatures("tnum")
			}
		})
		if g != nil && b.trashed && s.armed(c, deleteKey("group:"+g.ID)) {
			s.confirmButton(c, semantic, deleteKey("group:"+g.ID), "", "Delete", animate, func() { s.Library.DeleteGroup(g.ID) })
		}
	})
}

// rowFill is a row's ground: the selection's where it is in front on the
// desk or the keys are on it, the picked fill where it is picked, a step
// of grey under the pointer; and pressed, it gives a little.
func (s *State) rowFill(c *ui.Context, semantic theme.Theme, el ui.Element, id string, front, animate bool) {
	hover := components.HoverAmount(el, el.Hovered(), animate)
	selection := components.NavigationSelection(c, semantic)
	fill := components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover)
	switch {
	case s.isPicked(id):
		fill = components.Blend(selection.Picked, selection.PickedHover, hover)
	case front || s.field.chosen == id:
		fill = selection.Fill
	}
	el.Background(fill)
	press := float32(0)
	if el.Pressed() {
		press = 1
	}
	if animate {
		press = components.Follow(el, "press", press, components.MotionQuick)
	}
	if press > 0.001 {
		el.Scale(1-0.012*press, 1-0.012*press)
	}
	if el.FocusVisible() {
		el.Border(1, semantic.Border.Focused)
	}
}

// groupRow is a group inside a block: pressed, the query goes into it.
func (s *State) groupRow(c *ui.Context, semantic theme.Theme, g GroupItem, fr stage.Frame, m blockMetrics, animate bool) {
	id := "group:" + g.ID
	btn := ui.ButtonBase(c.Key("row-" + id)).WidthPercent(100).Height(m.groupRow).PaddingX(rowPad(m.scale)).
		Gap(8 * m.k).AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(m.scale).Radius).
		FocusRing(false).KeepFocus().Label(g.Name)
	s.rowFill(c, semantic, btn, id, false, animate)
	btn.ContextMenu(s.groupMenu(g, false))
	if !g.Inbox {
		btn.Drag(groupDrag{id: g.ID})
	}
	if btn.Clicked() {
		s.into(g, fr.Rect)
	}
	ink := semantic.Icon.Muted
	if tint := groupTint(semantic, g); tint != nil {
		ink = *tint
	}
	btn.Children(func() {
		ui.Icon(c, groupIcon(g).SVG()).FontSize(14 * m.k).TextColor(ink).Shrink(0)
		components.Label(c, g.Name, theme.TextSmall, semantic.Text.Default).Grow(1).MinWidth(0).SingleLine()
		components.Label(c, thousands(g.Count), theme.TextXSmall, semantic.Text.Muted).SingleLine().Shrink(0).FontFeatures("tnum")
	})
}

// commandRow is one of the app's commands: pressed, it runs.
func (s *State) commandRow(c *ui.Context, semantic theme.Theme, cmd Command, m blockMetrics, animate bool) {
	id := "command:" + cmd.ID
	btn := ui.ButtonBase(c.Key("row-" + id)).WidthPercent(100).Height(m.groupRow).PaddingX(rowPad(m.scale)).
		Gap(8 * m.k).AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(m.scale).Radius).
		FocusRing(false).KeepFocus().Label(cmd.Label)
	s.rowFill(c, semantic, btn, id, false, animate)
	if btn.Clicked() {
		s.activate(id, false)
	}
	btn.Children(func() {
		components.Label(c, cmd.Label, theme.TextSmall, semantic.Text.Default).Grow(1).MinWidth(0).SingleLine()
		if len(cmd.Keys) > 0 {
			components.Keycaps(c, components.KeycapStyle(semantic, m.scale, true), cmd.Keys, cmd.Binding).Shrink(0)
		}
	})
}

// entryRow is a sheet in a block, in full, as the sheet list showed it:
// pressed it opens on the desk, with Option alongside the sheets open;
// Cmd picks it with others or lets it go, Shift picks the run to it, and
// a menu on a picked sheet is for all the picked.
func (s *State) entryRow(c *ui.Context, semantic theme.Theme, e entry, trashed bool, h float32, single bool, m blockMetrics, animate bool) {
	sh := e.sheet
	id := "sheet:" + sh.ID
	pad, gap := rowPad(m.scale), rowGap(m.scale)
	confirming := trashed && (s.armed(c, deleteKey(id)) || s.armed(c, deletePickedKey(id)))
	front := sh.ID == s.ActivePage
	btn := ui.ButtonBase(c.Key("row-" + id)).WidthPercent(100).Height(h).Shrink(0).Padding(pad).
		AlignItems(ui.Start).Radius(theme.DefaultControls.Scaled(m.scale).Radius).
		FocusRing(false).KeepFocus().Label(sheetTitle(sh)).Selected(front)
	s.rowFill(c, semantic, btn, id, front, animate)
	if s.isPicked(id) && len(s.field.picked) > 1 {
		btn.ContextMenu(s.pickedMenu(id, trashed))
	} else {
		btn.ContextMenu(s.sheetMenu(sh, trashed))
	}
	if !trashed {
		btn.Drag(sheetDrag{id: sh.ID})
	}
	if !confirming && btn.Clicked() {
		mods := btn.ClickModifiers()
		switch {
		case mods&ui.Cmd != 0:
			s.pickToggle(id)
		case mods&ui.Shift != 0:
			s.pickTo(id)
		default:
			s.field.picked, s.field.anchor = nil, id
			s.activate(id, mods&ui.Alt != 0)
		}
	} else if !confirming && btn.Shortcut(0, ui.KeyEnter) {
		s.activate(id, false)
	}
	lines := 2
	if single {
		lines = 4
	}
	title := semantic.Text.Default
	if front || s.isPicked(id) {
		title = semantic.Text.Heading
	}
	btn.Children(func() {
		ui.Column(c).Key("entry").Grow(1).MinWidth(0).Gap(gap).Children(func() {
			ui.Row(c).Key("title").WidthPercent(100).AlignItems(ui.Center).Gap(pad).Children(func() {
				components.Label(c, sheetTitle(sh), theme.TextSmall, title).Grow(1).MinWidth(0).SingleLine()
				switch {
				case trashed && s.armed(c, deletePickedKey(id)):
					label := fmt.Sprintf("Delete %d", len(s.field.picked))
					s.confirmButton(c, semantic, deletePickedKey(id), "", label, animate, s.deletePicked)
				case confirming:
					s.confirmButton(c, semantic, deleteKey(id), "", "Delete", animate, func() { s.Library.DeleteSheet(sh.ID) })
				case !sh.Updated.IsZero():
					components.Label(c, when(sh.Updated, c.Now()), theme.TextXSmall, semantic.Text.Muted).
						SingleLine().Shrink(0).FontFeatures("tnum")
				}
			})
			if body := entryBody(e); body != "" {
				components.Label(c, body, theme.TextXSmall, semantic.Text.Muted).WidthPercent(100).MaxLines(lines)
			}
			s.rowTags(c, semantic, sh.Tags, m)
			components.Label(c, words(sh.Words), theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum").Opacity(0.7)
		})
	})
}

// rowTags are a sheet's tags on one line, as many as fit, the rest
// counted.
func (s *State) rowTags(c *ui.Context, semantic theme.Theme, tags []string, m blockMetrics) {
	if len(tags) == 0 {
		return
	}
	pad, gap := theme.Space06.DIPs(m.scale), theme.Space06.DIPs(m.scale)
	size := theme.TextXSmall.Size(m.scale)
	room := m.col - 2*rowPad(m.scale)
	chip := func(text string) float32 {
		w, _ := c.MeasureText(0, ui.Span{Text: text, Font: theme.UIFontFamily, Size: size})
		return w + 2*pad
	}
	shown, used := 0, float32(0)
	for i, tag := range tags {
		w := chip("#" + tag)
		rest := float32(0)
		if i < len(tags)-1 {
			rest = gap + chip(fmt.Sprintf("+%d", len(tags)-i-1))
		}
		if used+w+rest > room && shown > 0 {
			break
		}
		used += w + gap
		shown++
	}
	ui.Row(c).Key("tags").WidthPercent(100).Height(tagsHeight(m.scale)).Gap(gap).AlignItems(ui.Center).
		PaddingY(theme.Space04.DIPs(m.scale)).Clip().Children(func() {
		for _, tag := range tags[:shown] {
			components.TagChip(c, "#"+tag, theme.TextXSmall, pad, room, semantic.Tag.Background, semantic.Tag.Foreground).Shrink(0)
		}
		if hidden := len(tags) - shown; hidden > 0 {
			components.TagChip(c, fmt.Sprintf("+%d", hidden), theme.TextXSmall, pad, room, semantic.Tag.Background, semantic.Text.Muted).
				Shrink(0).Tooltip(strings.Join(tags[shown:], ", "))
		}
	})
}

// moreRow says how many sheets a block leaves out; pressed, the query goes
// into its group, where it shows more.
func (s *State) moreRow(c *ui.Context, semantic theme.Theme, b block, more int, fr stage.Frame, m blockMetrics, animate bool) {
	label := fmt.Sprintf("%s more", thousands(more))
	btn := ui.ButtonBase(c.Key("more")).WidthPercent(100).Height(m.groupRow).PaddingX(rowPad(m.scale)).
		AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(m.scale).Radius).FocusRing(false).KeepFocus().Label(label)
	s.rowFill(c, semantic, btn, "", false, animate)
	if b.group != nil && !b.scoped && btn.Clicked() {
		s.into(*b.group, fr.Rect)
	}
	btn.Children(func() {
		components.Label(c, label, theme.TextXSmall, semantic.Text.Muted).SingleLine()
	})
}

func sheetTitle(sh SheetItem) string {
	if sh.Title == "" {
		return "Untitled"
	}
	return sh.Title
}

func deleteKey(id string) string { return "delete-" + id }

func (s *State) renameInput(c *ui.Context, semantic theme.Theme, g GroupItem) {
	input := ui.TextInputBase(c.Key("rename-"+g.ID), &s.field.renameName).
		Grow(1).MinWidth(0).Font(theme.UIFontFamily).
		FontSize(theme.TextDefault.Size(s.Settings.UIFontSize)).
		TextColor(semantic.Text.Heading).Label("Group name")
	if s.field.renameFocus {
		s.field.renameFocus = false
		input.Focus()
	}
	switch inlineName(input) {
	case nameCancel:
		s.field.renaming = ""
	case nameCommit:
		s.field.renaming = ""
		name := strings.TrimSpace(s.field.renameName)
		if name == "" || name == g.Name {
			return
		}
		// The query going on in the group follows its new name.
		for i, text := range s.field.conditions {
			if c, ok := condition(text); ok && c.Key.Name == "in" && strings.EqualFold(c.Value, g.Name) {
				c.Value = name
				s.field.conditions[i] = c.String()
			}
		}
		s.Library.RenameGroup(g.ID, name)
	}
}

func (s *State) groupMenu(g GroupItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if trashed {
			if m.Item("Put Back").Chosen() {
				s.Library.RestoreGroup(g.ID)
			}
			m.Separator()
			if m.Item(deleteGroupLabel(g)).Chosen() {
				s.arm(deleteKey("group:" + g.ID))
			}
			return
		}
		if !g.Smart() {
			if m.Item("New Sheet").Chosen() {
				s.Library.NewSheet(g.ID, Last)
				s.ShowDesk()
			}
		}
		if g.Inbox {
			return
		}
		if !g.Smart() && m.Item("New Group Inside").Chosen() {
			s.newGroup(g.ID)
		}
		m.Separator()
		if m.Item("Rename…").Chosen() {
			s.startRename(g.ID)
		}
		m.Submenu("Color", func(m *ui.Menu) {
			for _, opt := range []struct {
				label string
				color int
			}{
				{"Default", TintMoonveil}, {"Violet", TintHalo}, {"Blue", TintHalo + 1}, {"Teal", TintHalo + 2},
				{"Amber", TintHalo + 3}, {"Rose", TintHalo + 4}, {"Indigo", TintHalo + 5},
			} {
				if m.Item(opt.label).Checked(g.Color == opt.color).Chosen() {
					s.Library.StyleGroup(g.ID, g.Icon, opt.color)
				}
			}
		})
		s.moveMenu(m, func(target string) { s.Library.MoveGroup(g.ID, target, Last) }, g.ID)
		m.Separator()
		if m.Item(trashGroupLabel(g)).Chosen() {
			s.Library.TrashGroup(g.ID)
		}
	}
}

func (s *State) sheetMenu(sh SheetItem, trashed bool) func(*ui.Menu) {
	return func(m *ui.Menu) {
		if trashed {
			if m.Item("Put Back").Chosen() {
				s.Library.RestoreSheet(sh.ID)
			}
			m.Separator()
			if m.Item("Delete Immediately…").Chosen() {
				s.arm(deleteKey("sheet:" + sh.ID))
			}
			return
		}
		if m.Item("Open").Chosen() {
			s.Library.OpenSheet(sh.ID, false)
			s.ShowDesk()
		}
		if m.Item("Open Alongside").Chosen() {
			s.Library.OpenSheet(sh.ID, true)
			s.ShowDesk()
		}
		m.Separator()
		if m.Item("Duplicate").Chosen() {
			s.Library.DuplicateSheet(sh.ID)
		}
		s.moveMenu(m, func(group string) { s.Library.MoveSheet(sh.ID, group, Last) }, "")
		m.Separator()
		if m.Item("Move to Trash").Chosen() {
			s.Library.TrashSheet(sh.ID)
		}
	}
}
