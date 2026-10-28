package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Tab is one document tab with independent select and close actions. Its
// source geometry is 40 high, 120..260 wide, and uses 12/8 horizontal padding.
type Tab struct {
	Key       string
	Title     string
	Selected  bool
	Animate   bool
	Icon      *ui.SVG
	CloseIcon *ui.SVG
	OnSelect  func()
	OnClose   func()
}

func (t Tab) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	height := theme.DefaultLayout.Titlebar * scale / theme.DefaultUIFontSize
	rest := semantic.GhostElement.Background
	textColor := semantic.Text.Muted
	iconColor := semantic.Icon.Muted
	if t.Selected {
		rest = semantic.Surface.Page
		textColor = semantic.Text.Heading
		iconColor = semantic.Icon.Accent
	}

	tab := ui.ButtonBase(c.Key(t.Key)).Role(ui.RoleTab).Label(t.Title).
		Selected(t.Selected).FocusRing(false).
		Height(height).MinWidth(120 * scale / theme.DefaultUIFontSize).
		MaxWidth(260 * scale / theme.DefaultUIFontSize).Shrink(0).
		PaddingStart(theme.Space12.DIPs(scale)).
		PaddingEnd(theme.Space08.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Radius(0)
	target := rest
	if !t.Selected && tab.Hovered() {
		target = semantic.GhostElement.Hover
	}
	tab.Background(target)
	tab.Children(func() {
		ui.Icon(c, t.Icon).FontSize(16 * scale / theme.DefaultUIFontSize).
			TextColor(iconColor)
		Label(c, t.Title, theme.TextSmall, textColor).Grow(1).MinWidth(0).SingleLine()
		IconButton{
			Key: "close", Label: "Close " + t.Title, Icon: t.CloseIcon,
			Style: SubtleButton, Size: CompactButton, Animate: t.Animate,
			IconColor: &semantic.Icon.Muted, Action: t.OnClose,
		}.View(c, semantic)
	})
	if tab.Clicked() && t.OnSelect != nil {
		t.OnSelect()
	}
	return tab
}
