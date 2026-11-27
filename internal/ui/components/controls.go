package components

import (
	"github.com/caelyreth/incipit/internal/ui/theme"
	"github.com/caelyreth/incipit/locales"
	"github.com/egoist/mygo/ui"
)

// TagChip is a tag as the app shows one beside the page, in the sheet
// list and the page's tag picker: its text at size on fill, rounded as the
// page rounds a tag's background, cut short where width cannot hold it.
func TagChip(c *ui.Context, text string, size theme.TextSize, pad, width float32, fill, ink ui.Color) ui.Element {
	scale := UIFontSize(c)
	chip := ui.Row(c).Height(size.LineHeight(scale) + theme.Space04.DIPs(scale)).
		Radius(3 * scale / theme.DefaultUIFontSize).PaddingX(pad).
		MaxWidth(width).MinWidth(0).AlignItems(ui.Center).Background(fill)
	chip.Children(func() {
		CenteredLabel(c, text, size, ink).MinWidth(0)
	})
	return chip
}

// BadgeFrame gives el a pill badge's geometry at the UI size scale: its
// height, its words inset, rounded to its middle. Its fill and words are
// its own.
func BadgeFrame(el ui.Element, scale float32) ui.Element {
	height := theme.DefaultControls.Scaled(scale).Badge
	return el.Shrink(0).Height(height).PaddingX(theme.Space06.DIPs(scale)).
		Radius(height / 2).AlignItems(ui.Center)
}

type Swatch struct {
	Key      string
	Label    string
	Color    ui.Color
	Selected bool
	Action   func()
}

func (s Swatch) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	size := 22 * scale / theme.DefaultUIFontSize
	ring := ui.Transparent
	if s.Selected {
		ring = semantic.Text.Heading
	}
	button := ui.ButtonBase(c.Key(s.Key)).Role(ui.RoleRadio).
		Label(s.Label).Selected(s.Selected).KeepFocus().
		Size(size, size).Padding(0).Radius(size/2).
		Background(s.Color).Border(2*scale/theme.DefaultUIFontSize, ring)
	Hint(c, semantic, button, s.Label, "", HintAbove)
	if button.Clicked() && s.Action != nil {
		s.Action()
	}
	return button
}

type MenuButton struct {
	Key     string
	Label   string
	Icon    *ui.SVG
	Animate bool
	Build   func(*ui.Menu)
}

func (b MenuButton) View(c *ui.Context, semantic theme.Theme) ui.Element {
	button := IconButton{
		Key: b.Key, Label: b.Label, Icon: b.Icon,
		Style: SubtleButton, Size: CompactButton,
		IconColor: semantic.Icon.Muted, Animate: b.Animate,
	}.View(c, semantic)
	return button.Menu(b.Build)
}

// ChoiceButton names a choice on its face, a chevron after it: a subtle,
// compact button like the icon buttons beside it, which opens what it
// chooses from (Action), lit while that is open (Selected), its hint where
// Tooltip says.
type ChoiceButton struct {
	Key      string
	Label    string
	Tooltip  string
	HintSide HintSide
	Disabled bool
	Selected bool
	Animate  bool
	// Size is its height's, CompactButton unless set.
	Size   ButtonSize
	Action func()
}

func (b ChoiceButton) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	k := scale / theme.DefaultUIFontSize
	if b.Size == 0 {
		b.Size = CompactButton
	}
	return buttonElement(c, semantic, buttonOptions{
		key: b.Key, label: b.Label, tooltip: b.Tooltip, side: b.HintSide,
		style: SubtleButton, size: b.Size, disabled: b.Disabled, selected: b.Selected,
		animate: b.Animate, action: b.Action,
		children: func() {
			// Its name is whole or clipped, never cut short with an
			// ellipsis: squeezed on the way, as a bar grows or folds about
			// it, it would flicker between the two.
			ink, chevron := semantic.Text.Default, semantic.Icon.Muted
			if b.Selected {
				ink, chevron = semantic.Text.OnInverse, semantic.Text.OnInverse
			}
			CenteredLabel(c, b.Label, theme.TextSmall, ink).Shrink(0)
			ui.Icon(c, IconChevronDown.SVG()).FontSize(12 * k).TextColor(chevron)
		},
	}).Shrink(0).PaddingStart(theme.Space08.DIPs(scale)).PaddingEnd(theme.Space06.DIPs(scale)).
		Gap(theme.Space04.DIPs(scale)).AlignItems(ui.Center)
}

type NumberField struct {
	Key         string
	Label       string
	Value       string
	CanDecrease bool
	CanIncrease bool
	Animate     bool
	Decrease    func()
	Increase    func()
}

func (n NumberField) View(c *ui.Context, semantic theme.Theme) ui.Element {
	scale := UIFontSize(c)
	metrics := theme.DefaultControls.Scaled(scale)
	return ui.Row(c).Height(metrics.Height).Shrink(0).AlignItems(ui.Center).
		Gap(theme.Space04.DIPs(scale)).PaddingX(theme.Space04.DIPs(scale)).
		Radius(metrics.Radius).Border(1, semantic.Border.Variant).
		Background(semantic.Surface.Inset).Children(func() {
		IconButton{
			Key: "decrease", Label: locales.Decrease(n.Label), Icon: IconMinus.SVG(),
			Style: SubtleButton, Size: CompactButton, Disabled: !n.CanDecrease,
			Animate: n.Animate, IconColor: semantic.Icon.Muted, Action: n.Decrease,
		}.View(c, semantic)
		ui.Box(c).MinWidth(58 * scale / theme.DefaultUIFontSize).
			AlignItems(ui.Center).Justify(ui.Center).Children(func() {
			CenteredLabel(c, n.Value, theme.TextSmall, semantic.Text.Default).
				FontFeatures("tnum")
		})
		IconButton{
			Key: "increase", Label: locales.Increase(n.Label), Icon: IconPlus.SVG(),
			Style: SubtleButton, Size: CompactButton, Disabled: !n.CanIncrease,
			Animate: n.Animate, IconColor: semantic.Icon.Muted, Action: n.Increase,
		}.View(c, semantic)
	})
}
