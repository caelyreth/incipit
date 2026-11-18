package shell

import (
	"math"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

// What is done to the field's place as a whole stands in one place that
// never moves (field-and-desk.md §4.7.1): a button at the pill's end,
// which opens the place's actions as capsules on the pill's own line at
// its right, ribbon-like but neutral, while the ribbons stand in the
// bands over and under it. One thing opens around the pill at a time:
// the actions put the offered ribbons away, and a key opened closes them.
// Each action is declared once, here, for the capsules and the field's
// menu alike.

// fieldAction is one thing that can be done to the field's place.
type fieldAction struct {
	id, label string
	icon      components.IconName
	// danger asks again before it runs: what cannot be undone.
	danger bool
	// notable marks the pill's button: the place has it to do, as a trash
	// with something in it.
	notable bool
	run     func()
}

// actionsMost is how many actions stand at once.
const actionsMost = 5

// fieldActions are what can be done to the field's place now, the most
// used first, nearest the pill.
func (s *State) fieldActions() []fieldAction {
	if s.Library == nil || s.InSettings() || strings.HasPrefix(s.field.words, ">") {
		return nil
	}
	if s.inTrash() {
		if len(s.field.found.cards) == 0 {
			return nil
		}
		return []fieldAction{{id: emptyTrashKey, label: "Empty Trash", icon: components.IconTrash, danger: true, notable: true,
			run: s.Library.EmptyTrash}}
	}
	var out []fieldAction
	if len(s.field.found.cards) == 0 && s.FieldQuery() != "" {
		out = append(out, fieldAction{id: "clear-search", label: "Clear Search", icon: components.IconClose, run: s.goHome})
	}
	out = append(out, fieldAction{id: "new-sheet", label: "New Sheet", icon: components.IconNewSheet, run: func() {
		s.Library.NewSheet(s.CurrentGroup(), Last)
		s.ShowDesk()
	}})
	scope, _, browse := s.browsing(query.Parse(s.wholeQuery()))
	if g, ok := s.Library.Group(scope); browse && (scope == "" || ok && !g.Smart()) {
		out = append(out, fieldAction{id: "new-group", label: "New Group", icon: components.IconNewGroup,
			run: func() { s.newGroup(scope) }})
	}
	return out[:min(len(out), actionsMost)]
}

// toggleActions opens the place's actions, closing any key, or closes
// them.
func (s *State) toggleActions() {
	s.disarm()
	s.field.acting = !s.field.acting
	if s.field.acting {
		s.closeKey()
	}
}

// actionsButton is the button at the pill's end that opens its actions,
// marked when one of them is notable: a round face the pill's height
// over 0.6, pressed anywhere in the whole height of the pill's end.
func (s *State) actionsButton(c *ui.Context, semantic theme.Theme, acts []fieldAction, pillH, k float32, animate bool) {
	label := map[bool]string{true: "Hide Actions", false: "Actions"}[s.field.acting]
	h := pillH * 0.6
	hit := ui.ButtonBase(c.Key("actions-button")).Size(pillH, pillH).Shrink(0).
		AlignItems(ui.Center).Justify(ui.Center).FocusRing(false).KeepFocus().Cursor(ui.CursorDefault).
		Label(label).Tooltip("Actions").Disabled(len(acts) == 0)
	if hit.Clicked() {
		s.toggleActions()
	}
	hover := components.HoverAmount(hit, hit.Hovered(), animate)
	var btn ui.Element
	hit.Children(func() { btn = ui.Box(c.Key("face")).Size(h, h).Radius(h / 2).IgnoreInput(true) })
	btn.AlignItems(ui.Center).Justify(ui.Center)
	if len(acts) == 0 {
		hit.Opacity(components.DisabledOpacity)
	}
	open := map[bool]float32{true: 1}[s.field.acting]
	if animate {
		open = components.Follow(hit, "open", open, components.MotionQuick)
	}
	btn.Background(components.Blend(components.Blend(semantic.GhostElement.Background, semantic.Surface.Chosen, open),
		semantic.GhostElement.Hover, hover))
	notable := false
	for _, a := range acts {
		notable = notable || a.notable
	}
	btn.Children(func() {
		ui.Icon(c, components.IconMore.SVG()).FontSize(h * 0.55).
			TextColor(components.Blend(semantic.Icon.Muted, semantic.Icon.Default, max(open, hover)))
		if notable {
			dot := 6 * k
			ui.Box(c.Key("notable")).Absolute().Top(h*0.12).Right(h*0.12).Size(dot, dot).Radius(dot / 2).
				Background(semantic.Accent.Foreground).Opacity(1 - open).IgnoreInput(true)
		}
	})
}

// actionsView stands the place's actions open beside the pill at r, w
// wide in the field: an arc about the pill's end, centred on its line,
// in order from the top, those further over or under it nearer the pill,
// as the end's own curve goes. Pressing anywhere else closes them.
func (s *State) actionsView(c *ui.Context, semantic theme.Theme, acts []fieldAction, r stage.Rect, w, gap, k float32, animate bool) {
	if len(acts) == 0 && s.field.acting {
		s.field.acting = false
	}
	if s.field.acting {
		away := ui.ButtonBase(c.Key("actions-away")).Absolute().Left(0).Top(0).Size(w, s.field.viewH).
			FocusRing(false).KeepFocus().Cursor(ui.CursorDefault).Label("Close Actions")
		if away.Clicked() {
			s.toggleActions()
		}
	}
	h := ribbonH * k
	ui.Box(c).Key("actions").Absolute().Left(0).Top(0).Translate(0, r.Y).Children(func() {
		if !s.field.acting {
			return
		}
		at := arc(len(acts), r, h, gap, ribbonGap*k)
		// Held in the window where the pill stands near its edge.
		lift := max(0, 8*k-(r.Y+at[0].Y)) - max(0, r.Y+at[len(at)-1].Y+h+8*k-s.field.viewH)
		for i, a := range acts {
			s.actionCapsule(c, semantic, a, at[i].X, at[i].Y+lift, k, at[i].Y < (r.H-h)/2, animate)
		}
	})
}

// arc is where n capsules h tall stand about the end of a pill r, gap
// from it and space apart, as offsets from r's top: centred on its line,
// in order down, along a circle that leaves the middle gap past its end.
func arc(n int, r stage.Rect, h, gap, space float32) []stage.Point {
	step := h + space
	radius := 3 * step
	cx := r.X + r.W + gap - radius
	mid := (r.H - h) / 2
	at := make([]stage.Point, n)
	for i := range at {
		dy := (float32(i) - float32(n-1)/2) * step
		at[i] = stage.Point{X: cx + float32(math.Sqrt(float64(radius*radius-dy*dy))), Y: mid + dy}
	}
	return at
}

// actionCapsule is one action at x, y: pressed, it runs, the actions
// closing; one that cannot be undone turns in the danger color first and
// runs on a second press.
func (s *State) actionCapsule(c *ui.Context, semantic theme.Theme, a fieldAction, x, y, k float32, above, animate bool) {
	h := ribbonH * k
	armed := a.danger && s.armed(c, a.id)
	size := theme.TextSmall.Size(s.Settings.UIFontSize)
	textW, _ := c.MeasureText(0, ui.Span{Text: a.label, Font: theme.UIFontFamily, Size: size})
	w := 2*ribbonStart*k + 14*k + 6*k + textW + 1
	btn := ui.ButtonBase(c.Key("action-"+a.id)).Absolute().Left(x).Top(y).Size(w, h).
		PaddingX(ribbonStart * k).Gap(6 * k).AlignItems(ui.Center).Radius(h / 2).
		FocusRing(false).KeepFocus().Label(a.label).Transition(ribbonTransition(animate, above, k))
	if armed {
		btn.Tooltip("Click again to " + strings.ToLower(a.label) + ". You can’t undo this.")
	}
	amount := map[bool]float32{true: 1}[armed]
	if animate {
		amount = max(0, min(1, components.Follow(btn, "armed", amount, motionArm)))
	}
	hover := components.HoverAmount(btn, btn.Hovered(), animate)
	danger := semantic.Status.Danger
	rest := components.Blend(semantic.Surface.Page, semantic.GhostElement.Hover, hover)
	btn.Background(components.Blend(rest, danger.Background, amount)).
		Border(1, components.Blend(semantic.Border.Variant, danger.Border, amount))
	if btn.FocusVisible() {
		btn.Border(1, semantic.Border.Focused)
	}
	if btn.Clicked() {
		if a.danger && !armed {
			s.arm(a.id)
		} else {
			s.disarm()
			s.field.acting = false
			a.run()
		}
	}
	btn.Children(func() {
		ui.Icon(c, a.icon.SVG()).FontSize(14 * k).TextColor(components.Blend(semantic.Icon.Muted, danger.Foreground, amount))
		components.CenteredLabel(c, a.label, theme.TextSmall, components.Blend(semantic.Text.Default, danger.Foreground, amount)).
			SingleLine().Shrink(0)
	})
}
