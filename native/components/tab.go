package components

import (
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// Tab is one document tab with independent select and close actions. It is a
// rounded pill inset in the title bar: 30 high, 96..220 wide, with 10/3
// horizontal padding so the close button sits concentric with the pill.
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
	height := theme.DefaultLayout.Tab * scale / theme.DefaultUIFontSize
	metrics := theme.DefaultControls.Scaled(scale)
	rest := semantic.GhostElement.Background
	textColor := semantic.Text.Muted
	iconColor := semantic.Icon.Muted
	if t.Selected {
		rest = semantic.Surface.Page
		textColor = semantic.Text.Heading
		iconColor = semantic.Icon.Accent
	}

	tab := ui.ButtonBase(c.Key(t.Key)).Role(ui.RoleTab).Label(t.Title).
		Tooltip(t.Title).
		Selected(t.Selected).FocusRing(false).
		Height(height).MinWidth(96 * scale / theme.DefaultUIFontSize).
		MaxWidth(220 * scale / theme.DefaultUIFontSize).Shrink(0).
		PaddingStart(10 * scale / theme.DefaultUIFontSize).
		PaddingEnd(3 * scale / theme.DefaultUIFontSize).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).Radius(metrics.Radius)
	hover := HoverAmount(tab, !t.Selected && tab.Hovered(), t.Animate)
	tab.Background(blend(rest, semantic.GhostElement.Hover, hover))
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
