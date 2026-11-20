package shell

import (
	"math"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
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
	run    func()
	// menu, in place of run, is what it offers to choose from, as Move To
	// does.
	menu func(*ui.Menu)
}

// actionsMenu adds acts to a menu: what asks again opens the actions,
// ready, and what offers a choice is a submenu.
func (s *State) actionsMenu(m *ui.Menu, acts []fieldAction) {
	for _, a := range acts {
		switch {
		case a.menu != nil:
			m.Submenu(a.label, a.menu)
		case a.danger:
			if m.Item(a.label + "…").Chosen() {
				s.field.acting = true
				s.closeKey()
				s.arm(a.id)
			}
		default:
			if m.Item(a.label).Chosen() {
				a.run()
			}
		}
	}
}

// actionsMost is how many actions stand at once.
const actionsMost = 5

// fieldActions are what the pill's end opens now: the selection's while
// something is selected; otherwise what answers the query as it stands,
// then the place's own.
func (s *State) fieldActions() []fieldAction {
	if s.Library == nil || s.mode() != modeLibrary {
		return nil
	}
	if acts := s.selectionActions(); len(acts) > 0 {
		return acts[:min(len(acts), actionsMost)]
	}
	var out []fieldAction
	if !s.inTrash() && len(s.field.found.cards) == 0 && s.FieldQuery() != "" {
		out = append(out, fieldAction{id: "clear-search", label: "Clear Search", icon: components.IconClose, run: s.goHome})
	}
	out = append(out, s.placeActions()...)
	return out[:min(len(out), actionsMost)]
}

// placeActions are what can be done to the place the query is in, by its
// conditions and words, whatever the pill is asking: the `>` commands
// list them too.
func (s *State) placeActions() []fieldAction {
	if s.Library == nil || s.InSettings() {
		return nil
	}
	if s.inTrash() {
		if !s.trashHolds() {
			return nil
		}
		return []fieldAction{{id: emptyTrashKey, label: "Empty Trash", icon: components.IconTrash, danger: true,
			run: s.Library.EmptyTrash}}
	}
	out := []fieldAction{{id: "new-sheet", label: "New Sheet", icon: components.IconNewSheet, run: func() {
		s.Library.NewSheet(s.CurrentGroup(), Last)
		s.ShowDesk()
	}}}
	asked := s.wholeQuery()
	if s.commanding() {
		asked = strings.Join(s.field.conditions, " ")
	}
	scope, _, browse := s.browsing(query.Parse(asked))
	if g, ok := s.Library.Group(scope); browse && (scope == "" || ok && !g.Smart()) {
		out = append(out, fieldAction{id: "new-group", label: "New Group", icon: components.IconNewGroup,
			run: func() { s.newGroup(scope) }})
	}
	return out
}

// trashHolds reports whether anything is in the trash: as the field shows
// it, or, while the pill asks the commands, as the library has it.
func (s *State) trashHolds() bool {
	if !s.commanding() {
		return len(s.field.found.cards) > 0
	}
	return len(s.Library.TrashedGroups())+len(s.Library.Sheets(Place{Kind: PlaceTrash})) > 0
}

// pressAction does a as pressing it does: what asks again turns first,
// and runs on a second press; then the actions close.
func (s *State) pressAction(a fieldAction) {
	if a.danger && s.armedKey != a.id {
		s.arm(a.id)
		return
	}
	s.disarm()
	s.field.acting = false
	a.run()
}

// actionKey moves the keys along the open actions, up and down, and
// Return presses the one they are on, reporting whether it took key.
func (s *State) actionKey(key ui.Key) bool {
	acts := s.fieldActions()
	if !s.field.acting || len(acts) == 0 {
		return false
	}
	switch key {
	case ui.KeyUp:
		s.field.actionAt = max(0, s.field.actionAt-1)
	case ui.KeyDown:
		s.field.actionAt = min(len(acts)-1, s.field.actionAt+1)
	case ui.KeyEnter:
		if s.field.actionAt < 0 || s.field.actionAt >= len(acts) {
			return false
		}
		// What offers a choice opens under the pointer alone.
		if acts[s.field.actionAt].menu == nil {
			s.pressAction(acts[s.field.actionAt])
		}
	default:
		return false
	}
	return true
}

// toggleActions opens the place's actions, closing any key, or closes
// them.
func (s *State) toggleActions() {
	s.disarm()
	s.field.acting = !s.field.acting
	// No action is under the keys until an arrow puts them on one.
	s.field.actionAt = -1
	if s.field.acting {
		s.closeKey()
	}
}

// actionsButton is the pill's end, pillH tall: pressed anywhere on it,
// it opens the place's actions, or closes them; its face is a circle
// that takes the chosen fill while they are open, standing in the pill's
// rounded end, inset all round alike, so the two curves are one.
func (s *State) actionsButton(c *ui.Context, semantic theme.Theme, acts []fieldAction, pillH, k float32, animate bool) {
	label := map[bool]string{true: "Hide Actions", false: "Actions"}[s.field.acting]
	inset := 4 * k
	end := ui.ButtonBase(c.Key("actions-button")).Size(pillH*1.25, pillH).Shrink(0).Padding(inset).
		AlignItems(ui.Center).Justify(ui.End).
		FocusRing(false).KeepFocus().Cursor(ui.CursorDefault).
		Label(label).Disabled(len(acts) == 0)
	// Its glyph says what it holds: a + where it makes things, as a new
	// sheet or group, and three dots where it holds what is done to the
	// selection or the trash. With nothing to hold it is not there.
	creates := len(s.field.picked) == 0 && !s.inTrash()
	components.Hint(c, semantic, end, map[bool]string{true: "New", false: "Actions"}[creates], "", components.HintAbove)
	if end.Clicked() {
		s.toggleActions()
	}
	if len(acts) == 0 {
		end.Opacity(0).IgnoreInput(true)
	}
	hover := components.HoverAmount(end, end.Hovered(), animate)
	open := map[bool]float32{true: 1}[s.field.acting]
	if animate {
		open = components.Follow(end, "open", open, components.MotionQuick)
	}
	face := pillH - 2*inset
	end.Children(func() {
		ui.Row(c.Key("face")).Size(face, face).Shrink(0).Radius(face / 2).AlignItems(ui.Center).Justify(ui.Center).IgnoreInput(true).
			Background(components.Blend(components.Blend(semantic.GhostElement.Background, semantic.Surface.Chosen, open),
				semantic.GhostElement.Hover, hover)).
			Children(func() {
				ink := components.Blend(semantic.Icon.Muted, semantic.Icon.Default, max(open, hover))
				if creates {
					ui.Icon(c, components.IconPlus.SVG()).FontSize(face * 0.5).TextColor(ink)
					return
				}
				dot := 3 * k
				ui.Row(c).Gap(dot).Children(func() {
					for range 3 {
						ui.Box(c).Size(dot, dot).Radius(dot / 2).Background(ink)
					}
				})
			})
	})
}

// actionsView stands the place's actions open beside the pill at r, w
// wide in the field: an arc about the pill's end, centred on its line,
// in order from the top, those further over or under it nearer the pill,
// as the end's own curve goes. Pressing anywhere else, the pill too,
// closes them.
func (s *State) actionsView(c *ui.Context, semantic theme.Theme, acts []fieldAction, r stageRect, w, gap, k float32, animate bool) {
	if len(acts) == 0 && s.field.acting {
		s.field.acting = false
	}
	// Over the field and the pill alike: any press but on an action,
	// the pill's end too, closes them.
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
			s.actionCapsule(c, semantic, a, at[i].X, at[i].Y+lift, k, at[i].Y < (r.H-h)/2, i == s.field.actionAt, animate)
		}
	})
}

// arc is where n capsules h tall stand about the end of a pill r, gap
// from it and space apart, as offsets from r's top: centred on its line,
// in order down, along a circle that leaves the middle gap past its end.
func arc(n int, r stageRect, h, gap, space float32) []stagePoint {
	step := h + space
	radius := 3 * step
	cx := r.X + r.W + gap - radius
	mid := (r.H - h) / 2
	at := make([]stagePoint, n)
	for i := range at {
		dy := (float32(i) - float32(n-1)/2) * step
		at[i] = stagePoint{X: cx + float32(math.Sqrt(float64(radius*radius-dy*dy))), Y: mid + dy}
	}
	return at
}

// actionCapsule is one action at x, y: pressed, it runs, the actions
// closing; one that cannot be undone turns in the danger color first and
// runs on a second press.
func (s *State) actionCapsule(c *ui.Context, semantic theme.Theme, a fieldAction, x, y, k float32, above, keyed, animate bool) {
	h := ribbonH * k
	armed := a.danger && s.armed(c, a.id)
	label := a.label
	if a.menu != nil {
		label += "…"
	}
	// Armed, it says on its face what a second press does: there is no
	// undoing it.
	if armed {
		label += " — Can’t Undo"
	}
	size := theme.TextSmall.Size(s.Settings.UIFontSize)
	textW, _ := c.MeasureText(0, ui.Span{Text: label, Font: theme.UIFontFamily, Size: size})
	w := 2*ribbonStart*k + 14*k + 6*k + textW + 1
	btn := ui.ButtonBase(c.Key("action-"+a.id)).Absolute().Left(x).Top(y).Size(w, h).
		PaddingX(ribbonStart * k).Gap(6 * k).AlignItems(ui.Center).Radius(h / 2).
		FocusRing(false).KeepFocus().Label(label).Transition(ribbonTransition(animate, above, k))
	amount, dangerInk := dangerTone(btn, semantic, armed, btn.Hovered() || keyed, animate,
		semantic.Surface.Page, semantic.Border.Variant)
	if a.menu != nil {
		btn.Menu(a.menu)
	} else if btn.Clicked() {
		s.pressAction(a)
	}
	btn.Children(func() {
		ui.Icon(c, a.icon.SVG()).FontSize(14 * k).TextColor(components.Blend(semantic.Icon.Muted, dangerInk, amount))
		components.CenteredLabel(c, label, theme.TextSmall, components.Blend(semantic.Text.Default, dangerInk, amount)).
			SingleLine().Shrink(0)
	})
}
