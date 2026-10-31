package shell

import (
	"fmt"

	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// placeChip is one of the places the library keeps in its strip of chips.
type placeChip struct {
	place Place
	label string
	icon  components.IconName
	count int
	// menu is the place's menu, if it has one; inbox takes what is dropped
	// on it.
	menu  func(*ui.Menu)
	inbox bool
}

// placeChips is the library's head: the Inbox and the filters as one strip
// of chips. The chosen one widens to a filled capsule with its name, the
// others keep to their icons, so the places a writer goes to most stay one
// click apart without taking a row each.
func (s *State) placeChips(c *ui.Context, semantic theme.Theme, animate bool) {
	scale := s.Settings.UIFontSize
	var chips []placeChip
	for _, group := range s.Library.Groups("") {
		if group.Inbox {
			chips = append(chips, placeChip{
				place: Place{Kind: PlaceGroup, Group: group.ID}, label: group.Name,
				icon: components.IconInbox, count: group.Count, menu: s.groupMenu(group), inbox: true,
			})
		}
	}
	chips = append(chips,
		placeChip{place: Place{Kind: PlaceAll}, label: "All", icon: components.IconSheets,
			count: len(s.Library.Sheets(Place{Kind: PlaceAll}))},
		placeChip{place: Place{Kind: PlaceRecent}, label: "Recent", icon: components.IconClock,
			count: len(s.Library.Sheets(Place{Kind: PlaceRecent}))},
	)
	// The strip shares the library's width: each chip is its icon's width,
	// and the chosen one takes the rest. Each rides its own spring, so as
	// one capsule opens the one before closes, and the strip keeps its
	// length.
	height := theme.DefaultControls.Scaled(scale).Height
	gap := theme.Space06.DIPs(scale)
	// It keeps to the traffic lights' line on both sides.
	inset := s.lightLine()
	width := (theme.DefaultLayout.Sidebar-2*float32(theme.Space08))*scale/theme.DefaultUIFontSize - 2*inset
	small := height + theme.Space08.DIPs(scale)
	large := width - float32(len(chips)-1)*(small+gap)
	// One stop for the keyboard: the arrows move along the strip.
	ui.Row(c).Key("place-chips").Role(ui.RoleTabList).Label("Places").FocusGroup(ui.Horizontal).
		WidthPercent(100).Gap(gap).
		Padding(theme.Space02.DIPs(scale), inset, theme.Space04.DIPs(scale), inset).Children(func() {
		for _, chip := range chips {
			s.chip(c, semantic, chip, small, large, animate)
		}
	})
}

func (s *State) chip(c *ui.Context, semantic theme.Theme, chip placeChip, small, large float32, animate bool) {
	scale := s.Settings.UIFontSize
	height := theme.DefaultControls.Scaled(scale).Height
	chosen := s.chosen(chip.place)
	tip := chip.label
	if chip.count > 0 {
		tip = fmt.Sprintf("%s · %s", chip.label, sheetCount(chip.count))
	}
	button := ui.ButtonBase(c.Key(placeKey(chip.place))).Label(chip.label).Tooltip(tip).
		Role(ui.RoleTab).Selected(chosen).
		Height(height).Radius(height / 2).Shrink(0).Clip().
		PaddingStart((small - 16*scale/theme.DefaultUIFontSize) / 2).PaddingEnd(theme.Space12.DIPs(scale)).
		AlignItems(ui.Center).Gap(theme.Space06.DIPs(scale)).FocusRing(false)
	on := float32(0)
	if chosen {
		on = 1
	}
	hover := components.HoverAmount(button, !chosen && button.Hovered(), animate)
	if animate {
		on = components.Follow(button, "chosen", on, motionPlace)
	}
	on = max(0, min(on, 1))
	button.Width(small + (large-small)*on)
	// A press sinks the chip a little, so the click is felt before the
	// place changes.
	press := float32(0)
	if button.Pressed() {
		press = 1
	}
	if animate {
		press = components.Follow(button, "press", press, components.MotionImmediate)
		button.Scale(1-0.04*press, 1-0.04*press)
	}
	rest := components.Blend(semantic.Surface.Inset, semantic.GhostElement.Hover, hover)
	button.Background(components.Blend(rest, semantic.Accent.Foreground, on))
	if button.FocusVisible() {
		button.Border(1, semantic.Border.Focused)
	}
	ink := components.Blend(semantic.Icon.Muted, semantic.Accent.OnAccent, on)
	button.Children(func() {
		ui.Icon(c, chip.icon.SVG()).FontSize(16 * scale / theme.DefaultUIFontSize).TextColor(ink).Shrink(0)
		if on > 0.01 {
			ui.Row(c).Key("name").Shrink(0).Gap(theme.Space04.DIPs(scale)).AlignItems(ui.Center).
				Opacity(on * on).Children(func() {
				components.Label(c, chip.label, theme.TextSmall, ink).SingleLine().MinWidth(0)
				if chip.count > 0 {
					components.Label(c, fmt.Sprint(chip.count), theme.TextXSmall, ink).
						SingleLine().FontFeatures("tnum").Opacity(0.75)
				}
			})
		}
		// Unfiled sheets wait in the Inbox: a dot says so while it is not
		// the place shown.
		if chip.inbox && chip.count > 0 && !chosen {
			dot := 6 * scale / theme.DefaultUIFontSize
			ui.Box(c).Key("dot").Absolute().Top(theme.Space06.DIPs(scale)).Right(theme.Space08.DIPs(scale)).
				Size(dot, dot).Radius(dot / 2).Background(semantic.Accent.Foreground).
				Transition(popTransition(animate))
		}
	})
	if chip.menu != nil {
		button.ContextMenu(chip.menu)
	}
	if chip.inbox {
		s.dropHighlight(c, semantic, button, true, false, true, animate)
		if sheet, ok := ui.Drop[sheetDrag](button); ok {
			s.Library.MoveSheet(sheet.id, chip.place.Group, Last)
		}
		if tab, ok := ui.Drop[tabDrag](button); ok {
			s.Library.MoveSheet(tab.id, chip.place.Group, Last)
		}
	}
	if button.Clicked() {
		s.Go(chip.place)
	}
}
