package shell

import (
	"math"
	"strconv"
	"strings"

	"github.com/caelyreth/neverland/internal/query"
	"github.com/caelyreth/neverland/native/components"
	"github.com/caelyreth/neverland/native/stage"
	"github.com/caelyreth/neverland/native/theme"
	"github.com/egoist/mygo/ui"
)

type pillState struct {
	query        string
	input        queryInput
	focus        bool
	typing       bool
	scopeGroup   string
	fidelityStep int
	pan          stage.Point
}

// PillQuery returns the text currently written in the field pill.
func (s *State) PillQuery() string {
	if s == nil {
		return ""
	}
	return s.pill.query
}

// SetPillQuery sets the pill's query text.
func (s *State) SetPillQuery(q string) {
	if s != nil {
		s.pill.query = q
	}
}

// pillView draws the viewport-anchored Neverland pill inside the field room:
// the three-strip mark and wordmark, quiet posture chips, and the live query
// field.
func (s *State) pillView(c *ui.Context, semantic theme.Theme, scene stage.Scene, animate bool) {
	if scene.Pill.Opacity <= 0.001 {
		return
	}
	scale := s.Settings.UIFontSize
	k := scale / theme.DefaultUIFontSize
	roomW := scene.FieldRoom.W
	if roomW <= 0 {
		roomW = scene.Field.Rect.W
	}
	width := min(392*k, max(240*k, roomW-48*k))

	pill := ui.Column(c).Key("pill").Absolute().
		Left(0).Top(0).
		TranslatePercent(-50, -50).
		Translate(scene.Pill.Rect.X+s.field.damped.X, scene.Pill.Rect.Y+s.field.damped.Y).
		Scale(scene.Pill.Scale, scene.Pill.Scale).
		Opacity(scene.Pill.Opacity).
		Width(width).
		Padding(10*k, 12*k, 10*k, 12*k).Gap(theme.Space08.DIPs(scale)).
		Role(ui.RoleGroup).Label("Neverland pill")
	components.ElevatedSurface(pill, semantic, scale, components.PopoverElevation)
	pill.Radius(16 * k)
	if !scene.Pill.Interactive {
		pill.IgnoreInput(true)
	}

	pill.Children(func() {
		ui.Row(c).Key("pill-head").WidthPercent(100).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).Children(func() {
			home := ui.ButtonBase(c.Key("pill-home")).AlignItems(ui.Center).Gap(theme.Space08.DIPs(scale)).
				FocusRing(false).KeepFocus().Label("Center field").Tooltip("Return to field center")
			if home.Clicked() {
				s.pill.pan = stage.Point{}
				s.field.selectedID = ""
			}
			home.Children(func() {
				s.threeStripMark(c, semantic, k)
				ui.Row(c).Key("pill-wordmark").AlignItems(ui.End).Children(func() {
					size := 13 * k
					ui.Text(c, "Neverland").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
						LetterSpacing(-0.02 * size).TextColor(semantic.Text.Heading).SingleLine()
					ui.Text(c, ".").Font(theme.UIFontFamily).FontSize(size).FontWeight(700).
						TextColor(semantic.Accent.Foreground).SingleLine()
				})
			})
			if s.pill.scopeGroup != "" && s.Library != nil {
				if group, ok := s.Library.Group(s.pill.scopeGroup); ok {
					s.pillScopeChip(c, semantic, group, k, animate)
				}
			}
			ui.Box(c).Grow(1)
			s.pillPostureChips(c, semantic, k, animate)
		})

		_, input := queryField{
			Key:         "pill-query",
			Label:       "Search field",
			Placeholder: "Search sheets, #group, $ for keys",
			Value:       &s.pill.query,
			In:          &s.pill.input,
			Scope:       query.InNotebook,
			Focus:       s.pill.focus && scene.Pill.Interactive,
			Animate:     animate,
			Keys: func(key ui.TextKey) bool {
				return s.pillKey(key)
			},
		}.View(s, c, semantic)
		s.pill.typing = input.Focused()
		if scene.Pill.Interactive {
			s.pill.focus = false
		}
		s.syncPillScopeToken()
		s.pillRibbon(c, semantic, k, animate)
	})
}

func (s *State) pillScopeChip(c *ui.Context, semantic theme.Theme, group GroupItem, k float32, animate bool) {
	ink := semantic.Text.Default
	if tint := groupTint(semantic, group); tint != nil {
		ink = *tint
	}
	chip := ui.ButtonBase(c.Key("pill-scope")).Height(22 * k).PaddingX(7 * k).Gap(4 * k).
		AlignItems(ui.Center).Radius(7 * k).Background(semantic.Surface.Chosen).
		Transition(popTransition(animate)).
		FocusRing(false).KeepFocus().Label("Clear scope " + group.Name).Tooltip("Show whole library")
	if !group.Smart() && s.Library != nil {
		if dropped, ok := s.dropOn(c, semantic, chip, takes{sheets: true, groups: !group.Inbox}, animate); ok {
			switch d := dropped.(type) {
			case sheetDrag:
				s.Library.MoveSheet(d.id, group.ID, Last)
			case groupDrag:
				if d.id != group.ID {
					s.Library.MoveGroup(d.id, group.ID, Last)
				}
			}
		}
	}
	if chip.Clicked() {
		s.SetPillScope("")
	}
	chip.Children(func() {
		ui.Icon(c, groupIcon(group).SVG()).FontSize(12 * k).TextColor(ink).Shrink(0)
		components.Label(c, group.Name, theme.TextXSmall, semantic.Text.Default).SingleLine().MaxWidth(96 * k)
		ui.Icon(c, components.IconClose.SVG()).FontSize(11 * k).TextColor(semantic.Icon.Muted).Shrink(0)
	})
}

func (s *State) pillKey(key ui.TextKey) bool {
	switch key.Key {
	case ui.KeyEscape:
		if s.pill.query != "" {
			s.pill.query = ""
			s.pill.input.edit = &queryEdit{from: 0, to: math.MaxInt, text: ""}
			return true
		}
		if s.pill.scopeGroup != "" {
			s.SetPillScope("")
			return true
		}
		s.ReturnToDesk()
		return true
	case ui.KeyBackspace:
		if s.pill.query == "" && s.pill.scopeGroup != "" {
			s.SetPillScope("")
			return true
		}
		return false
	case ui.KeyEnter:
		alongside := key.Mods&(ui.Cmd|ui.Alt) != 0
		s.activateSelectedOrNearestCard(alongside)
		return true
	case ui.KeyUp:
		if key.Mods == 0 {
			s.moveFieldSelection(0, -1)
			return true
		}
		if key.Mods&ui.Alt != 0 {
			s.StepFidelity(-1)
			return true
		}
	case ui.KeyDown:
		if key.Mods == 0 {
			s.moveFieldSelection(0, 1)
			return true
		}
		if key.Mods&ui.Alt != 0 {
			s.StepFidelity(1)
			return true
		}
	case ui.KeyLeft:
		if key.Mods == 0 && s.pill.query == "" {
			s.moveFieldSelection(-1, 0)
			return true
		}
	case ui.KeyRight:
		if key.Mods == 0 && s.pill.query == "" {
			s.moveFieldSelection(1, 0)
			return true
		}
	case ui.KeyEqual:
		if key.Mods&ui.Cmd != 0 {
			s.StepFidelity(1)
			return true
		}
	case ui.KeyMinus:
		if key.Mods&ui.Cmd != 0 {
			s.StepFidelity(-1)
			return true
		}
	}
	return false
}

func (s *State) setPillQueryEdit(next string) {
	s.pill.query = next
	s.pill.input.edit = &queryEdit{from: 0, to: math.MaxInt, text: next}
}

func (s *State) syncPillScopeToken() {
	if s.Library == nil || !strings.Contains(s.pill.query, "#") {
		return
	}
	// Auto-apply #group when followed by whitespace so the user sees the group
	// scope chip appear smoothly while keeping the rest of the query.
	fields := strings.Fields(s.pill.query)
	trailingSpace := len(s.pill.query) > 0 && s.pill.query[len(s.pill.query)-1] == ' '
	var kept []string
	changed := false
	groups := s.allGroupsFlat()
	for i, f := range fields {
		if strings.HasPrefix(f, "#") && len(f) > 1 && (i < len(fields)-1 || trailingSpace) {
			name := strings.TrimPrefix(f, "#")
			if g, ok := findGroupByName(groups, name); ok {
				s.SetPillScope(g.ID)
				changed = true
				continue
			}
		}
		kept = append(kept, f)
	}
	if changed {
		next := strings.Join(kept, " ")
		if trailingSpace && next != "" {
			next += " "
		}
		s.setPillQueryEdit(next)
	}
}

func (s *State) pillRibbon(c *ui.Context, semantic theme.Theme, k float32, animate bool) {
	parsed := query.Parse(s.pill.query)
	conditions := query.Conditions(parsed)
	fidelity := s.PillFidelity()
	hasOpen := queryHasFilter(parsed, "open")
	hasRecent := queryHasFilterValue(parsed, "updated", "7d")
	hasTrash := queryHasFilter(parsed, "trash")

	token := func(key, text, a11yLabel string, active bool, action func()) ui.Element {
		btn := ui.ButtonBase(c.Key(key)).Height(20 * k).PaddingX(7 * k).
			AlignItems(ui.Center).Justify(ui.Center).
			Radius(6 * k).Label(a11yLabel).FocusRing(false).KeepFocus()
		hover := components.HoverAmount(btn, btn.Hovered(), animate)
		activeTarget := float32(0)
		if active {
			activeTarget = 1
		}
		activeAmount := activeTarget
		if animate {
			activeAmount = max(0, min(1, components.Follow(btn, "pill-token-active", activeTarget, components.MotionQuick)))
		}
		rest := components.Blend(semantic.Surface.Inset, semantic.Element.Hover, hover)
		bg := components.Blend(rest, semantic.Surface.Chosen, activeAmount)
		fg := components.Blend(semantic.Text.Muted, semantic.Text.Default, max(activeAmount, hover*0.6))
		btn.Background(bg)
		if btn.Clicked() {
			action()
		}
		btn.Children(func() {
			components.Label(c, text, theme.TextXSmall, fg).SingleLine()
		})
		return btn
	}

	ui.Row(c).Key("pill-ribbon").WidthPercent(100).Wrap().AlignItems(ui.Center).Gap(4 * k).Children(func() {
		sheetsActive := fidelity == stage.FidelitySheets
		viewText := "Boards"
		viewLabel := "Show all sheets"
		if sheetsActive {
			viewText = "Sheets"
			viewLabel = "Show overview boards"
		}
		token("pill-fidelity", viewText, viewLabel, sheetsActive, func() {
			if s.pill.fidelityStep == 0 {
				s.SetFidelityStep(1)
			} else {
				s.SetFidelityStep(0)
			}
		})

		token("pill-filter-open", "Open", "Filter open sheets", hasOpen, func() {
			s.togglePillFilter("open", "", "$open")
		})
		token("pill-filter-recent", "Recent", "Filter recent sheets", hasRecent, func() {
			s.togglePillFilter("updated", "7d", "$updated:7d")
		})
		trashBtn := token("pill-filter-trash", "Trash", "Show trash", hasTrash, func() {
			s.togglePillFilter("trash", "", "$trash")
		})
		if s.Library != nil {
			if dropped, ok := s.dropOn(c, semantic, trashBtn, takes{sheets: true, groups: true}, animate); ok {
				switch d := dropped.(type) {
				case sheetDrag:
					s.Library.TrashSheet(d.id)
				case groupDrag:
					s.Library.TrashGroup(d.id)
					s.ReconcileLibrary()
				}
			}
		}
		if !hasTrash && s.Library != nil {
			token("pill-new-group", "+ Group", "New group", false, func() {
				s.newGroupOnField(s.pill.scopeGroup)
			})
		} else if hasTrash && s.Library != nil {
			trashCount := s.Library.SheetCount(Place{Kind: PlaceTrash}) + len(s.Library.TrashedGroups())
			if trashCount > 0 {
				s.confirmButton(c, semantic, emptyTrashKey, "Empty", "Delete All", animate, s.Library.EmptyTrash)
			}
		}

		for i, cond := range conditions {
			if cond.Key == nil {
				continue
			}
			// Quick toggles already cover $open, $trash, and $updated:7d.
			if !cond.Not && (cond.Key.Name == "open" || cond.Key.Name == "trash" ||
				(cond.Key.Name == "updated" && strings.EqualFold(cond.Value, "7d"))) {
				continue
			}
			label := cond.Key.Label + " " + query.Describe(cond.Key, cond.Value)
			if cond.Not {
				label = "Not " + label
			}
			span := cond.Span
			chip := ui.ButtonBase(c.Key("pill-cond-" + strconv.Itoa(i))).Height(20 * k).PaddingX(6 * k).Gap(4 * k).
				AlignItems(ui.Center).Radius(6 * k).Background(semantic.Surface.Chosen).
				Transition(popTransition(animate)).FocusRing(false).KeepFocus().
				Label("Remove filter " + label)
			if chip.Clicked() {
				s.setPillQueryEdit(query.Cut(s.pill.query, span))
			}
			chip.Children(func() {
				components.Label(c, label, theme.TextXSmall, semantic.Text.Default).SingleLine().MaxWidth(116 * k)
				ui.Icon(c, components.IconClose.SVG()).FontSize(10 * k).TextColor(semantic.Icon.Muted).Shrink(0)
			})
		}
	})
}

func (s *State) togglePillFilter(keyName, wantVal, part string) {
	parsed := query.Parse(s.pill.query)
	for _, cond := range query.Conditions(parsed) {
		if cond.Key != nil && cond.Key.Name == keyName && !cond.Not &&
			(wantVal == "" || strings.EqualFold(cond.Value, wantVal)) {
			s.setPillQueryEdit(query.Cut(s.pill.query, cond.Span))
			return
		}
	}
	s.setPillQueryEdit(query.And(s.pill.query, part))
}

func queryHasFilterValue(q query.Query, keyName, wantVal string) bool {
	for _, cond := range query.Conditions(q) {
		if cond.Key != nil && cond.Key.Name == keyName && !cond.Not && strings.EqualFold(cond.Value, wantVal) {
			return true
		}
	}
	return false
}

func (s *State) allGroupsFlat() []GroupItem {
	if s.Library == nil {
		return nil
	}
	var out []GroupItem
	seen := map[string]bool{}
	var walk func(parent string)
	walk = func(parent string) {
		for _, g := range s.Library.Groups(parent) {
			if seen[g.ID] {
				continue
			}
			seen[g.ID] = true
			out = append(out, g)
			if g.HasGroups {
				walk(g.ID)
			}
		}
	}
	walk("")
	return out
}

func findGroupByName(groups []GroupItem, name string) (GroupItem, bool) {
	for _, g := range groups {
		if strings.EqualFold(g.Name, name) {
			return g, true
		}
	}
	return GroupItem{}, false
}

// threeStripMark draws the rounded three-strip Neverland emblem.
func (s *State) threeStripMark(c *ui.Context, semantic theme.Theme, k float32) {
	ui.Column(c).Key("pill-mark").Gap(2 * k).AlignItems(ui.Start).Children(func() {
		widths := [3]float32{14 * k, 10 * k, 6 * k}
		for i, w := range widths {
			color := semantic.Text.Muted
			if i == 2 {
				color = semantic.Accent.Foreground
			}
			ui.Box(c).Width(w).Height(2.5 * k).Radius(2 * k).Background(color)
		}
	})
}

func (s *State) pillPostureChips(c *ui.Context, semantic theme.Theme, k float32, animate bool) {
	chip := func(key, label string, active bool, action func()) {
		btn := ui.ButtonBase(c.Key(key)).Height(22 * k).PaddingX(8 * k).
			AlignItems(ui.Center).Justify(ui.Center).
			Radius(7 * k).Label(label).
			FocusRing(false).KeepFocus()
		hover := components.HoverAmount(btn, btn.Hovered(), animate)
		activeTarget := float32(0)
		if active {
			activeTarget = 1
		}
		activeAmount := activeTarget
		if animate {
			activeAmount = max(0, min(1, components.Follow(btn, "pill-chip-active", activeTarget, components.MotionQuick)))
		}
		rest := components.Blend(semantic.GhostElement.Background, semantic.Element.Hover, hover)
		bg := components.Blend(rest, semantic.Surface.Chosen, activeAmount)
		fg := components.Blend(semantic.Text.Muted, semantic.Text.Default, max(activeAmount, hover*0.6))
		btn.Background(bg)
		if btn.Clicked() {
			action()
		}
		btn.Children(func() {
			components.Label(c, label, theme.TextXSmall, fg).SingleLine()
		})
	}
	ui.Row(c).Key("pill-postures").AlignItems(ui.Center).Gap(2 * k).Children(func() {
		if len(s.Open) > 0 {
			chip("pill-glance", "Glance", s.posture == stage.PostureGlance, func() {
				s.SetPosture(stage.PostureGlance)
			})
		}
		chip("pill-arrive", "Explore", s.posture == stage.PostureArrive, func() {
			s.SetPosture(stage.PostureArrive)
		})
		if len(s.Open) > 0 {
			chip("pill-desk", "Desk", false, s.ReturnToDesk)
		}
	})
}
