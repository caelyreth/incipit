package shell

import (
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// The desk is the canvas's half that holds the open sheets, side by side
// in the order they were opened, each a page written in where it lies.
// The one in front has the accent's edge, and the writing actions act on
// it; clicking another's head, or writing in it, brings it in front.

// The desk's measures at the reference type size.
const (
	panelPad = 16
	panelGap = 24
)

// canvasDesk draws the open sheets, height tall, or a word on how to open
// one while none is.
func (s *State) canvasDesk(c *ui.Context, semantic theme.Theme, height float32, animate bool) {
	k := components.UIFontSize(c) / theme.DefaultUIFontSize
	desk := ui.Row(c).Key("canvas-desk").Shrink(0).AlignItems(ui.Start).Gap(panelGap * k)
	desk.Children(func() {
		if len(s.Open) == 0 {
			ui.Column(c).Key("desk-empty").Width(320*k).Height(height).Shrink(0).
				AlignItems(ui.Center).Justify(ui.Center).Gap(12*k).Radius(10*k).
				Border(1, semantic.Border.Variant).Children(func() {
				ui.Icon(c, components.IconDocument.SVG()).FontSize(40 * k).
					TextColor(semantic.Icon.Muted).Opacity(0.35)
				components.Label(c, "Click a sheet to open it here", theme.TextXSmall, semantic.Text.Muted).
					Opacity(0.8).TextAlign(ui.Center)
			})
			return
		}
		for _, page := range s.Open {
			s.canvasPanel(c, semantic, page, height, animate)
		}
	})
}

// canvasPanel is an open sheet on the desk: a head with its group's mark,
// its title, and what takes it back to the columns or closes it, over its
// page, written in where it lies.
func (s *State) canvasPanel(c *ui.Context, semantic theme.Theme, page Page, height float32, animate bool) {
	size := components.UIFontSize(c)
	k := size / theme.DefaultUIFontSize
	z := s.canvas.scale()
	prose := s.Settings.Prose().Zoomed(z)
	front := page.ID == s.ActivePage
	edge := semantic.Border.Variant
	if front {
		edge = semantic.Accent.Foreground
	}
	panel := ui.Column(c).Key("panel-"+page.ID).Role(ui.RoleGroup).Label(page.Title).
		Width(prose.Column()+2*panelPad*k).Height(height).Shrink(0).Clip().Radius(10*k).
		Background(semantic.Surface.Page).Border(1, edge).Transition(listTransition(animate, true))
	panel.Children(func() {
		head := ui.Row(c).Key("panel-head").WidthPercent(100).Shrink(0).AlignItems(ui.Center).
			Gap(6*k).PaddingStart(panelPad*k).PaddingEnd(6*k).PaddingY(6*k).
			Border(0, ui.Transparent).Role(ui.RoleButton).Label("Bring " + page.Title + " in front")
		head.Children(func() {
			ink := semantic.Icon.Muted
			if group, ok := s.groupOf(page.ID); ok {
				if tint := groupTint(semantic, group); tint != nil {
					ink = *tint
				}
				ui.Icon(c, groupIcon(group).SVG()).FontSize(14 * k).Shrink(0).TextColor(ink)
			}
			title := semantic.Text.Muted
			if front {
				title = semantic.Text.Heading
			}
			components.Label(c, page.Title, theme.TextSmall, title).SingleLine().MinWidth(0).Grow(1)
			s.panelButton(c, semantic, "panel-columns-"+page.ID, "Write "+page.Title+" in columns", "Write in columns", components.IconSidebar,
				animate, func() {
					s.closeCanvas()
					if s.SelectPage != nil {
						s.SelectPage(page.ID)
					}
					s.reveal(page.ID)
				})
			s.panelButton(c, semantic, "panel-close-"+page.ID, "Close "+page.Title, "Close sheet", components.IconClose,
				animate, func() {
					if s.ClosePageID != nil {
						s.ClosePageID(page.ID)
					}
				})
		})
		if head.Clicked() && !front && s.SelectPage != nil {
			s.SelectPage(page.ID)
		}
		ui.Box(c).WidthPercent(100).Height(1).Shrink(0).Background(semantic.Border.Variant)
		ui.Column(c).Key("panel-page").WidthPercent(100).Grow(1).MinHeight(0).PaddingX(panelPad * k).
			Children(func() {
				if s.Sheet != nil {
					s.Sheet(c, page.ID, SheetFrame{
						Prose: prose, UIFontSize: size, Top: 16 * k, Bottom: 48 * k, Focus: front,
					})
				}
			})
	})
	if s.canvas.reveal == page.ID {
		s.revealPanel(panel.LayoutBounds())
	}
}

// panelButton is one of a panel head's small actions: label names it with
// its sheet, tooltip as it shows.
func (s *State) panelButton(
	c *ui.Context,
	semantic theme.Theme,
	key, label, tooltip string,
	icon components.IconName,
	animate bool,
	action func(),
) {
	k := components.UIFontSize(c) / theme.DefaultUIFontSize
	components.IconButton{
		Key: key, Label: label, Tooltip: tooltip, Icon: icon.SVG(),
		Style: components.SubtleButton, Size: components.CompactButton,
		IconSize: 13 * k, IconColor: &semantic.Icon.Muted, Animate: animate, Action: action,
	}.View(c, semantic)
}

// revealPanel pans the surface so a panel just opened, laid out at bounds,
// stands in the middle of its window, once it has been laid out.
func (s *State) revealPanel(bounds ui.Rect) {
	v, f := s.canvas.viewport, s.canvas.field
	if bounds.W <= 0 || v.W <= 0 {
		return
	}
	// Both bounds move with the surface's scroll, so the panel's place in
	// the field is theirs apart.
	x := bounds.X - f.X
	y := bounds.Y - f.Y
	s.canvas.scroll.X = max(0, x-(v.W-bounds.W)/2)
	s.canvas.scroll.Y = max(0, y-(v.H-bounds.H)/2)
	s.canvas.reveal = ""
}

// openOnCanvas opens a sheet on the desk, beside the others open, in
// front, and brings it into view.
func (s *State) openOnCanvas(id string) {
	if s.isOpen(id) {
		if s.SelectPage != nil {
			s.SelectPage(id)
		}
	} else {
		s.Library.OpenSheet(id, true)
	}
	s.canvas.reveal = id
}

// groupOf is the group a sheet is filed in.
func (s *State) groupOf(sheet string) (GroupItem, bool) {
	id := s.Library.SheetGroup(sheet)
	if id == "" {
		return GroupItem{}, false
	}
	return s.Library.Group(id)
}
