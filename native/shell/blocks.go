package shell

import (
	"fmt"
	"slices"
	"strings"

	"github.com/caelyreth/neverland/locales"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// view is what a block holds: a group's head and rows, taking sheets and
// groups dropped on it.
func (b block) view(s *State, c *ui.Context, semantic theme.Theme, box ui.Element, fr stageFrame, sh shape, current bool, m blockMetrics, animate bool) {
	if b.divider || b.section != "" {
		s.dividerView(c, semantic, b, box, fr, m, animate)
		return
	}
	// Its rows come and go as what it holds changes, sheets made, trashed
	// or brought back, the rest making room (listTransition): those pushed
	// past its room go into its count, and those it has room for again
	// come out of it.
	box.Gap(m.headGap).Children(func() {
		if !b.single && !b.create {
			s.blockHead(c, semantic, b, fr, m, animate)
		}
		cardRows(c, sh, m, func(r row) {
			switch {
			case r.group != nil:
				s.groupRow(c, semantic, *r.group, fr, m, animate)
			case r.entry != nil:
				s.entryRow(c, semantic, *r.entry, b.trashed, r.h, b.single, m, animate)
			case r.create:
				g := *b.group
				components.CreateArea{Key: "create", Label: locales.NewSheetArea(), Animate: animate, Action: func() {
					s.Library.NewSheet(g.ID, Last)
					s.ShowDesk()
				}}.View(c, semantic).Height(r.h).Transition(listTransition(animate, false))
			default:
				s.moreRow(c, semantic, b, sh.more, fr, m, animate)
			}
		})
	})
}

// dividerView is a list's divider: a group's head across the list, as the
// head of its card does, or a sort's step, named.
func (s *State) dividerView(c *ui.Context, semantic theme.Theme, b block, box ui.Element, fr stageFrame, m blockMetrics, animate bool) {
	// Its head alone, standing where a card's words do.
	over := m
	box.Padding(0).Children(func() {
		ui.Row(c).Key("over").WidthPercent(100).PaddingX(m.pad).Shrink(0).Children(func() {
			if b.section != "" {
				ui.Row(c).Key("step").Height(over.head).PaddingX(8 * m.k).AlignItems(ui.Center).Children(func() {
					components.CenteredLabel(c, b.section, theme.TextLarge, semantic.Text.Heading)
				})
			} else {
				s.blockHead(c, semantic, b, fr, over, animate)
			}
		})
	})
}

func (b block) name() string {
	if b.group != nil {
		return b.group.Name
	}
	return locales.Trash()
}

// blockHead is a block's head: its group's icon, the groups it is in from
// the one the query is in, its name and count, pressed to go into it.
func (s *State) blockHead(c *ui.Context, semantic theme.Theme, b block, fr stageFrame, m blockMetrics, animate bool) {
	ui.Row(c).Key("head").Height(m.head).WidthPercent(100).AlignItems(ui.Center).Gap(4 * m.k).Shrink(0).Children(func() {
		g := b.group
		icon, ink := components.IconTrash, semantic.Icon.Muted
		if g != nil {
			icon = groupIcon(*g)
			ink = groupInk(semantic, *g, ink)
		}
		head := ui.ButtonBase(c.Key("name")).Grow(1).MinWidth(0).FillHeight().PaddingX(8 * m.k).Gap(8 * m.k).
			AlignItems(ui.Center).Radius(8 * m.k).FocusRing(false).KeepFocus().Label(b.name())
		if g == nil {
			hover := components.HoverAmount(head, head.Hovered(), animate)
			head.Background(components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover))
		} else {
			// The head is the group's row: selected, chosen, pressed and
			// arriving as any row is.
			id := "group:" + g.ID
			s.rowFill(c, semantic, head, id, animate)
			head.ContextMenu(s.rowMenu(id, s.groupMenu(*g, b.trashed)))
			if !b.trashed && !g.Inbox {
				s.dragFrom(head, groupDrag{id: g.ID})
			}
			// What is dropped on its head is filed in the group, and held
			// there it goes into it.
			if !b.trashed && !g.Smart() {
				over, dropped, ok := s.dropOn(c, semantic, head, filing(*g), animate)
				if ok {
					s.fileInto(*g, dropped)
				}
				s.springLoad(c, id, *g, over, fr.Rect)
			}
			if s.field.renaming != g.ID && head.Clicked() && !s.pickRow(id, head.ClickModifiers()) && !b.trashed {
				s.into(*g, fr.Rect)
			}
		}
		head.Children(func() {
			ui.Icon(c, icon.SVG()).FontSize(15 * m.k).TextColor(ink).Shrink(0)
			if g != nil && s.field.renaming == g.ID {
				s.renameInput(c, semantic, *g)
				return
			}
			components.BaselineRow(c).Key("words").Grow(1).MinWidth(0).Gap(8 * m.k).Children(func() {
				ui.Row(c).Key("title").Grow(1).MinWidth(0).Gap(4 * m.k).AlignItems(ui.Baseline).Children(func() {
					if len(b.path) > 0 {
						components.Label(c, strings.Join(b.path, " › ")+" ›", theme.TextDefault, semantic.Text.Muted).MinWidth(0).SingleLine()
					}
					components.Label(c, b.name(), theme.TextDefault, semantic.Text.Heading).Shrink(0).MaxWidthPercent(100).SingleLine()
				})
				if g != nil {
					components.Label(c, thousands(g.Count), theme.TextXSmall, semantic.Text.Muted).
						SingleLine().Shrink(0).FontFeatures("tnum")
				}
			})
			if g != nil {
				taskRing(c, semantic, g.Tasks, m)
			}
			if g != nil && g.Listed {
				listBadge(c, semantic, m)
			}
			if g != nil && g.Pinned {
				pinBadge(c, semantic, m)
			}
		})
		if g != nil && b.trashed && s.armed(c, deleteKey("group:"+g.ID)) {
			s.confirmButton(c, semantic, deleteKey("group:"+g.ID), "", locales.Delete(), animate, func() { s.Library.DeleteGroup(g.ID) })
		}
	})
}

// badge is a small mark on a row, a line of the smallest text tall, in a
// family's color with the dark ink: an icon, and what it counts if it
// does. Badges say what a row is beside what it shows: pinned, open on
// the desk.
func badge(c *ui.Context, semantic theme.Theme, key, label string, family theme.Accent, icon components.IconName, text string, m blockMetrics) {
	line := theme.TextXSmall.LineHeight(m.scale)
	ink := semantic.Accent.OnAccent
	row := ui.Row(c.Key(key)).Height(line).MinWidth(line).Shrink(0).Gap(3 * m.k).Radius(line / 2).
		AlignItems(ui.Center).Justify(ui.Center).Background(semantic.Families[family].Foreground).
		IgnoreInput(true).Label(label)
	if text != "" {
		row.PaddingX(5 * m.k)
	}
	row.Children(func() {
		ui.Icon(c, icon.SVG()).FontSize(line * 0.7).TextColor(ink)
		if text != "" {
			components.CenteredLabel(c, text, theme.TextXSmall, ink).SingleLine().FontFeatures("tnum")
		}
	})
}

// pinBadge says a group or sheet is pinned: a thumbtack on Remnant.
func pinBadge(c *ui.Context, semantic theme.Theme, m blockMetrics) {
	badge(c, semantic, "pin-badge", locales.Pinned(), theme.Remnant, components.IconPin, "", m)
}

// taskRing is how far a sheet's or a group's tasks are done, as a ring
// that fills as they are, a line of the smallest text across, checked once
// all are; nothing where there are none. Its count is a hint, the pointer
// resting on it, so the card stays as quiet as the ring.
func taskRing(c *ui.Context, semantic theme.Theme, t Tasks, m blockMetrics) {
	if t.All() == 0 {
		return
	}
	line := theme.TextXSmall.LineHeight(m.scale)
	label := locales.TasksDone(fmt.Sprint(t.Done), fmt.Sprint(t.All()))
	ring := components.ProgressRing(c, "task-ring", line*0.8, 2*m.k, float32(t.Done)/float32(t.All()),
		semantic.Border.Default, semantic.Accent.Foreground, label)
	components.Hint(c, semantic, ring, locales.TasksDoneHint(fmt.Sprint(t.Done), fmt.Sprint(t.All())), "", components.HintAbove)
}

// listBadge says a group is kept shown as a list: what is kept is said
// where it is kept, so turning the view in it is never a guess at what
// lasts.
func listBadge(c *ui.Context, semantic theme.Theme, m blockMetrics) {
	badge(c, semantic, "list-badge", locales.AlwaysShownAsList(), theme.Unripe, components.IconList, "", m)
}

// rowFill is a row's ground: the selection's where the keys are on it,
// the picked fill where it is selected, a step of the ink under the
// pointer, the accent's tint a moment as it arrives; and pressed, it
// gives a little. A sheet open on the desk is never a fill, which would
// read as a hover left behind, but its strip (openMark). It keeps where
// the row stands, for the arrows (keys.go).
func (s *State) rowFill(c *ui.Context, semantic theme.Theme, el ui.Element, id string, animate bool) {
	hover := components.HoverAmount(el, el.Hovered(), animate)
	selection := components.NavigationSelection(c, semantic)
	fill := components.Blend(semantic.GhostElement.Background, semantic.GhostElement.Hover, hover)
	switch {
	case s.isPicked(id):
		fill = components.Blend(selection.Picked, selection.PickedHover, hover)
	case s.field.chosen == id:
		fill = selection.Fill
	}
	el.Background(components.Blend(fill, semantic.Accent.Tint, s.arrivalAt(id, c.Now())))
	press := float32(0)
	if el.Pressed() {
		press = 1
	}
	if animate {
		press = components.Follow(el, "press", press, components.MotionQuick)
	}
	if press > 0.001 {
		// Pressed, a row sinks as far whatever its width: a ratio would
		// sink a list's divider, as wide as the list, past where the press
		// is taken for a drag, and its click would be lost.
		w := max(el.LayoutBounds().W, 1)
		sink := 1 - min(0.012, rowSink/w)*press
		el.Scale(sink, sink)
	}
	if el.FocusVisible() {
		el.Border(1, semantic.Border.Focused)
	}
	s.rowPlace(el, id)
}

// groupRow is a group inside a block: pressed, the query goes into it.
func (s *State) groupRow(c *ui.Context, semantic theme.Theme, g GroupItem, fr stageFrame, m blockMetrics, animate bool) {
	id := "group:" + g.ID
	btn := ui.ButtonBase(c.Key("row-" + id)).WidthPercent(100).Height(m.groupRow).PaddingX(rowPad(m.scale)).
		Gap(8 * m.k).AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(m.scale).Radius).
		FocusRing(false).KeepFocus().Label(g.Name).Transition(listTransition(animate, false))
	s.rowFill(c, semantic, btn, id, animate)
	// What is dropped on the row is filed in its group, not in the block's,
	// and held there it goes into it.
	over, dropped, ok := s.dropOn(c, semantic, btn, filing(g), animate)
	if ok {
		s.fileInto(g, dropped)
	}
	s.springLoad(c, id, g, over, fr.Rect)
	btn.ContextMenu(s.rowMenu(id, s.groupMenu(g, false)))
	if !g.Inbox {
		s.dragFrom(btn, groupDrag{id: g.ID})
	}
	if btn.Clicked() && !s.pickRow(id, btn.ClickModifiers()) {
		s.into(g, fr.Rect)
	}
	ink := semantic.Icon.Muted
	ink = groupInk(semantic, g, ink)
	btn.Children(func() {
		ui.Icon(c, groupIcon(g).SVG()).FontSize(14 * m.k).TextColor(ink).Shrink(0)
		components.BaselineRow(c).Key("words").Grow(1).MinWidth(0).Gap(8 * m.k).Children(func() {
			components.Label(c, g.Name, theme.TextSmall, semantic.Text.Default).Grow(1).MinWidth(0).SingleLine()
			components.Label(c, thousands(g.Count), theme.TextXSmall, semantic.Text.Muted).SingleLine().Shrink(0).FontFeatures("tnum")
		})
		taskRing(c, semantic, g.Tasks, m)
		if g.Listed {
			listBadge(c, semantic, m)
		}
		if g.Pinned {
			pinBadge(c, semantic, m)
		}
	})
}

// filing is what g takes to be filed in it: sheets, the tray, and groups
// but in the Inbox.
func (s *State) entryRow(c *ui.Context, semantic theme.Theme, e entry, trashed bool, h float32, single bool, m blockMetrics, animate bool) {
	sh := e.sheet
	id := "sheet:" + sh.ID
	pad, gap := rowPad(m.scale), rowGap(m.scale)
	confirming := trashed && s.armed(c, deleteKey(id))
	front := sh.ID == s.ActivePage
	btn := ui.ButtonBase(c.Key("row-" + id)).WidthPercent(100).Height(h).Shrink(0).Padding(pad).
		AlignItems(ui.Start).Radius(theme.DefaultControls.Scaled(m.scale).Radius).
		FocusRing(false).KeepFocus().Label(sheetTitle(sh)).Selected(front).
		Transition(listTransition(animate, false))
	s.rowFill(c, semantic, btn, id, animate)
	btn.ContextMenu(s.rowMenu(id, s.sheetMenu(sh, trashed)))
	// Dragged, a sheet that is a card of its own goes as the card, on the
	// card's ground, what it leaves standing dimmed in its place; a row of
	// a group's card is a chip of its title alone.
	dragged := !trashed && s.dragFrom(btn, sheetDrag{id: sh.ID})
	// Selected with the one dragged, it goes too, and dims as it does.
	if d, ok := s.field.drag.(sheetDrag); ok && !dragged && d.id != sh.ID && slices.Contains(s.carried(d), sh.ID) {
		btn.Opacity(0.4)
	}
	switch {
	case dragged && single:
		btn.Background(semantic.Surface.Page).Border(1, semantic.Border.Variant).Radius(12 * m.k)
	case dragged:
		// Its title line tall, the rest of its place kept below it, so
		// nothing around it moves.
		chip := 2*pad + theme.TextSmall.LineHeight(m.scale)
		btn.Height(chip).Margin(0, 0, h-chip, 0).
			Background(semantic.Surface.Elevated).Border(1, semantic.Border.Default)
	}
	if !confirming && btn.Clicked() {
		if mods := btn.ClickModifiers(); !s.pickRow(id, mods) {
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
		if dragged {
			dragCount(c, semantic, len(s.carried(sheetDrag{id: sh.ID})), m)
		}
		at := s.deskPlace(sh.ID)
		if at > 0 {
			openMark(c, semantic, h, m)
		}
		if at > 0 || sh.Pinned || sh.Tasks.All() > 0 {
			ui.Row(c.Key("badges")).Absolute().Right(pad).Bottom(pad).Gap(4 * m.k).AlignItems(ui.Center).Children(func() {
				taskRing(c, semantic, sh.Tasks, m)
				if sh.Pinned {
					pinBadge(c, semantic, m)
				}
				if at > 0 {
					badge(c, semantic, "open-badge", locales.OpenOnDesk(fmt.Sprint(at)), theme.Halo,
						components.IconDesk, fmt.Sprint(at), m)
				}
			})
		}
		ui.Column(c).Key("entry").Grow(1).MinWidth(0).Gap(gap).Children(func() {
			if confirming {
				ui.Row(c).Key("title").WidthPercent(100).AlignItems(ui.Center).Gap(pad).Children(func() {
					components.CenteredLabel(c, sheetTitle(sh), theme.TextSmall, title).Grow(1).MinWidth(0)
					s.confirmButton(c, semantic, deleteKey(id), "", locales.Delete(), animate, func() { s.Library.DeleteSheet(sh.ID) })
				})
			} else {
				components.BaselineRow(c).Key("title").WidthPercent(100).Gap(pad).Children(func() {
					components.Label(c, sheetTitle(sh), theme.TextSmall, title).Grow(1).MinWidth(0).SingleLine()
					if !sh.Updated.IsZero() {
						components.Label(c, when(sh.Updated, c.Now()), theme.TextXSmall, semantic.Text.Muted).
							SingleLine().Shrink(0).FontFeatures("tnum")
					}
				})
			}
			// A chip is its title alone; a card goes whole.
			if dragged && !single {
				return
			}
			if body := entryBody(e); body != "" || sh.Picture != "" {
				ui.Row(c).Key("body").WidthPercent(100).Gap(pad).AlignItems(ui.Start).Children(func() {
					if body != "" {
						components.Label(c, body, theme.TextXSmall, semantic.Text.Muted).Grow(1).MinWidth(0).MaxLines(lines)
					} else {
						ui.Box(c).Grow(1)
					}
					if sh.Picture != "" {
						s.thumbnail(c, semantic, sh.Picture, thumbSide(m, lines), m)
					}
				})
			}
			s.rowTags(c, semantic, sh.Tags, m)
			foot := words(sh.Words)
			if e.in != "" {
				foot = e.in + " · " + foot
			}
			components.Label(c, foot, theme.TextXSmall, semantic.Text.Muted).SingleLine().FontFeatures("tnum").Opacity(0.7)
		})
	})
}

// dragCount marks what is dragged with how many sheets go with it, where
// more than one does: a round count in the accent on its corner, as a
// Mac's files dragged together are counted.
func dragCount(c *ui.Context, semantic theme.Theme, n int, m blockMetrics) {
	if n < 2 {
		return
	}
	line := theme.TextXSmall.LineHeight(m.scale)
	ui.Row(c.Key("drag-count")).Absolute().Top(-line / 3).Right(-line / 3).Height(line).MinWidth(line).
		PaddingX(4 * m.k).Radius(line / 2).AlignItems(ui.Center).Justify(ui.Center).
		Background(semantic.Accent.Foreground).IgnoreInput(true).Label(locales.SheetsCount(n)).Children(func() {
		components.CenteredLabel(c, fmt.Sprint(n), theme.TextXSmall, semantic.Accent.OnAccent).SingleLine().FontFeatures("tnum")
	})
}

// thumbnail is a sheet's first picture, side across, cropped to fill its
// rounded square: its place, inset, while it is read, and a muted mark
// where it cannot show.
func (s *State) thumbnail(c *ui.Context, semantic theme.Theme, media string, side float32, m blockMetrics) {
	var bitmap *ui.Bitmap
	shows := true
	if s.Thumbnail != nil {
		bitmap, shows = s.Thumbnail(media, c.Now())
	}
	frame := ui.Box(c.Key("thumbnail")).Size(side, side).Shrink(0).Radius(6*m.k).Clip().
		Background(semantic.Surface.Inset).Border(1, semantic.Border.Variant).
		AlignItems(ui.Center).Justify(ui.Center).IgnoreInput(true)
	frame.Children(func() {
		switch {
		case bitmap != nil:
			ui.Image(c, bitmap).Size(side, side).Fit(ui.Cover)
		case !shows:
			ui.Icon(c, components.IconImageOff.SVG()).FontSize(side * 0.4).TextColor(semantic.Text.Muted)
		}
	})
}

// deskPlace is where the sheet id stands on the desk, counted from one as
// the desk's columns go, 0 where it is not open.
func (s *State) deskPlace(id string) int {
	for i, page := range s.Open {
		if page.ID == id {
			return i + 1
		}
	}
	return 0
}

// openMark marks a sheet's row h tall as open on the desk
// (field-and-desk.md §4.7.4): a strip in the accent along its leading
// edge, in the card's margin, as the sheet list's was; its badge, of the
// desk and the number for its column in Halo, stands at the foot's end.
func openMark(c *ui.Context, semantic theme.Theme, h float32, m blockMetrics) {
	thin, inset := 3*m.k, 6*m.k
	ui.Box(c.Key("open-strip")).Absolute().Left(-m.pad/2-thin/2).Top(inset).Size(thin, h-2*inset).Radius(thin / 2).
		Background(semantic.Accent.Foreground).IgnoreInput(true)
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
			more := components.TagChip(c, fmt.Sprintf("+%d", hidden), theme.TextXSmall, pad, room, semantic.Tag.Background, semantic.Text.Muted).
				Shrink(0).Label(strings.Join(tags[shown:], ", "))
			components.Hint(c, semantic, more, strings.Join(tags[shown:], ", "), "", components.HintAbove)
		}
	})
}

// moreRow says how many sheets a block leaves out; pressed, the query goes
// into its group, where it shows more.
func (s *State) moreRow(c *ui.Context, semantic theme.Theme, b block, more int, fr stageFrame, m blockMetrics, animate bool) {
	label := locales.More(thousands(more))
	btn := ui.ButtonBase(c.Key("more")).WidthPercent(100).Height(m.groupRow).PaddingX(rowPad(m.scale)).
		AlignItems(ui.Center).Radius(theme.DefaultControls.Scaled(m.scale).Radius).FocusRing(false).KeepFocus().Label(label).
		Transition(listTransition(animate, false))
	s.rowFill(c, semantic, btn, "", animate)
	if b.group != nil && btn.Clicked() {
		s.into(*b.group, fr.Rect)
	}
	btn.Children(func() {
		components.CenteredLabel(c, label, theme.TextXSmall, semantic.Text.Muted).FontFeatures("tnum")
	})
}

func sheetTitle(sh SheetItem) string {
	if sh.Title == "" {
		return locales.Untitled()
	}
	return sh.Title
}

func (s *State) renameInput(c *ui.Context, semantic theme.Theme, g GroupItem) {
	input := ui.TextInputBase(c.Key("rename-"+g.ID), &s.field.renameName).
		Grow(1).MinWidth(0).Font(theme.UIFontFamily).
		FontSize(theme.TextDefault.Size(s.Settings.UIFontSize)).
		TextColor(semantic.Text.Heading).Label(locales.GroupName())
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
		s.renameGroup(g.ID, name)
	}
}

// rowSink is the most a row pressed narrows by, in DIPs, half of it each
// side: well within the press a drag starts past.
const rowSink = 3
